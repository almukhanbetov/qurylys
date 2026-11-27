package brands

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/database"
	"github.com/qurylys/qurylys/backend/internal/products"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func like(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

// NormalizeSQL is the SQL form of Normalize (trim, single spaces) for a text $n.
func normalizeSQL(ph string) string { return `regexp_replace(btrim(` + ph + `), '\s+', ' ', 'g')` }

// manager re-checks the permission inside the transaction.
func manager(ctx context.Context, tx *sql.Tx, p auth.Principal) error {
	ok, err := auth.HasPermission(ctx, tx, p.UserID, auth.PermCatalogBrands)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden
	}
	return nil
}

func (r *Repository) list(ctx context.Context, q string, page int) (Page, error) {
	out := Page{Items: []Brand{}, Page: page}
	where := ` FROM brands b WHERE ($1 = '' OR b.name ILIKE $2)`
	if err := r.db.QueryRowContext(ctx, `SELECT count(*)`+where, q, like(q)).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = (out.Total + pageSize - 1) / pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.id, b.name, (SELECT count(*) FROM products p WHERE p.brand_id = b.id), b.version`+where+`
		ORDER BY lower(b.name), b.id LIMIT `+strconv.Itoa(pageSize)+` OFFSET $3`, q, like(q), (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.Products, &b.Version); err != nil {
			return out, err
		}
		out.Items = append(out.Items, b)
	}
	return out, rows.Err()
}

func (r *Repository) create(ctx context.Context, p auth.Principal, name string) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `INSERT INTO brands (name) VALUES ($1) RETURNING id`, name).Scan(&id)
		if database.UniqueViolation(err) == "brands_name_key" {
			return errExists
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.brand_created",
			Details: map[string]any{"brand_id": id, "name": name}})
	})
	return id, err
}

func lockBrand(ctx context.Context, tx *sql.Tx, id int64, version int) (name string, err error) {
	var v int
	err = tx.QueryRowContext(ctx, `SELECT name, version FROM brands WHERE id = $1 FOR UPDATE`, id).Scan(&name, &v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNotFound
	}
	if err == nil && v != version {
		err = errStale
	}
	return name, err
}

// rename corrects the spelling; linked cards show the new name (N3: cosmetic).
func (r *Repository) rename(ctx context.Context, p auth.Principal, id int64, version int, name string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		old, err := lockBrand(ctx, tx, id, version)
		if err != nil || old == name {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE brands SET name = $2 WHERE id = $1`, id, name)
		if database.UniqueViolation(err) == "brands_name_key" {
			return errExists
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.brand_renamed",
			Details: map[string]any{"brand_id": id, "old": old, "new": name}})
	})
}

func (r *Repository) remove(ctx context.Context, p auth.Principal, id int64, version int) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		name, err := lockBrand(ctx, tx, id, version)
		if err != nil {
			return err
		}
		var used bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE brand_id = $1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return errInUse
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM brands WHERE id = $1`, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.brand_deleted",
			Details: map[string]any{"brand_id": id, "name": name}})
	})
}

// legacy lists brand texts of real cards without a brand. Demo cards are
// maintained by the seed and never linked.
func (r *Repository) legacy(ctx context.Context, q string) ([]LegacyText, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.brand, t.n, b.id, b.name, (SELECT count(*) FROM products p WHERE p.brand_id = b.id), b.version
		FROM (
			SELECT brand, count(*) AS n FROM products
			WHERE brand_id IS NULL AND NOT is_demo AND btrim(COALESCE(brand, '')) <> '' AND ($1 = '' OR brand ILIKE $2)
			GROUP BY brand
		) t
		LEFT JOIN brands b ON lower(b.name) = lower(`+normalizeSQL("t.brand")+`)
		ORDER BY lower(t.brand), t.brand LIMIT 200`, q, like(q))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LegacyText{}
	for rows.Next() {
		var t LegacyText
		var id, n, v sql.NullInt64
		var name sql.NullString
		if err := rows.Scan(&t.Text, &t.Products, &id, &name, &n, &v); err != nil {
			return nil, err
		}
		if id.Valid {
			t.Match = &Brand{ID: id.Int64, Name: name.String, Products: int(n.Int64), Version: int(v.Int64)}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// link attaches every unlinked real card whose brand text is exactly text.
// The old text is kept in the catalogue journal of each card.
func (r *Repository) link(ctx context.Context, p auth.Principal, id int64, text string) (n int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		var name string
		err := tx.QueryRowContext(ctx, `SELECT name FROM brands WHERE id = $1 FOR SHARE`, id).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			WITH linked AS (
				UPDATE products SET brand_id = $1
				WHERE brand_id IS NULL AND NOT is_demo AND brand = $2
				RETURNING id
			)
			INSERT INTO catalog_events (product_id, actor_user_id, actor_name, action, comment, details)
			SELECT l.id, u.id, u.full_name, 'card_edited', '',
				jsonb_build_object('kind', 'brand_link', 'old', jsonb_build_object('brand', $2::text),
					'new', jsonb_build_object('brand', $4::text, 'brand_id', $1::bigint))
			FROM linked l, users u WHERE u.id = $3`, id, text, p.UserID, name)
		if err != nil {
			return err
		}
		if n, _ = res.RowsAffected(); n == 0 {
			return errNoCards
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.brand_linked",
			Details: map[string]any{"brand_id": id, "text": text, "cards": n}})
	})
	return n, err
}

// publicFrom keeps only brands with at least one publicly shown card and
// counts only such cards (products.PublicCardSQL).
const publicFrom = ` FROM brands b JOIN products p ON p.brand_id = b.id JOIN categories cat ON cat.id = p.category_id
	WHERE ` + products.PublicCardSQL + ` `

const publicPageSize = 24

func (r *Repository) publicList(ctx context.Context, q string, page int) (PublicPage, error) {
	out := PublicPage{Items: []PublicBrand{}, Page: page, PageSize: publicPageSize}
	filter := `AND ($1 = '' OR b.name ILIKE $2)`
	if err := r.db.QueryRowContext(ctx, `SELECT count(DISTINCT b.id)`+publicFrom+filter, q, like(q)).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = (out.Total + publicPageSize - 1) / publicPageSize
	rows, err := r.db.QueryContext(ctx, `SELECT b.id, b.name, count(*)`+publicFrom+filter+`
		GROUP BY b.id ORDER BY lower(b.name), b.id LIMIT `+strconv.Itoa(publicPageSize)+` OFFSET $3`, q, like(q), (page-1)*publicPageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var b PublicBrand
		if err := rows.Scan(&b.ID, &b.Name, &b.Products); err != nil {
			return out, err
		}
		out.Items = append(out.Items, b)
	}
	return out, rows.Err()
}

func (r *Repository) publicGet(ctx context.Context, id int64) (*PublicBrand, error) {
	var b PublicBrand
	err := r.db.QueryRowContext(ctx, `SELECT b.id, b.name, count(*)`+publicFrom+`AND b.id = $1 GROUP BY b.id`, id).Scan(&b.ID, &b.Name, &b.Products)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errPublicNotFound
	}
	return &b, err
}
