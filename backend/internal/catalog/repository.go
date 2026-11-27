package catalog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/companies"
	"github.com/qurylys/qurylys/backend/internal/database"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const columns = `r.id, r.status, r.submission_no, r.category_id, r.name_kk, r.name_ru, r.brand, r.unit,
	r.description_kk, r.description_ru, r.gtin, r.gtin_source_kind, r.gtin_source,
	COALESCE(cat.name_kk, ''), COALESCE(cat.name_ru, ''), r.gtin_confirmed_at IS NOT NULL, r.comment,
	r.product_id, p.catalog_no, r.submitted_at, r.decided_at, r.updated_at, r.company_id, c.name, r.author_user_id`

const from = ` FROM catalog_requests r
	JOIN companies c ON c.id = r.company_id
	LEFT JOIN categories cat ON cat.id = r.category_id
	LEFT JOIN products p ON p.id = r.product_id `

// conflictSQL: user $1 wrote the request, or is or was involved in its company.
const conflictSQL = `(r.author_user_id = $1 OR ` + companies.InvolvedSQL + `)`

func dest(r *Request) []any {
	return []any{&r.ID, &r.Status, &r.SubmissionNo, &r.CategoryID, &r.NameKK, &r.NameRU, &r.Brand, &r.Unit,
		&r.DescriptionKK, &r.DescriptionRU, &r.GTIN, &r.SourceKind, &r.Source,
		&r.CategoryKK, &r.CategoryRU, &r.GTINConfirmed, &r.Comment,
		&r.ProductID, &r.CatalogNo, &r.SubmittedAt, &r.DecidedAt, &r.UpdatedAt, &r.companyID, &r.companyName, &r.authorID}
}

func one(row *sql.Row) (*Request, error) {
	var r Request
	err := row.Scan(dest(&r)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("catalog request: %w", err)
	}
	return &r, nil
}

func ownRequest(ctx context.Context, q database.Querier, companyID, id int64) (*Request, error) {
	return one(q.QueryRowContext(ctx, `SELECT `+columns+from+`WHERE r.id = $1 AND r.company_id = $2`, id, companyID))
}

func writeEvent(ctx context.Context, tx *sql.Tx, r *Request, actorID int64, action, comment string, details any) error {
	d, err := json.Marshal(details)
	if err != nil || details == nil {
		d = []byte("{}")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO catalog_events (request_id, company_id, product_id, submission_no, actor_user_id, actor_name, action, comment, details)
		SELECT $1, $2, $3, $4, u.id, u.full_name, $6, $7, $8::jsonb FROM users u WHERE u.id = $5`,
		r.ID, r.companyID, r.ProductID, r.SubmissionNo, actorID, action, comment, string(d)); err != nil {
		return fmt.Errorf("catalog event: %w", err)
	}
	return nil
}

func (r *Repository) events(ctx context.Context, requestID int64, forCompany bool) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, submission_no, action,
			CASE WHEN $2 AND action IN ('gtin_confirmed', 'changes_requested', 'approved', 'rejected') THEN '' ELSE actor_name END,
			comment, created_at
		FROM catalog_events WHERE request_id = $1 ORDER BY id DESC LIMIT 200`, requestID, forCompany)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.SubmissionNo, &e.Action, &e.Actor, &e.Comment, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) companyList(ctx context.Context, p auth.Principal, companyID int64) ([]Request, error) {
	if _, err := companies.Authorize(ctx, r.db, p, companyID, true, false); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+from+`WHERE r.company_id = $1 ORDER BY r.id DESC LIMIT 200`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		var x Request
		if err := rows.Scan(dest(&x)...); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Repository) companyGet(ctx context.Context, p auth.Principal, companyID, id int64) (*CompanyRequest, error) {
	if _, err := companies.Authorize(ctx, r.db, p, companyID, true, false); err != nil {
		return nil, err
	}
	req, err := ownRequest(ctx, r.db, companyID, id)
	if err != nil {
		return nil, err
	}
	ev, err := r.events(ctx, id, true)
	return &CompanyRequest{Request: *req, Events: ev}, err
}

// authorizeSupplier locks the company: only its owner, only while it is a supplier.
func authorizeSupplier(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID int64) error {
	if _, err := companies.Authorize(ctx, tx, p, companyID, true, true); err != nil {
		return err
	}
	var ok bool
	if err := tx.QueryRowContext(ctx, `SELECT is_supplier FROM companies WHERE id = $1`, companyID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errNotSupplier
	}
	return nil
}

func checkCategory(ctx context.Context, tx *sql.Tx, id *int64) error {
	if id == nil {
		return nil
	}
	var ok bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1 AND is_active)`, *id).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errCategory
	}
	return nil
}

func gtinTaken(ctx context.Context, q database.Querier, gtin *string) error {
	if gtin == nil {
		return nil
	}
	var taken bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE gtin = $1)`, *gtin).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return errGTINExists
	}
	return nil
}

func (r *Repository) create(ctx context.Context, p auth.Principal, companyID int64, f Fields) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := authorizeSupplier(ctx, tx, p, companyID); err != nil {
			return err
		}
		var open int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*) FROM catalog_requests WHERE company_id = $1 AND status IN ('draft', 'in_review', 'needs_changes')`, companyID).Scan(&open); err != nil {
			return err
		}
		if open >= maxOpen {
			return errTooMany
		}
		if err := checkCategory(ctx, tx, f.CategoryID); err != nil {
			return err
		}
		if err := gtinTaken(ctx, tx, f.GTIN); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO catalog_requests (company_id, author_user_id, category_id, name_kk, name_ru, brand, unit, description_kk, description_ru, gtin, gtin_source_kind, gtin_source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
			companyID, p.UserID, f.CategoryID, f.NameKK, f.NameRU, f.Brand, f.Unit, f.DescriptionKK, f.DescriptionRU, f.GTIN, f.SourceKind, f.Source).Scan(&id); err != nil {
			return fmt.Errorf("create catalog request: %w", err)
		}
		return writeEvent(ctx, tx, &Request{ID: id, companyID: companyID}, p.UserID, "created", "", nil)
	})
	return id, err
}

func editable(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID, id int64) (*Request, error) {
	if err := authorizeSupplier(ctx, tx, p, companyID); err != nil {
		return nil, err
	}
	req, err := ownRequest(ctx, tx, companyID, id)
	if err != nil {
		return nil, err
	}
	if req.Status != StatusDraft && req.Status != StatusNeedsChanges {
		return nil, errNotEditable
	}
	return req, nil
}

func (r *Repository) update(ctx context.Context, p auth.Principal, companyID, id int64, f Fields) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := editable(ctx, tx, p, companyID, id); err != nil {
			return err
		}
		if err := checkCategory(ctx, tx, f.CategoryID); err != nil {
			return err
		}
		if err := gtinTaken(ctx, tx, f.GTIN); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE catalog_requests SET category_id = $2, name_kk = $3, name_ru = $4, brand = $5, unit = $6,
				description_kk = $7, description_ru = $8, gtin = $9, gtin_source_kind = $10, gtin_source = $11
			WHERE id = $1`,
			id, f.CategoryID, f.NameKK, f.NameRU, f.Brand, f.Unit, f.DescriptionKK, f.DescriptionRU, f.GTIN, f.SourceKind, f.Source)
		return err
	})
}

func (r *Repository) submit(ctx context.Context, p auth.Principal, companyID, id int64) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := editable(ctx, tx, p, companyID, id)
		if err != nil {
			return err
		}
		if err := req.Fields.complete(); err != nil {
			return err
		}
		if err := checkCategory(ctx, tx, req.CategoryID); err != nil {
			return err
		}
		if err := gtinTaken(ctx, tx, req.GTIN); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE catalog_requests SET status = 'in_review', submission_no = submission_no + 1,
				submitted_at = now(), decided_at = NULL, comment = ''
			WHERE id = $1`, id); err != nil {
			return err
		}
		req.SubmissionNo++
		return writeEvent(ctx, tx, req, p.UserID, "submitted", "", req.Fields)
	})
}

func (r *Repository) list(ctx context.Context, p auth.Principal, status string, page int) (ListPage, error) {
	out := ListPage{Items: []ListItem{}, Page: page}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+`, `+conflictSQL+`, count(*) OVER ()`+from+`
		WHERE r.status <> 'draft' AND ($2 = '' OR r.status = $2)
		ORDER BY (r.status = 'in_review') DESC, CASE WHEN r.status = 'in_review' THEN r.submitted_at END ASC, r.updated_at DESC, r.id DESC
		LIMIT $3 OFFSET $4`, p.UserID, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i ListItem
		if err := rows.Scan(append(dest(&i.Request), &i.Conflict, &out.Total)...); err != nil {
			return out, err
		}
		i.CompanyName = i.companyName
		out.Items = append(out.Items, i)
	}
	out.TotalPages = (out.Total + pageSize - 1) / pageSize
	return out, rows.Err()
}

// reviewable returns a submitted request the moderator has no conflict with.
// While the supplier is correcting it, the draft changes stay private.
func reviewable(ctx context.Context, q database.Querier, p auth.Principal, id int64) (*Request, error) {
	req, err := one(q.QueryRowContext(ctx, `SELECT `+columns+from+`WHERE r.id = $1 AND r.status <> 'draft'`, id))
	if err != nil {
		return nil, err
	}
	var conflict bool
	if err := q.QueryRowContext(ctx, `SELECT `+conflictSQL+from+`WHERE r.id = $2`, p.UserID, id).Scan(&conflict); err != nil {
		return nil, err
	}
	if conflict {
		return nil, errConflict
	}
	return req, nil
}

func (r *Repository) review(ctx context.Context, p auth.Principal, id int64) (*Review, error) {
	req, err := reviewable(ctx, r.db, p, id)
	if err != nil {
		return nil, err
	}
	if req.Status == StatusNeedsChanges {
		// Show what was submitted, not the supplier's unfinished edits.
		var raw []byte
		if err := r.db.QueryRowContext(ctx, `
			SELECT details FROM catalog_events WHERE request_id = $1 AND action = 'submitted' ORDER BY id DESC LIMIT 1`, id).Scan(&raw); err != nil {
			return nil, err
		}
		var f Fields
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		req.Fields = f
		if err := r.db.QueryRowContext(ctx, `SELECT name_kk, name_ru FROM categories WHERE id = $1`, f.CategoryID).Scan(&req.CategoryKK, &req.CategoryRU); err != nil {
			return nil, err
		}
	}
	v := &Review{Request: *req, CompanyName: req.companyName, Similar: []SimilarProduct{}}
	if err := r.db.QueryRowContext(ctx, `
		SELECT confirmed_source_kind, confirmed_source,
			(SELECT count(*) FROM catalog_requests o WHERE o.gtin = $2 AND o.status = 'in_review' AND o.id <> $1)
		FROM catalog_requests WHERE id = $1`, id, req.GTIN).Scan(&v.ConfirmedKind, &v.ConfirmedSource, &v.SameGTINRequests); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, catalog_no, name_kk, name_ru, brand, gtin, COALESCE(gtin = $1, FALSE), is_demo FROM products
		WHERE gtin = $1 OR similarity(name_kk, $2) > 0.3 OR similarity(name_ru, $3) > 0.3
		ORDER BY COALESCE(gtin = $1, FALSE) DESC, greatest(similarity(name_kk, $2), similarity(name_ru, $3)) DESC, id LIMIT 10`,
		req.GTIN, req.NameKK, req.NameRU)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s SimilarProduct
		if err := rows.Scan(&s.ID, &s.CatalogNo, &s.NameKK, &s.NameRU, &s.Brand, &s.GTIN, &s.SameGTIN, &s.IsDemo); err != nil {
			return nil, err
		}
		v.Similar = append(v.Similar, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	v.Events, err = r.events(ctx, id, false)
	return v, err
}

// lockForModeration serialises moderators and the owner on the company row, then
// re-checks the permission, the conflict and the exact submission.
func lockForModeration(ctx context.Context, tx *sql.Tx, p auth.Principal, id int64, submissionNo int) (*Request, error) {
	var companyID int64
	err := tx.QueryRowContext(ctx, `SELECT company_id FROM catalog_requests WHERE id = $1 AND status <> 'draft'`, id).Scan(&companyID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM companies WHERE id = $1 FOR UPDATE`, companyID); err != nil {
		return nil, err
	}
	ok, err := auth.HasPermission(ctx, tx, p.UserID, auth.PermCatalogModerate)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNotFound
	}
	req, err := reviewable(ctx, tx, p, id)
	if err != nil {
		return nil, err
	}
	if req.Status != StatusInReview || req.SubmissionNo != submissionNo {
		return nil, errStale
	}
	return req, nil
}

func auditAction(ctx context.Context, tx *sql.Tx, p auth.Principal, req *Request, action string) error {
	return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: req.companyID, Action: action,
		Details: map[string]any{"request_id": req.ID, "submission_no": req.SubmissionNo}})
}

func (r *Repository) confirmGTIN(ctx context.Context, p auth.Principal, id int64, submissionNo int, kind, source string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := lockForModeration(ctx, tx, p, id, submissionNo)
		if err != nil {
			return err
		}
		if err := gtinTaken(ctx, tx, req.GTIN); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE catalog_requests SET confirmed_source_kind = $2, confirmed_source = $3, gtin_confirmed_at = now(), gtin_confirmed_by = $4
			WHERE id = $1`, id, kind, source, p.UserID); err != nil {
			return err
		}
		if err := auditAction(ctx, tx, p, req, "catalog.gtin_confirmed"); err != nil {
			return err
		}
		return writeEvent(ctx, tx, req, p.UserID, "gtin_confirmed", source,
			map[string]any{"gtin": req.GTIN, "source_kind": kind, "source": source})
	})
}

func (r *Repository) decide(ctx context.Context, p auth.Principal, id int64, submissionNo int, status, action, comment string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		req, err := lockForModeration(ctx, tx, p, id, submissionNo)
		if err != nil {
			return err
		}
		if status == StatusApproved {
			if !req.GTINConfirmed {
				return errUnconfirmed
			}
			if err := req.Fields.complete(); err != nil {
				return err
			}
			if err := checkCategory(ctx, tx, req.CategoryID); err != nil {
				return err
			}
			// Publishes the shared card only; no offer is created.
			var productID int64
			err := tx.QueryRowContext(ctx, `
				INSERT INTO products (category_id, slug, name_kk, name_ru, description_kk, description_ru, brand, brand_id, unit,
					gtin, gtin_source_kind, gtin_source, gtin_confirmed_at, gtin_confirmed_by, needs_review)
				SELECT category_id, $2, name_kk, name_ru, description_kk, description_ru, NULLIF(brand, ''),
					(SELECT b.id FROM brands b WHERE lower(b.name) = lower(regexp_replace(btrim(catalog_requests.brand), '\s+', ' ', 'g'))), unit,
					gtin, confirmed_source_kind, confirmed_source, gtin_confirmed_at, gtin_confirmed_by, FALSE
				FROM catalog_requests WHERE id = $1 RETURNING id`, id, "p-"+strings.ToLower(rand.Text())).Scan(&productID)
			if database.UniqueViolation(err) == "products_gtin_key" {
				return errGTINExists
			}
			if err != nil {
				return fmt.Errorf("create product: %w", err)
			}
			req.ProductID = &productID
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE catalog_requests SET status = $2, comment = $3, decided_at = now(), product_id = $4 WHERE id = $1`,
			id, status, comment, req.ProductID); err != nil {
			return err
		}
		if err := auditAction(ctx, tx, p, req, "catalog.request_"+action); err != nil {
			return err
		}
		return writeEvent(ctx, tx, req, p.UserID, action, comment, nil)
	})
}
