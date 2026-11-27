package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/database"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// userByContact returns nil when nobody has the contact.
func (r *Repository) userByContact(ctx context.Context, kind, contact string) (*user, error) {
	var u user
	err := r.db.QueryRowContext(ctx, `
		SELECT id, password_hash, status FROM users
		WHERE ($1 = 'email' AND email = $2) OR ($1 = 'phone' AND phone = $2)`, kind, contact).
		Scan(&u.ID, &u.PasswordHash, &u.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user by contact: %w", err)
	}
	return &u, nil
}

func (r *Repository) passwordHash(ctx context.Context, userID int64) (h string, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&h)
	return h, err
}

// insertUser stores a user whose contact has been confirmed.
func insertUser(ctx context.Context, q database.Querier, fullName, kind, contact, hash string) (id int64, err error) {
	err = q.QueryRowContext(ctx, `
		INSERT INTO users (full_name, password_hash, email, email_verified_at, phone, phone_verified_at)
		VALUES ($1, $2,
			CASE WHEN $3 = 'email' THEN $4 END, CASE WHEN $3 = 'email' THEN now() END,
			CASE WHEN $3 = 'phone' THEN $4 END, CASE WHEN $3 = 'phone' THEN now() END)
		RETURNING id`, fullName, hash, kind, contact).Scan(&id)
	if database.UniqueViolation(err) != "" {
		return 0, errContactTaken
	}
	return id, err
}

func (r *Repository) createChallenge(ctx context.Context, tokenHash []byte, c challenge) error {
	// Opportunistic cleanup keeps short-lived personal data from piling up.
	for _, q := range []string{
		`DELETE FROM auth_challenges WHERE expires_at < now() - interval '1 day'`,
		`DELETE FROM rate_limits WHERE expires_at < now()`,
		`DELETE FROM sessions WHERE expires_at < now()`,
	} {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("cleanup: %w", err)
		}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_challenges (token_hash, purpose, contact_type, contact, user_id, full_name, password_hash, code_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		tokenHash, c.Purpose, c.ContactType, c.Contact, c.UserID, c.FullName, c.PasswordHash, c.CodeHash, challengeTTL.Seconds())
	if err != nil {
		return fmt.Errorf("create challenge: %w", err)
	}
	return nil
}

// attempt atomically spends one attempt; errBadCode if none is left.
func (r *Repository) attempt(ctx context.Context, tokenHash []byte, purpose string) (challenge, error) {
	var c challenge
	err := r.db.QueryRowContext(ctx, `
		UPDATE auth_challenges SET attempts = attempts + 1
		WHERE token_hash = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > now() AND attempts < $3
		RETURNING id, purpose, contact_type, contact, user_id, full_name, password_hash, code_hash`,
		tokenHash, purpose, maxCodeAttempts).
		Scan(&c.ID, &c.Purpose, &c.ContactType, &c.Contact, &c.UserID, &c.FullName, &c.PasswordHash, &c.CodeHash)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errBadCode
	}
	return c, err
}

// consume makes the code single-use even for concurrent requests.
func consume(ctx context.Context, tx *sql.Tx, id int64) error {
	res, err := tx.ExecContext(ctx, `UPDATE auth_challenges SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errBadCode
	}
	return nil
}

func (r *Repository) registerUser(ctx context.Context, c challenge) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := consume(ctx, tx, c.ID); err != nil {
			return err
		}
		if id, err = insertUser(ctx, tx, *c.FullName, c.ContactType, c.Contact, *c.PasswordHash); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: id, TargetUserID: id, Action: "user.registered"})
	})
	return id, err
}

func (r *Repository) resetPassword(ctx context.Context, c challenge, hash string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := consume(ctx, tx, c.ID); err != nil {
			return err
		}
		// The contact may have been replaced since the code was issued.
		res, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = $2
			WHERE id = $1 AND status = 'active' AND (email = $3 OR phone = $3)`, *c.UserID, hash, c.Contact)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errBadCode
		}
		if _, err := tx.ExecContext(ctx, `UPDATE auth_challenges SET consumed_at = now() WHERE user_id = $1 AND consumed_at IS NULL`, *c.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, *c.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: *c.UserID, TargetUserID: *c.UserID, Action: "user.password_reset"})
	})
}

func (r *Repository) changePassword(ctx context.Context, p Principal, hash string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, p.UserID, hash); err != nil {
			return err
		}
		// Outstanding reset codes and other sessions stop working.
		if _, err := tx.ExecContext(ctx, `UPDATE auth_challenges SET consumed_at = now() WHERE user_id = $1 AND consumed_at IS NULL`, p.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, p.UserID, p.SessionID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, TargetUserID: p.UserID, Action: "user.password_changed"})
	})
}

func (r *Repository) setContact(ctx context.Context, c challenge) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := consume(ctx, tx, c.ID); err != nil {
			return err
		}
		q := `UPDATE users SET email = $2, email_verified_at = now() WHERE id = $1`
		if c.ContactType == "phone" {
			q = `UPDATE users SET phone = $2, phone_verified_at = now() WHERE id = $1`
		}
		if _, err := tx.ExecContext(ctx, q, *c.UserID, c.Contact); err != nil {
			if database.UniqueViolation(err) != "" {
				return errContactTaken
			}
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: *c.UserID, TargetUserID: *c.UserID,
			Action: "user.contact_changed", Details: map[string]any{"type": c.ContactType}})
	})
}

// createSession selects the company automatically when the user has exactly one.
func (r *Repository) createSession(ctx context.Context, userID int64, tokenHash []byte, userAgent string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, user_agent, expires_at, active_company_id)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4),
			(SELECT min(company_id) FROM company_members WHERE user_id = $1 HAVING count(*) = 1))`,
		userID, tokenHash, userAgent, sessionLifetime.Seconds())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *Repository) principal(ctx context.Context, tokenHash []byte) (*Principal, error) {
	var p Principal
	var roles, perms string
	var stale bool
	err := r.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, COALESCE(s.active_company_id, 0), s.last_seen_at < now() - interval '5 minutes',
			COALESCE((SELECT string_agg(role, ',' ORDER BY role) FROM platform_roles pr WHERE pr.user_id = u.id), ''),
			COALESCE((SELECT string_agg(permission, ',' ORDER BY permission) FROM platform_permissions pp WHERE pp.user_id = u.id), '')
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.status = 'active'
			AND s.last_seen_at > now() - make_interval(secs => $2)`, tokenHash, sessionIdle.Seconds()).
		Scan(&p.SessionID, &p.UserID, &p.ActiveCompanyID, &stale, &roles, &perms)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	p.Roles, p.Permissions = splitRoles(roles), splitRoles(perms)
	if stale {
		if _, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = now() WHERE id = $1`, p.SessionID); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

func splitRoles(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func (r *Repository) deleteSessionByToken(ctx context.Context, tokenHash []byte) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *Repository) deleteSession(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) deleteOtherSessions(ctx context.Context, p Principal) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, p.UserID, p.SessionID)
	return err
}

func (r *Repository) sessions(ctx context.Context, p Principal) ([]Session, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_agent, created_at, last_seen_at, id = $2 FROM sessions
		WHERE user_id = $1 AND expires_at > now() AND last_seen_at > now() - make_interval(secs => $3)
		ORDER BY last_seen_at DESC, id DESC`, p.UserID, p.SessionID, sessionIdle.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserAgent, &s.CreatedAt, &s.LastSeenAt, &s.Current); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) me(ctx context.Context, p Principal) (Me, error) {
	m := Me{PlatformRoles: p.Roles, Permissions: p.Permissions, Companies: []Membership{}}
	if p.ActiveCompanyID != 0 {
		m.ActiveCompanyID = &p.ActiveCompanyID
	}
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.full_name, u.email, u.phone,
			(SELECT count(*) FROM company_invitations i
			 WHERE i.status = 'pending' AND i.expires_at > now() AND i.contact IN (u.email, u.phone))
		FROM users u WHERE u.id = $1`, p.UserID).
		Scan(&m.ID, &m.FullName, &m.Email, &m.Phone, &m.PendingInvitations)
	if err != nil {
		return m, fmt.Errorf("me: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, cm.role, c.is_buyer, c.is_supplier, c.status, v.status
		FROM company_members cm JOIN companies c ON c.id = cm.company_id
		JOIN company_verification v ON v.company_id = c.id
		WHERE cm.user_id = $1 ORDER BY c.name, c.id`, p.UserID)
	if err != nil {
		return m, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Membership
		if err := rows.Scan(&x.ID, &x.Name, &x.Role, &x.IsBuyer, &x.IsSupplier, &x.Status, &x.Verification); err != nil {
			return m, err
		}
		m.Companies = append(m.Companies, x)
	}
	return m, rows.Err()
}

func (r *Repository) setFullName(ctx context.Context, userID int64, name string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET full_name = $2 WHERE id = $1`, userID, name)
	return err
}

// setActiveCompany switches only to a company the user belongs to.
func (r *Repository) setActiveCompany(ctx context.Context, p Principal, companyID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE sessions s SET active_company_id = $3
		WHERE s.id = $1 AND EXISTS (SELECT 1 FROM company_members WHERE company_id = $3 AND user_id = $2)`,
		p.SessionID, p.UserID, companyID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) listUsers(ctx context.Context, q string) ([]AdminUser, error) {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.full_name, u.email, u.phone,
			COALESCE((SELECT string_agg(role, ',' ORDER BY role) FROM platform_roles pr WHERE pr.user_id = u.id), ''),
			COALESCE((SELECT string_agg(permission, ',' ORDER BY permission) FROM platform_permissions pp WHERE pp.user_id = u.id), '')
		FROM users u
		WHERE u.full_name ILIKE $1 OR u.email ILIKE $1 OR u.phone ILIKE $1
		ORDER BY u.id DESC LIMIT 50`, "%"+esc+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminUser{}
	for rows.Next() {
		var u AdminUser
		var roles, perms string
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &roles, &perms); err != nil {
			return nil, err
		}
		u.PlatformRoles, u.Permissions = splitRoles(roles), splitRoles(perms)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) setRole(ctx context.Context, actorID, userID int64, role string, enabled bool) error {
	return r.setGrant(ctx, actorID, userID, "platform_role", "role", role, enabled,
		`INSERT INTO platform_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		`DELETE FROM platform_roles WHERE user_id = $1 AND role = $2`)
}

func (r *Repository) setPermission(ctx context.Context, actorID, userID int64, permission string, enabled bool) error {
	return r.setGrant(ctx, actorID, userID, "platform_permission", "permission", permission, enabled,
		`INSERT INTO platform_permissions (user_id, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		`DELETE FROM platform_permissions WHERE user_id = $1 AND permission = $2`)
}

// setGrant changes a role or permission and records it. The queries are constants.
func (r *Repository) setGrant(ctx context.Context, actorID, userID int64, kind, key, value string, enabled bool, grant, revoke string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errNotFound
		}
		q, action := grant, kind+".granted"
		if !enabled {
			q, action = revoke, kind+".revoked"
		}
		res, err := tx.ExecContext(ctx, q, userID, value)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // already in the requested state
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: actorID, TargetUserID: userID, Action: action,
			Details: map[string]any{key: value}})
	})
}

// HasPermission re-checks a permission inside a transaction, like HasRole.
func HasPermission(ctx context.Context, q database.Querier, userID int64, permission string) (ok bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM platform_permissions WHERE user_id = $1 AND permission = $2)`, userID, permission).Scan(&ok)
	return ok, err
}

// HasRole re-checks a platform role inside a transaction, so that a revocation
// takes effect even for a request already in progress.
func HasRole(ctx context.Context, q database.Querier, userID int64, role string) (ok bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM platform_roles WHERE user_id = $1 AND role = $2)`, userID, role).Scan(&ok)
	return ok, err
}

// CreateAdmin is used only by the operator command.
func (r *Repository) CreateAdmin(ctx context.Context, fullName, kind, contact, hash string) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if id, err = insertUser(ctx, tx, fullName, kind, contact, hash); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_roles (user_id, role) VALUES ($1, $2)`, id, RoleAdmin); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{TargetUserID: id, Action: "platform_role.granted",
			Details: map[string]any{"role": RoleAdmin, "source": "admin command"}})
	})
	return id, err
}

// SetAdmin grants or revokes the administrator role of an existing user (operator command).
func (r *Repository) SetAdmin(ctx context.Context, kind, contact string, enabled bool) error {
	u, err := r.userByContact(ctx, kind, contact)
	if err != nil {
		return err
	}
	if u == nil {
		return errNotFound
	}
	return r.setRole(ctx, 0, u.ID, RoleAdmin, enabled)
}
