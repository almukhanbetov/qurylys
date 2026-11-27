package products

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/offers"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// sortClauses is the only source of ORDER BY text. User input selects a key;
// it is never concatenated into SQL. Every clause ends with p.id so paging is stable.
var sortClauses = map[Sort]string{
	SortNewest:    "p.created_at DESC, p.id DESC",
	SortPriceAsc:  "os.min_price_tiyn ASC NULLS LAST, p.id ASC",
	SortPriceDesc: "os.min_price_tiyn DESC NULLS LAST, p.id DESC",
	SortOffers:    "COALESCE(os.offer_count, 0) DESC, p.id DESC",
}

// offerStatsJoin computes min price and count over available offers per product.
// The availability rule lives in offers.AvailableCondition (docs/catalog.md).
// PublicCardSQL is the single rule for a card shown on public pages (alias p
// for products, cat for its category). Brand pages use it too.
const PublicCardSQL = `p.is_active AND p.blocked_at IS NULL AND cat.is_active`

var offerStatsJoin = `
	LEFT JOIN LATERAL (
		SELECT MIN(o.price_tiyn) AS min_price_tiyn, COUNT(*) AS offer_count
		FROM offers o
		JOIN companies s ON s.id = o.supplier_company_id
		WHERE o.product_id = p.id AND ` + offers.AvailableCondition + `
	) os ON TRUE`

// queryBuilder collects positional arguments so each value gets its own $n.
type queryBuilder struct {
	args []any
}

func (b *queryBuilder) arg(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// escapeLike neutralises LIKE wildcards in user input (backslash is the
// default ESCAPE character in PostgreSQL).
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// buildWhere returns the CTE prefix (for category subtree), the WHERE clause and args.
func buildWhere(f ListFilter) (cte string, where string, b *queryBuilder) {
	b = &queryBuilder{}
	conds := []string{PublicCardSQL}

	if f.CategorySlug != "" {
		cte = `WITH RECURSIVE subtree AS (
			SELECT id FROM categories WHERE slug = ` + b.arg(f.CategorySlug) + ` AND is_active
			UNION ALL
			SELECT c.id FROM categories c JOIN subtree t ON c.parent_id = t.id WHERE c.is_active
		) `
		conds = append(conds, "p.category_id IN (SELECT id FROM subtree)")
	}

	if f.BrandID > 0 {
		conds = append(conds, "p.brand_id = "+b.arg(f.BrandID))
	}

	for _, term := range f.Terms {
		ph := b.arg("%" + escapeLike(term) + "%")
		conds = append(conds, fmt.Sprintf(
			"(p.name_kk ILIKE %[1]s OR p.name_ru ILIKE %[1]s OR p.brand ILIKE %[1]s)", ph))
	}

	return cte, "WHERE " + strings.Join(conds, " AND "), b
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]ListItem, int, error) {
	order, ok := sortClauses[f.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("unsupported sort %q", f.Sort)
	}

	cte, where, b := buildWhere(f)

	countSQL := cte + `SELECT COUNT(*) FROM products p
		JOIN categories cat ON cat.id = p.category_id ` + where
	var total int
	if err := r.db.QueryRowContext(ctx, countSQL, b.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}
	if total == 0 {
		return []ListItem{}, 0, nil
	}

	limit := b.arg(f.PageSize)
	offset := b.arg((f.Page - 1) * f.PageSize)
	listSQL := cte + `SELECT p.id, p.slug, p.catalog_no, p.gtin, p.name_kk, p.name_ru, p.brand, p.brand_id, p.unit, p.is_demo,
			cat.id, cat.slug, cat.name_kk, cat.name_ru,
			os.min_price_tiyn, COALESCE(os.offer_count, 0)
		FROM products p
		JOIN categories cat ON cat.id = p.category_id` + offerStatsJoin + `
		` + where + `
		ORDER BY ` + order + `
		LIMIT ` + limit + ` OFFSET ` + offset

	rows, err := r.db.QueryContext(ctx, listSQL, b.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		var it ListItem
		var minPrice sql.NullInt64
		if err := rows.Scan(&it.ID, &it.Slug, &it.CatalogNo, &it.GTIN, &it.NameKK, &it.NameRU, &it.Brand, &it.BrandID, &it.Unit, &it.IsDemo,
			&it.Category.ID, &it.Category.Slug, &it.Category.NameKK, &it.Category.NameRU,
			&minPrice, &it.OfferCount); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		if minPrice.Valid {
			it.MinPrice = &Money{AmountTiyn: minPrice.Int64, Currency: "KZT"}
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// CategoryExists reports whether an active category with the slug exists.
func (r *Repository) CategoryExists(ctx context.Context, slug string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM categories WHERE slug = $1 AND is_active)`, slug).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check category: %w", err)
	}
	return ok, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (*Detail, error) {
	const q = `
		SELECT p.id, p.slug, p.catalog_no, p.gtin, p.name_kk, p.name_ru, p.brand, p.brand_id, p.unit, p.is_demo,
			cat.id, cat.slug, cat.name_kk, cat.name_ru,
			os.min_price_tiyn, COALESCE(os.offer_count, 0),
			p.description_kk, p.description_ru, p.attributes, p.updated_at
		FROM products p
		JOIN categories cat ON cat.id = p.category_id`

	var d Detail
	var minPrice sql.NullInt64
	var attrs []byte
	err := r.db.QueryRowContext(ctx, q+offerStatsJoin+`
		WHERE p.id = $1 AND `+PublicCardSQL, id).Scan(
		&d.ID, &d.Slug, &d.CatalogNo, &d.GTIN, &d.NameKK, &d.NameRU, &d.Brand, &d.BrandID, &d.Unit, &d.IsDemo,
		&d.Category.ID, &d.Category.Slug, &d.Category.NameKK, &d.Category.NameRU,
		&minPrice, &d.OfferCount,
		&d.DescriptionKK, &d.DescriptionRU, &attrs, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if minPrice.Valid {
		d.MinPrice = &Money{AmountTiyn: minPrice.Int64, Currency: "KZT"}
	}
	if d.Attributes, err = decodeAttributes(attrs); err != nil {
		return nil, fmt.Errorf("decode attributes of product %d: %w", id, err)
	}
	if d.CategoryPath, err = r.categoryPath(ctx, d.Category.ID); err != nil {
		return nil, err
	}
	return &d, nil
}

// categoryPath returns ancestors from root to the given category inclusive.
func (r *Repository) categoryPath(ctx context.Context, categoryID int64) ([]CategoryRef, error) {
	const q = `
		WITH RECURSIVE path AS (
			SELECT id, parent_id, slug, name_kk, name_ru, 0 AS lvl FROM categories WHERE id = $1
			UNION ALL
			SELECT c.id, c.parent_id, c.slug, c.name_kk, c.name_ru, p.lvl + 1
			FROM categories c JOIN path p ON c.id = p.parent_id
		)
		SELECT id, slug, name_kk, name_ru FROM path ORDER BY lvl DESC`

	rows, err := r.db.QueryContext(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("category path: %w", err)
	}
	defer rows.Close()

	path := []CategoryRef{}
	for rows.Next() {
		var c CategoryRef
		if err := rows.Scan(&c.ID, &c.Slug, &c.NameKK, &c.NameRU); err != nil {
			return nil, fmt.Errorf("scan category path: %w", err)
		}
		path = append(path, c)
	}
	return path, rows.Err()
}
