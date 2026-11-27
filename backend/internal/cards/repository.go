package cards

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/companies"
	"github.com/qurylys/qurylys/backend/internal/database"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// conflictSQL: user $1 is or was in a company with a real offer on card $2, or
// in the company (or is the author) of the request that created it.
const conflictSQL = `(
	EXISTS (SELECT 1 FROM offers o JOIN companies c ON c.id = o.supplier_company_id
		WHERE o.product_id = $2 AND NOT o.is_demo AND ` + companies.InvolvedSQL + `)
	OR EXISTS (SELECT 1 FROM catalog_requests r JOIN companies c ON c.id = r.company_id
		WHERE r.product_id = $2 AND (r.author_user_id = $1 OR ` + companies.InvolvedSQL + `)))`

const itemColumns = `p.id, p.catalog_no, p.name_kk, p.name_ru, p.brand, p.brand_id, p.category_id, cat.name_kk, cat.name_ru,
	p.unit, p.gtin, p.needs_review, p.is_demo, p.blocked_at IS NOT NULL,
	(SELECT count(*) FROM offers o WHERE o.product_id = p.id AND NOT o.is_demo)`

const itemFrom = ` FROM products p JOIN categories cat ON cat.id = p.category_id `

func itemDest(i *Item) []any {
	return []any{&i.ID, &i.CatalogNo, &i.NameKK, &i.NameRU, &i.Brand, &i.BrandID, &i.CategoryID, &i.CategoryKK, &i.CategoryRU,
		&i.Unit, &i.GTIN, &i.NeedsReview, &i.IsDemo, &i.Blocked, &i.Offers}
}

// Fixed conditions; the chosen key never reaches the SQL text.
var stateSQL = map[string]string{
	"needs_review": "p.needs_review AND NOT p.is_demo",
	"reviewed":     "NOT p.needs_review",
	"blocked":      "p.blocked_at IS NOT NULL",
	"no_gtin":      "p.gtin IS NULL",
	"real":         "NOT p.is_demo",
	"demo":         "p.is_demo",
}

func like(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

func (r *Repository) list(ctx context.Context, f Filter) (Page, error) {
	conds, args := []string{"TRUE"}, []any{}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	cte := ""
	if f.Q != "" {
		ph := arg(like(f.Q))
		c := fmt.Sprintf("p.name_kk ILIKE %[1]s OR p.name_ru ILIKE %[1]s OR p.catalog_no ILIKE %[1]s", ph)
		if d := strings.NewReplacer(" ", "", "-", "").Replace(f.Q); isDigits(d) && len(d) <= 14 {
			c += " OR p.gtin = lpad(" + arg(d) + ", 14, '0')"
		}
		conds = append(conds, "("+c+")")
	}
	if f.CategoryID > 0 {
		cte = `WITH RECURSIVE subtree AS (
			SELECT id FROM categories WHERE id = ` + arg(f.CategoryID) + `
			UNION ALL SELECT c.id FROM categories c JOIN subtree t ON c.parent_id = t.id) `
		conds = append(conds, "p.category_id IN (SELECT id FROM subtree)")
	}
	switch f.Brand {
	case "":
	case "none":
		conds = append(conds, "p.brand_id IS NULL")
	default:
		id, _ := strconv.ParseInt(f.Brand, 10, 64) // validated by the handler
		conds = append(conds, "p.brand_id = "+arg(id))
	}
	if c := stateSQL[f.State]; c != "" {
		conds = append(conds, c)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	out := Page{Items: []Item{}, Page: f.Page}
	if err := r.db.QueryRowContext(ctx, cte+"SELECT count(*)"+itemFrom+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = (out.Total + pageSize - 1) / pageSize
	rows, err := r.db.QueryContext(ctx, cte+"SELECT "+itemColumns+itemFrom+where+
		" ORDER BY p.is_demo, p.needs_review DESC, p.id DESC LIMIT "+strconv.Itoa(pageSize)+" OFFSET "+arg((f.Page-1)*pageSize), args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i Item
		if err := rows.Scan(itemDest(&i)...); err != nil {
			return out, err
		}
		out.Items = append(out.Items, i)
	}
	return out, rows.Err()
}

const cardColumns = itemColumns + `, p.description_kk, p.description_ru, p.attributes, p.gtin_source_kind, p.gtin_source,
	p.gtin_confirmed_at, p.blocked_at, p.block_reason, p.version, p.updated_at,
	(SELECT min(id) FROM catalog_requests r WHERE r.product_id = p.id)`

func scanCard(row *sql.Row) (*Card, error) {
	var c Card
	var attrs []byte
	err := row.Scan(append(itemDest(&c.Item), &c.DescriptionKK, &c.DescriptionRU, &attrs, &c.GTINSourceKind, &c.GTINSource,
		&c.GTINConfirmedAt, &c.BlockedAt, &c.BlockReason, &c.Version, &c.UpdatedAt, &c.RequestID)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Attributes = attrs
	return &c, nil
}

func (r *Repository) get(ctx context.Context, p auth.Principal, id int64) (*Card, error) {
	c, err := scanCard(r.db.QueryRowContext(ctx, `SELECT `+cardColumns+itemFrom+`WHERE p.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE NOT is_demo),
			count(*) FILTER (WHERE NOT is_demo AND owner_state = 'draft'),
			count(*) FILTER (WHERE NOT is_demo AND owner_state = 'published'),
			count(*) FILTER (WHERE NOT is_demo AND owner_state = 'hidden'),
			count(*) FILTER (WHERE is_demo)
		FROM offers WHERE product_id = $1`, id).Scan(
		&c.OfferStats.Total, &c.OfferStats.Draft, &c.OfferStats.Published, &c.OfferStats.Hidden, &c.OfferStats.Demo); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT `+conflictSQL, p.UserID, id).Scan(&c.Conflict); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, action, actor_name, comment, details, created_at FROM catalog_events
		WHERE product_id = $1 ORDER BY id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c.Events = []Event{}
	for rows.Next() {
		var e Event
		var d []byte
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Comment, &d, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Details = d
		c.Events = append(c.Events, e)
	}
	return c, rows.Err()
}

// editor re-checks the permission inside the transaction (a revocation takes
// effect at once).
func editor(ctx context.Context, tx *sql.Tx, p auth.Principal) error {
	ok, err := auth.HasPermission(ctx, tx, p.UserID, auth.PermCatalogModerate)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden
	}
	return nil
}

// locked is a card under FOR UPDATE: it waits for offers being created on it
// (they hold FOR SHARE), so the count of linked offers is final.
type locked struct {
	Card
	linked bool
}

func lockCard(ctx context.Context, tx *sql.Tx, p auth.Principal, id int64, version int) (*locked, error) {
	if err := editor(ctx, tx, p); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM products WHERE id = $1 FOR UPDATE`, id); err != nil {
		return nil, err
	}
	c, err := scanCard(tx.QueryRowContext(ctx, `SELECT `+cardColumns+itemFrom+`WHERE p.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if c.IsDemo {
		return nil, errDemo
	}
	var conflict bool
	if err := tx.QueryRowContext(ctx, `SELECT `+conflictSQL, p.UserID, id).Scan(&conflict); err != nil {
		return nil, err
	}
	if conflict {
		return nil, errConflict
	}
	if c.Version != version {
		return nil, errStale
	}
	return &locked{Card: *c, linked: c.Offers > 0}, nil
}

func writeEvent(ctx context.Context, tx *sql.Tx, p auth.Principal, productID int64, action, comment string, details map[string]any) error {
	d, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO catalog_events (product_id, actor_user_id, actor_name, action, comment, details)
		SELECT $1, u.id, u.full_name, $3, $4, $5::jsonb FROM users u WHERE u.id = $2`,
		productID, p.UserID, action, comment, string(d)); err != nil {
		return fmt.Errorf("catalog event: %w", err)
	}
	return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog." + action,
		Details: map[string]any{"product_id": productID}})
}

func checkRefs(ctx context.Context, tx *sql.Tx, f Fields) error {
	var category, brand bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1 AND is_active),
			$2::bigint IS NULL OR EXISTS (SELECT 1 FROM brands WHERE id = $2)`, f.CategoryID, f.BrandID).Scan(&category, &brand); err != nil {
		return err
	}
	if !category {
		return errCategory
	}
	if !brand {
		return errBrand
	}
	return nil
}

func (r *Repository) create(ctx context.Context, p auth.Principal, f Fields, g GTIN) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := editor(ctx, tx, p); err != nil {
			return err
		}
		if err := checkRefs(ctx, tx, f); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO products (category_id, slug, name_kk, name_ru, description_kk, description_ru, brand_id, unit,
				gtin, gtin_source_kind, gtin_source, gtin_confirmed_at, gtin_confirmed_by, needs_review)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), $12, FALSE) RETURNING id`,
			f.CategoryID, "p-"+strings.ToLower(rand.Text()), f.NameKK, f.NameRU, f.DescriptionKK, f.DescriptionRU, f.BrandID, f.Unit,
			g.GTIN, g.SourceKind, g.Source, p.UserID).Scan(&id)
		if database.UniqueViolation(err) == "products_gtin_key" {
			return errGTINExists
		}
		if err != nil {
			return err
		}
		return writeEvent(ctx, tx, p, id, "card_created", "", map[string]any{"new": f, "gtin": g})
	})
	return id, err
}

// changes lists the edited fields as old and new values.
func changes(c *locked, f Fields) (old, new map[string]any) {
	old, new = map[string]any{}, map[string]any{}
	add := func(k string, a, b any) {
		if a != b {
			old[k], new[k] = a, b
		}
	}
	add("category_id", c.CategoryID, f.CategoryID)
	add("name_kk", c.NameKK, f.NameKK)
	add("name_ru", c.NameRU, f.NameRU)
	add("description_kk", c.DescriptionKK, f.DescriptionKK)
	add("description_ru", c.DescriptionRU, f.DescriptionRU)
	add("unit", c.Unit, f.Unit)
	var a, b int64
	if c.BrandID != nil {
		a = *c.BrandID
	}
	if f.BrandID != nil {
		b = *f.BrandID
	}
	add("brand_id", a, b)
	return old, new
}

// update applies N3: with supplier offers only cosmetic changes are allowed.
func (r *Repository) update(ctx context.Context, p auth.Principal, id int64, version int, f Fields, reason string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		c, err := lockCard(ctx, tx, p, id, version)
		if err != nil {
			return err
		}
		old, new := changes(c, f)
		if len(new) == 0 {
			return errNoChange
		}
		kind := "correction"
		if c.linked {
			kind = "cosmetic"
			_, category := new["category_id"]
			_, unit := new["unit"]
			_, brand := new["brand_id"]
			if category || unit || (brand && c.BrandID != nil) {
				return errLocked
			}
			if !sameNumbers(c.NameKK, f.NameKK) || !sameNumbers(c.NameRU, f.NameRU) {
				return errIdentity
			}
		}
		if err := checkRefs(ctx, tx, f); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE products SET category_id = $2, name_kk = $3, name_ru = $4, description_kk = $5, description_ru = $6,
				brand_id = $7, brand = CASE WHEN $7::bigint IS NULL AND brand_id IS NOT NULL THEN NULL ELSE brand END, unit = $8
			WHERE id = $1`, id, f.CategoryID, f.NameKK, f.NameRU, f.DescriptionKK, f.DescriptionRU, f.BrandID, f.Unit); err != nil {
			return err
		}
		if _, ok := old["brand_id"]; ok && c.Brand != nil {
			old["brand"] = *c.Brand // the text shown before, also for unlinked cards
		}
		return writeEvent(ctx, tx, p, id, "card_edited", reason, map[string]any{"kind": kind, "old": old, "new": new})
	})
}

// confirmGTIN records a checked GTIN with its source and marks the card
// reviewed. A card with offers keeps its GTIN (R11); old cards get one only
// from a real source (C4).
func (r *Repository) confirmGTIN(ctx context.Context, p auth.Principal, id int64, version int, g GTIN, reason string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		c, err := lockCard(ctx, tx, p, id, version)
		if err != nil {
			return err
		}
		if c.linked && c.GTIN != nil && *c.GTIN != g.GTIN {
			return errLocked
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE products SET gtin = $2, gtin_source_kind = $3, gtin_source = $4, gtin_confirmed_at = now(),
				gtin_confirmed_by = $5, needs_review = FALSE WHERE id = $1`, id, g.GTIN, g.SourceKind, g.Source, p.UserID)
		if database.UniqueViolation(err) == "products_gtin_key" {
			return errGTINExists
		}
		if err != nil {
			return err
		}
		return writeEvent(ctx, tx, p, id, "card_gtin_confirmed", reason, map[string]any{
			"old": map[string]any{"gtin": c.GTIN, "source_kind": c.GTINSourceKind, "source": c.GTINSource, "needs_review": c.NeedsReview},
			"new": g})
	})
}

// setBlocked: R6. Offers are not touched; the card's offers cannot be published
// while it is blocked, and public pages do not show it.
func (r *Repository) setBlocked(ctx context.Context, p auth.Principal, id int64, version int, block bool, reason string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		c, err := lockCard(ctx, tx, p, id, version)
		if err != nil {
			return err
		}
		if block == c.Blocked {
			if block {
				return errBlocked
			}
			return errNotBlocked
		}
		if block {
			_, err = tx.ExecContext(ctx, `UPDATE products SET blocked_at = now(), block_reason = $2, blocked_by = $3 WHERE id = $1`, id, reason, p.UserID)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE products SET blocked_at = NULL, block_reason = NULL, blocked_by = NULL WHERE id = $1`, id)
		}
		if err != nil {
			return err
		}
		action := "card_unblocked"
		if block {
			action = "card_blocked"
		}
		return writeEvent(ctx, tx, p, id, action, reason, map[string]any{"offers": c.Offers})
	})
}
