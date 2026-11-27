package verification

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/companies"
	"github.com/qurylys/qurylys/backend/internal/database"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const requestColumns = `r.id, r.company_id, r.status, r.submission_no, r.comment, r.snapshot, r.submitted_at, r.decided_at`

func scanRequest(row *sql.Row) (*request, error) {
	var r request
	var snap []byte
	err := row.Scan(&r.ID, &r.CompanyID, &r.Status, &r.SubmissionNo, &r.Comment, &snap, &r.SubmittedAt, &r.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err == nil && snap != nil {
		r.Snapshot = &Requisites{}
		err = json.Unmarshal(snap, r.Snapshot)
	}
	return &r, err
}

// latest returns the newest request of the company, nil if none. Callers that
// change it hold the company row lock (companies.Authorize with change=true).
func latest(ctx context.Context, q database.Querier, companyID int64) (*request, error) {
	return scanRequest(q.QueryRowContext(ctx, `SELECT `+requestColumns+`
		FROM verification_requests r WHERE r.company_id = $1 ORDER BY r.id DESC LIMIT 1`, companyID))
}

func requisites(ctx context.Context, q database.Querier, companyID int64) (r Requisites, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT name, COALESCE(bin, ''), COALESCE(city, ''), COALESCE(legal_address, ''), COALESCE(actual_address, ''),
			COALESCE(work_phone, ''), COALESCE(representative_name, ''), is_buyer, is_supplier
		FROM companies WHERE id = $1`, companyID).
		Scan(&r.Name, &r.BIN, &r.City, &r.LegalAddress, &r.ActualAddress, &r.WorkPhone, &r.Representative, &r.IsBuyer, &r.IsSupplier)
	return r, err
}

func documents(ctx context.Context, q database.Querier, where string, args ...any) ([]Document, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT d.id, d.kind, d.filename, d.content_type, d.size, d.created_at
		FROM verification_documents d WHERE `+where+` ORDER BY d.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Kind, &d.Filename, &d.ContentType, &d.Size, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Documents the owner is editing, and documents belonging to submission $2.
const (
	currentDocs   = `d.request_id = $1 AND d.removed_after IS NULL`
	submittedDocs = `d.request_id = $1 AND d.added_after < $2 AND (d.removed_after IS NULL OR d.removed_after >= $2)`
)

type event struct {
	req     *request
	actorID int64
	action  string
	from    string
	to      string
	comment string
	details any
}

func writeEvent(ctx context.Context, q database.Querier, e event) error {
	d, err := json.Marshal(e.details)
	if err != nil || e.details == nil {
		d = []byte("{}")
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO verification_events (request_id, company_id, submission_no, actor_user_id, actor_name, action, from_status, to_status, comment, details)
		SELECT $1, $2, $3, u.id, u.full_name, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9::jsonb FROM users u WHERE u.id = $4`,
		e.req.ID, e.req.CompanyID, e.req.SubmissionNo, e.actorID, e.action, e.from, e.to, e.comment, string(d)); err != nil {
		return fmt.Errorf("verification event: %w", err)
	}
	return nil
}

// events lists the history; forCompany hides the names of reviewers.
func events(ctx context.Context, q database.Querier, companyID int64, forCompany bool) ([]Event, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, submission_no, action,
			CASE WHEN $2 AND action IN ('approved', 'changes_requested', 'rejected') THEN '' ELSE actor_name END,
			comment, COALESCE(details -> 'changed', '[]'::jsonb)::text, created_at
		FROM verification_events WHERE company_id = $1 ORDER BY id DESC LIMIT 200`, companyID, forCompany)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var changed string
		if err := rows.Scan(&e.ID, &e.SubmissionNo, &e.Action, &e.Actor, &e.Comment, &changed, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(changed), &e.Changed); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) companyView(ctx context.Context, p auth.Principal, companyID int64) (CompanyView, error) {
	v := CompanyView{Documents: []Document{}, MaxFileSize: MaxFileSize, MaxDocuments: maxDocuments}
	role, err := companies.Authorize(ctx, r.db, p, companyID, false, false)
	if err != nil {
		return v, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT status FROM company_verification WHERE company_id = $1`, companyID).Scan(&v.Status); err != nil {
		return v, err
	}
	if v.Requisites, err = requisites(ctx, r.db, companyID); err != nil {
		return v, err
	}
	req, err := latest(ctx, r.db, companyID)
	if err != nil {
		return v, err
	}
	if req != nil {
		v.Comment, v.SubmittedAt, v.DecidedAt = req.Comment, req.SubmittedAt, req.DecidedAt
		if req.Status == StatusInReview {
			v.Requisites = *req.Snapshot
		}
		if role == companies.RoleOwner {
			v.CanEdit = req.Status == StatusDraft || req.Status == StatusNeedsChanges
			if v.Documents, err = documents(ctx, r.db, currentDocs, req.ID); err != nil {
				return v, err
			}
		}
	}
	v.Events, err = events(ctx, r.db, companyID, true)
	return v, err
}

func (r *Repository) createDraft(ctx context.Context, p auth.Principal, companyID int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := companies.Authorize(ctx, tx, p, companyID, true, true); err != nil {
			return err
		}
		var verified bool
		if err := tx.QueryRowContext(ctx, `SELECT verified_request_id IS NOT NULL FROM companies WHERE id = $1`, companyID).Scan(&verified); err != nil {
			return err
		}
		if verified {
			return errVerified
		}
		req := &request{CompanyID: companyID}
		err := tx.QueryRowContext(ctx, `INSERT INTO verification_requests (company_id) VALUES ($1) RETURNING id`, companyID).Scan(&req.ID)
		if database.UniqueViolation(err) != "" {
			return errOpen
		}
		if err != nil {
			return err
		}
		return writeEvent(ctx, tx, event{req: req, actorID: p.UserID, action: "created", to: StatusDraft})
	})
}

// editable locks the company and returns its request if the owner may change it.
func editable(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID int64) (*request, error) {
	if _, err := companies.Authorize(ctx, tx, p, companyID, true, true); err != nil {
		return nil, err
	}
	req, err := latest(ctx, tx, companyID)
	if err != nil {
		return nil, err
	}
	if req == nil || (req.Status != StatusDraft && req.Status != StatusNeedsChanges) {
		return nil, errNotEditable
	}
	return req, nil
}

func (r *Repository) addDocument(ctx context.Context, p auth.Principal, companyID int64, kind, filename, contentType string, data []byte) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := editable(ctx, tx, p, companyID)
		if err != nil {
			return err
		}
		var current, stored int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*) FILTER (WHERE removed_after IS NULL), count(*)
			FROM verification_documents WHERE request_id = $1`, req.ID).Scan(&current, &stored); err != nil {
			return err
		}
		if current >= maxDocuments || stored >= maxStored {
			return errTooMany
		}
		sum := sha256.Sum256(data)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO verification_documents (request_id, kind, filename, content_type, size, sha256, data, added_after)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			req.ID, kind, filename, contentType, len(data), sum[:], data, req.SubmissionNo)
		return err
	})
}

// removeDocument deletes a file that was never submitted; submitted files are
// only hidden, because the history of decisions refers to them.
func (r *Repository) removeDocument(ctx context.Context, p auth.Principal, companyID, docID int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := editable(ctx, tx, p, companyID)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			DELETE FROM verification_documents WHERE id = $1 AND request_id = $2 AND added_after = $3 AND removed_after IS NULL`,
			docID, req.ID, req.SubmissionNo)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return nil
		}
		// removed_after = N keeps the file visible in submission N and hides it from N+1 on.
		res, err = tx.ExecContext(ctx, `
			UPDATE verification_documents SET removed_after = $3 WHERE id = $1 AND request_id = $2 AND removed_after IS NULL`,
			docID, req.ID, req.SubmissionNo)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNotFound
		}
		return nil
	})
}

func (r *Repository) submit(ctx context.Context, p auth.Principal, companyID int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := editable(ctx, tx, p, companyID)
		if err != nil {
			return err
		}
		reqs, err := requisites(ctx, tx, companyID)
		if err != nil {
			return err
		}
		if !reqs.complete() {
			return errIncomplete
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, kind, filename, sha256 FROM verification_documents d WHERE `+currentDocs+` ORDER BY id`, req.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type docRef struct {
			ID     int64  `json:"id"`
			Kind   string `json:"kind"`
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		}
		docs, hasRegistration := []docRef{}, false
		for rows.Next() {
			var d docRef
			var sum []byte
			if err := rows.Scan(&d.ID, &d.Kind, &d.Name, &sum); err != nil {
				return err
			}
			d.SHA256 = hex.EncodeToString(sum)
			docs, hasRegistration = append(docs, d), hasRegistration || d.Kind == "registration"
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if !hasRegistration {
			return errDocRequired
		}
		snap, err := json.Marshal(reqs)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE verification_requests SET status = 'in_review', submission_no = submission_no + 1,
				snapshot = $2::jsonb, submitted_at = now(), decided_at = NULL, comment = ''
			WHERE id = $1`, req.ID, string(snap)); err != nil {
			return err
		}
		var changed []Change
		if req.Snapshot != nil {
			changed = diff(*req.Snapshot, reqs)
		}
		from := req.Status
		req.SubmissionNo++
		return writeEvent(ctx, tx, event{req: req, actorID: p.UserID, action: "submitted", from: from, to: StatusInReview,
			details: map[string]any{"requisites": reqs, "documents": docs, "changed": nonNil(changed)}})
	})
}

func nonNil(c []Change) []Change {
	if c == nil {
		return []Change{}
	}
	return c
}

func diff(a, b Requisites) []Change {
	var out []Change
	yes := map[bool]string{true: "yes", false: "no"}
	for _, f := range []Change{
		{"name", a.Name, b.Name}, {"bin", a.BIN, b.BIN}, {"city", a.City, b.City},
		{"legal_address", a.LegalAddress, b.LegalAddress}, {"actual_address", a.ActualAddress, b.ActualAddress},
		{"work_phone", a.WorkPhone, b.WorkPhone}, {"representative_name", a.Representative, b.Representative},
		{"is_buyer", yes[a.IsBuyer], yes[b.IsBuyer]}, {"is_supplier", yes[a.IsSupplier], yes[b.IsSupplier]},
	} {
		if f.Old != f.New {
			out = append(out, f)
		}
	}
	return out
}

type file struct {
	Document
	Data []byte
}

func scanFile(row *sql.Row) (*file, error) {
	var f file
	err := row.Scan(&f.ID, &f.Filename, &f.ContentType, &f.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return &f, err
}

// companyFile returns a current document of the owner's own company.
func (r *Repository) companyFile(ctx context.Context, p auth.Principal, companyID, docID int64) (*file, error) {
	if _, err := companies.Authorize(ctx, r.db, p, companyID, true, false); err != nil {
		return nil, err
	}
	return scanFile(r.db.QueryRowContext(ctx, `
		SELECT d.id, d.filename, d.content_type, d.data
		FROM verification_documents d JOIN verification_requests r ON r.id = d.request_id
		WHERE d.id = $1 AND r.company_id = $2 AND d.removed_after IS NULL`, docID, companyID))
}

// conflictSQL is true when user $1 is or was a member of company c, or acted for
// it. Leaving the company does not lift the conflict.
const conflictSQL = `(` + companies.InvolvedSQL + `
	OR EXISTS (SELECT 1 FROM verification_events e WHERE e.company_id = c.id AND e.actor_user_id = $1
		AND e.action IN ('created', 'submitted')))`

// list shows submitted requests only; drafts are private to the company.
func (r *Repository) list(ctx context.Context, p auth.Principal, status string, page int) (ListPage, error) {
	out := ListPage{Items: []ListItem{}, Page: page}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.snapshot ->> 'name', r.snapshot ->> 'bin', r.snapshot ->> 'city', r.status, r.submission_no,
			r.submitted_at, r.decided_at, `+conflictSQL+`, count(*) OVER ()
		FROM verification_requests r JOIN companies c ON c.id = r.company_id
		WHERE r.status <> 'draft' AND ($2 = '' OR r.status = $2)
		ORDER BY (r.status = 'in_review') DESC, CASE WHEN r.status = 'in_review' THEN r.submitted_at END ASC, r.updated_at DESC, r.id DESC
		LIMIT $3 OFFSET $4`, p.UserID, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i ListItem
		if err := rows.Scan(&i.ID, &i.CompanyName, &i.BIN, &i.City, &i.Status, &i.SubmissionNo, &i.SubmittedAt, &i.DecidedAt, &i.Conflict, &out.Total); err != nil {
			return out, err
		}
		out.Items = append(out.Items, i)
	}
	out.TotalPages = (out.Total + pageSize - 1) / pageSize
	return out, rows.Err()
}

// reviewable returns a submitted request the reviewer has no conflict with.
func reviewable(ctx context.Context, q database.Querier, p auth.Principal, id int64) (*request, error) {
	req, err := scanRequest(q.QueryRowContext(ctx, `SELECT `+requestColumns+`
		FROM verification_requests r WHERE r.id = $1 AND r.status <> 'draft'`, id))
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errNotFound
	}
	var conflict bool
	if err := q.QueryRowContext(ctx, `SELECT `+conflictSQL+` FROM companies c WHERE c.id = $2`, p.UserID, req.CompanyID).Scan(&conflict); err != nil {
		return nil, err
	}
	if conflict {
		return nil, errConflict
	}
	return req, nil
}

func (r *Repository) reviewView(ctx context.Context, p auth.Principal, id int64) (ReviewView, error) {
	var v ReviewView
	req, err := reviewable(ctx, r.db, p, id)
	if err != nil {
		return v, err
	}
	v = ReviewView{ID: req.ID, Status: req.Status, SubmissionNo: req.SubmissionNo, Comment: req.Comment,
		SubmittedAt: req.SubmittedAt, DecidedAt: req.DecidedAt, Requisites: *req.Snapshot}
	if v.Documents, err = documents(ctx, r.db, submittedDocs, req.ID, req.SubmissionNo); err != nil {
		return v, err
	}
	v.Events, err = events(ctx, r.db, req.CompanyID, false)
	return v, err
}

func (r *Repository) reviewFile(ctx context.Context, p auth.Principal, id, docID int64) (*file, error) {
	req, err := reviewable(ctx, r.db, p, id)
	if err != nil {
		return nil, err
	}
	return scanFile(r.db.QueryRowContext(ctx, `
		SELECT d.id, d.filename, d.content_type, d.data FROM verification_documents d
		WHERE `+submittedDocs+` AND d.id = $3`, req.ID, req.SubmissionNo, docID))
}

// decide applies a decision to exactly the submission the reviewer saw and
// returns the owners' confirmed emails. Lock order: company, then request.
func (r *Repository) decide(ctx context.Context, p auth.Principal, id int64, submissionNo int, to, action, comment string) (emails []string, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var companyID int64
		err := tx.QueryRowContext(ctx, `SELECT company_id FROM verification_requests WHERE id = $1 AND status <> 'draft'`, id).Scan(&companyID)
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM companies WHERE id = $1 FOR UPDATE`, companyID).Scan(&status); err != nil {
			return err
		}
		// Re-read under the lock: membership, role and status may have changed meanwhile.
		req, err := reviewable(ctx, tx, p, id)
		if err != nil {
			return err
		}
		if ok, err := auth.HasRole(ctx, tx, p.UserID, auth.RoleMRX); err != nil || !ok {
			if err == nil {
				err = errNotFound
			}
			return err
		}
		if req.Status != StatusInReview || req.SubmissionNo != submissionNo {
			return errStale
		}
		if to == StatusApproved {
			if status != "active" {
				return errSuspended
			}
			live, err := requisites(ctx, tx, companyID)
			if err != nil {
				return err
			}
			if keyChanged(*req.Snapshot, live) {
				return errStale
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE verification_requests SET status = $2, comment = $3, decided_at = now() WHERE id = $1`, id, to, comment); err != nil {
			return err
		}
		if to == StatusApproved {
			if _, err := tx.ExecContext(ctx, `UPDATE companies SET verified_request_id = $2 WHERE id = $1`, companyID, id); err != nil {
				return err
			}
		}
		if err := writeEvent(ctx, tx, event{req: req, actorID: p.UserID, action: action, from: StatusInReview, to: to, comment: comment}); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT u.email FROM company_members m JOIN users u ON u.id = m.user_id
			WHERE m.company_id = $1 AND m.role = 'owner' AND u.status = 'active' AND u.email IS NOT NULL`, companyID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err != nil {
				return err
			}
			emails = append(emails, e)
		}
		return rows.Err()
	})
	return emails, err
}

// keyChanged mirrors companies_verification_guard in migration 00008.
func keyChanged(a, b Requisites) bool {
	return a.Name != b.Name || a.BIN != b.BIN || a.LegalAddress != b.LegalAddress || a.Representative != b.Representative ||
		(b.IsSupplier && !a.IsSupplier)
}

func (r *Repository) checkOwner(ctx context.Context, p auth.Principal, companyID int64) error {
	_, err := companies.Authorize(ctx, r.db, p, companyID, true, false)
	return err
}
