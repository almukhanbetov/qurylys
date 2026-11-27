package companies

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/database"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// InvolvedSQL is true when user $1 is or was a member of company c. Leaving the
// company does not lift a conflict of interest.
const InvolvedSQL = `(
	EXISTS (SELECT 1 FROM company_members m WHERE m.company_id = c.id AND m.user_id = $1)
	OR EXISTS (SELECT 1 FROM audit_events a WHERE a.company_id = c.id AND (
		(a.actor_user_id = $1 AND a.action IN ('company.created', 'company.member_joined'))
		OR (a.target_user_id = $1 AND a.action LIKE 'company.member%'))))`

// IsNotFound reports that the caller is not a member (or the company does not exist).
func IsNotFound(err error) bool { return errors.Is(err, errNotFound) }

// access returns the user's role and the company status. Non-members get
// "not found", so ids of foreign companies reveal nothing. With lock the company
// row is locked, which serialises membership changes.
func access(ctx context.Context, q database.Querier, userID, companyID int64, lock bool) (role, status string, err error) {
	query := `
		SELECT cm.role, c.status FROM companies c
		JOIN company_members cm ON cm.company_id = c.id AND cm.user_id = $2
		WHERE c.id = $1`
	if lock {
		// Lock first, read the role afterwards: a concurrent demotion must be visible.
		if _, err := q.ExecContext(ctx, `SELECT 1 FROM companies WHERE id = $1 FOR UPDATE`, companyID); err != nil {
			return "", "", err
		}
	}
	err = q.QueryRowContext(ctx, query, companyID, userID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", errNotFound
	}
	return role, status, err
}

// Authorize is the single permission check for company-scoped requests:
// membership, then the selected company, then role and status for changes.
func Authorize(ctx context.Context, q database.Querier, p auth.Principal, companyID int64, ownerOnly, change bool) (string, error) {
	role, status, err := access(ctx, q, p.UserID, companyID, change)
	switch {
	case err != nil:
		return "", err
	case p.ActiveCompanyID != companyID:
		return "", errContext
	case ownerOnly && role != RoleOwner:
		return "", errForbidden
	case change && status != statusActive:
		return "", errSuspended
	}
	return role, nil
}

func (r *Repository) create(ctx context.Context, p auth.Principal, slug string, d Details) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			INSERT INTO companies (slug, name, bin, city, legal_address, actual_address, work_phone, representative_name, is_buyer, is_supplier)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
			slug, d.Name, d.BIN, d.City, d.LegalAddress, d.ActualAddress, d.WorkPhone, d.Representative, d.IsBuyer, d.IsSupplier).Scan(&id)
		if database.UniqueViolation(err) == "companies_bin_key" {
			return errBINTaken
		}
		if err != nil {
			return fmt.Errorf("create company: %w", err)
		}
		if err := addMember(ctx, tx, id, p.UserID, RoleOwner); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET active_company_id = $2 WHERE id = $1`, p.SessionID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: id, Action: "company.created"})
	})
	return id, err
}

func addMember(ctx context.Context, tx *sql.Tx, companyID, userID int64, role string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO company_members (company_id, user_id, role) VALUES ($1, $2, $3)`, companyID, userID, role)
	if database.UniqueViolation(err) != "" {
		return errAlreadyMember
	}
	return err
}

func (r *Repository) get(ctx context.Context, p auth.Principal, id int64) (Company, error) {
	c := Company{ID: id}
	role, err := Authorize(ctx, r.db, p, id, false, false)
	if err != nil {
		return c, err
	}
	c.Role = role
	err = r.db.QueryRowContext(ctx, `
		SELECT name, COALESCE(bin, ''), COALESCE(city, ''), COALESCE(legal_address, ''), COALESCE(actual_address, ''),
			COALESCE(work_phone, ''), COALESCE(representative_name, ''), is_buyer, is_supplier, c.status, v.status
		FROM companies c JOIN company_verification v ON v.company_id = c.id WHERE c.id = $1`, id).
		Scan(&c.Name, &c.BIN, &c.City, &c.LegalAddress, &c.ActualAddress, &c.WorkPhone, &c.Representative, &c.IsBuyer, &c.IsSupplier, &c.Status, &c.Verification)
	return c, err
}

func (r *Repository) update(ctx context.Context, p auth.Principal, id int64, d Details) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := Authorize(ctx, tx, p, id, true, true); err != nil {
			return err
		}
		if !d.IsSupplier {
			var has bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM offers WHERE supplier_company_id = $1)`, id).Scan(&has); err != nil {
				return err
			}
			if has {
				return errHasOffers
			}
		}
		if err := keyChange(ctx, tx, p, id, d); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE companies SET name = $2, bin = $3, city = $4, legal_address = $5, actual_address = $6,
				work_phone = $7, representative_name = $8, is_buyer = $9, is_supplier = $10
			WHERE id = $1`,
			id, d.Name, d.BIN, d.City, d.LegalAddress, d.ActualAddress, d.WorkPhone, d.Representative, d.IsBuyer, d.IsSupplier)
		if database.UniqueViolation(err) == "companies_bin_key" {
			return errBINTaken
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: id, Action: "company.updated"})
	})
}

// keyChange refuses to change key details during a review and records that a
// verified company loses its verification. The companies_verification_guard
// trigger enforces the same rule and clears verified_request_id.
func keyChange(ctx context.Context, tx *sql.Tx, p auth.Principal, id int64, d Details) error {
	var old Details
	var verified sql.NullInt64
	var inReview bool
	err := tx.QueryRowContext(ctx, `
		SELECT name, COALESCE(bin, ''), COALESCE(legal_address, ''), COALESCE(representative_name, ''), is_supplier, verified_request_id,
			EXISTS (SELECT 1 FROM verification_requests WHERE company_id = c.id AND status = 'in_review')
		FROM companies c WHERE id = $1`, id).
		Scan(&old.Name, &old.BIN, &old.LegalAddress, &old.Representative, &old.IsSupplier, &verified, &inReview)
	if err != nil {
		return err
	}
	type change struct {
		Field string `json:"field"`
		Old   string `json:"old"`
		New   string `json:"new"`
	}
	var changed []change
	for _, f := range []change{
		{"name", old.Name, d.Name}, {"bin", old.BIN, d.BIN}, {"legal_address", old.LegalAddress, d.LegalAddress},
		{"representative_name", old.Representative, d.Representative},
	} {
		if f.Old != f.New {
			changed = append(changed, f)
		}
	}
	if d.IsSupplier && !old.IsSupplier {
		changed = append(changed, change{"is_supplier", "no", "yes"})
	}
	if len(changed) == 0 {
		return nil
	}
	if inReview {
		return errInReview
	}
	if !verified.Valid {
		return nil
	}
	details, err := json.Marshal(map[string]any{"changed": changed})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO verification_events (request_id, company_id, submission_no, actor_user_id, actor_name, action, from_status, to_status, details)
		SELECT r.id, r.company_id, r.submission_no, u.id, u.full_name, 'verification_lost', 'approved', 'reverification', $3::jsonb
		FROM verification_requests r, users u WHERE r.id = $1 AND u.id = $2`, verified.Int64, p.UserID, string(details))
	return err
}

func (r *Repository) members(ctx context.Context, p auth.Principal, id int64) ([]Member, error) {
	role, err := Authorize(ctx, r.db, p, id, false, false)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.full_name, cm.role, CASE WHEN $3 THEN u.email END, CASE WHEN $3 THEN u.phone END, u.id = $2
		FROM company_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.company_id = $1 ORDER BY cm.role DESC, u.full_name, u.id`, id, p.UserID, role == RoleOwner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Role, &m.Email, &m.Phone, &m.IsMe); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// keepOwner fails when userID is the only owner. The company row is already locked.
func keepOwner(ctx context.Context, tx *sql.Tx, companyID, userID int64) error {
	var ok bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM company_members WHERE company_id = $1 AND role = 'owner' AND user_id <> $2)`,
		companyID, userID).Scan(&ok)
	if err == nil && !ok {
		return errLastOwner
	}
	return err
}

func (r *Repository) setRole(ctx context.Context, p auth.Principal, id, userID int64, role string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := Authorize(ctx, tx, p, id, true, true); err != nil {
			return err
		}
		if role != RoleOwner {
			if err := keepOwner(ctx, tx, id, userID); err != nil {
				return err
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE company_members SET role = $3 WHERE company_id = $1 AND user_id = $2`, id, userID, role)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errMemberNotFound
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: id, TargetUserID: userID,
			Action: "company.member_role", Details: map[string]any{"role": role}})
	})
}

// removeMember lets owners remove anyone and members remove themselves.
func (r *Repository) removeMember(ctx context.Context, p auth.Principal, id, userID int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := Authorize(ctx, tx, p, id, userID != p.UserID, true); err != nil {
			return err
		}
		if err := keepOwner(ctx, tx, id, userID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM company_members WHERE company_id = $1 AND user_id = $2`, id, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errMemberNotFound
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: id, TargetUserID: userID, Action: "company.member_removed"})
	})
}

func (r *Repository) invitations(ctx context.Context, p auth.Principal, id int64) ([]Invitation, error) {
	if _, err := Authorize(ctx, r.db, p, id, true, false); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, contact, expires_at FROM company_invitations
		WHERE company_id = $1 AND status = 'pending' AND expires_at > now() ORDER BY id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		if err := rows.Scan(&i.ID, &i.Contact, &i.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *Repository) invite(ctx context.Context, p auth.Principal, id int64, kind, contact string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := Authorize(ctx, tx, p, id, true, true); err != nil {
			return err
		}
		// Expired invitations must not block a new one.
		if _, err := tx.ExecContext(ctx, `
			UPDATE company_invitations SET status = 'cancelled'
			WHERE company_id = $1 AND status = 'pending' AND expires_at <= now()`, id); err != nil {
			return err
		}
		var pending int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM company_invitations WHERE company_id = $1 AND status = 'pending'`, id).Scan(&pending); err != nil {
			return err
		}
		if pending >= maxPendingInvites {
			return errInvited
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO company_invitations (company_id, contact_type, contact, invited_by, expires_at)
			VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5))`, id, kind, contact, p.UserID, invitationLifetime.Seconds())
		if database.UniqueViolation(err) != "" {
			return errInvited
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: id, Action: "company.invited"})
	})
}

func (r *Repository) cancelInvitation(ctx context.Context, p auth.Principal, id, invitationID int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := Authorize(ctx, tx, p, id, true, true); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE company_invitations SET status = 'cancelled' WHERE id = $2 AND company_id = $1 AND status = 'pending'`, id, invitationID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errMemberNotFound
		}
		return nil
	})
}

func (r *Repository) myInvitations(ctx context.Context, userID int64) ([]MyInvitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, c.name, c.bin, inviter.full_name, i.expires_at
		FROM company_invitations i
		JOIN users u ON u.id = $1 AND i.contact IN (u.email, u.phone)
		JOIN companies c ON c.id = i.company_id
		LEFT JOIN users inviter ON inviter.id = i.invited_by
		WHERE i.status = 'pending' AND i.expires_at > now() ORDER BY i.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MyInvitation{}
	for rows.Next() {
		var i MyInvitation
		if err := rows.Scan(&i.ID, &i.CompanyName, &i.BIN, &i.InvitedBy, &i.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// answerInvitation works only for an invitation sent to a confirmed contact of the user.
func (r *Repository) answerInvitation(ctx context.Context, p auth.Principal, invitationID int64, accept bool) error {
	status := "declined"
	if accept {
		status = "accepted"
	}
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var companyID int64
		err := tx.QueryRowContext(ctx, `
			UPDATE company_invitations i SET status = $3
			FROM users u, companies c
			WHERE i.id = $1 AND u.id = $2 AND i.contact IN (u.email, u.phone)
				AND c.id = i.company_id AND c.status = 'active'
				AND i.status = 'pending' AND i.expires_at > now()
			RETURNING i.company_id`, invitationID, p.UserID, status).Scan(&companyID)
		if errors.Is(err, sql.ErrNoRows) {
			return errMemberNotFound
		}
		if err != nil {
			return err
		}
		if !accept {
			return nil
		}
		if err := addMember(ctx, tx, companyID, p.UserID, RoleEmployee); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE sessions SET active_company_id = $2 WHERE id = $1 AND active_company_id IS NULL`, p.SessionID, companyID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: companyID, TargetUserID: p.UserID, Action: "company.member_joined"})
	})
}
