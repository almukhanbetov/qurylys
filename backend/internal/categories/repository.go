package categories

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ListActive returns all active categories in display order with direct product counts.
func (r *Repository) ListActive(ctx context.Context) ([]Category, error) {
	const q = `
		SELECT c.id, c.parent_id, c.slug, c.name_kk, c.name_ru, c.sort_order,
		       COUNT(p.id) AS direct_product_count
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id AND p.is_active
		WHERE c.is_active
		GROUP BY c.id
		ORDER BY c.sort_order, c.id`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		var parent sql.NullInt64
		if err := rows.Scan(&c.ID, &parent, &c.Slug, &c.NameKK, &c.NameRU, &c.SortOrder, &c.DirectProductCount); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		if parent.Valid {
			p := parent.Int64
			c.ParentID = &p
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
