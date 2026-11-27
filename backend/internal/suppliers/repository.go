package suppliers

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// PublicSupplierSQL is the single rule for a public supplier (alias c): an
// active real company with the supplier direction and a current verification.
// Losing the verification or the direction removes it at once.
const PublicSupplierSQL = `c.is_supplier AND c.status = 'active' AND c.verified_request_id IS NOT NULL AND NOT c.is_demo`

func like(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

func (r *Repository) list(ctx context.Context, q, city string, page int) (Page, error) {
	out := Page{Items: []Supplier{}, Cities: []string{}, Page: page, PageSize: pageSize}
	where := ` FROM companies c WHERE ` + PublicSupplierSQL + ` AND ($1 = '' OR c.name ILIKE $2) AND ($3 = '' OR c.city = $3)`
	if err := r.db.QueryRowContext(ctx, `SELECT count(*)`+where, q, like(q), city).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = (out.Total + pageSize - 1) / pageSize
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.name, NULLIF(btrim(c.city), '')`+where+`
		ORDER BY lower(c.name), c.id LIMIT `+strconv.Itoa(pageSize)+` OFFSET $4`, q, like(q), city, (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		s := Supplier{Verified: true}
		if err := rows.Scan(&s.ID, &s.Name, &s.City); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT DISTINCT c.city FROM companies c
		WHERE `+PublicSupplierSQL+` AND btrim(COALESCE(c.city, '')) <> '' ORDER BY c.city`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return out, err
		}
		out.Cities = append(out.Cities, c)
	}
	return out, rows.Err()
}

func (r *Repository) get(ctx context.Context, id int64) (*Supplier, error) {
	s := Supplier{Verified: true}
	err := r.db.QueryRowContext(ctx, `SELECT c.id, c.name, NULLIF(btrim(c.city), '') FROM companies c
		WHERE c.id = $1 AND `+PublicSupplierSQL, id).Scan(&s.ID, &s.Name, &s.City)
	if errors.Is(err, sql.ErrNoRows) {
		// The same answer for a missing, hidden, unverified or demo company.
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
