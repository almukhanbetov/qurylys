package offers

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ProductVisible reports whether the product exists and is shown in the catalogue.
func (r *Repository) ProductVisible(ctx context.Context, productID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM products p JOIN categories c ON c.id = p.category_id
			WHERE p.id = $1 AND p.is_active AND c.is_active
		)`, productID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check product: %w", err)
	}
	return ok, nil
}

// ListByProduct returns active offers of active suppliers: available ones first,
// then by price ascending. Out-of-stock offers stay visible but flagged.
func (r *Repository) ListByProduct(ctx context.Context, productID int64) ([]Offer, error) {
	q := `
		SELECT o.id, s.id, s.slug, s.name, s.city, s.is_demo,
			o.price_tiyn, o.currency,
			trim_scale(o.stock_quantity)::text, trim_scale(o.min_order_quantity)::text,
			(` + AvailableCondition + `) AS is_available,
			o.delivery_available, o.delivery_days_min, o.delivery_days_max,
			o.delivery_terms_kk, o.delivery_terms_ru, o.city, o.is_demo, o.updated_at
		FROM offers o
		JOIN companies s ON s.id = o.supplier_company_id
		WHERE o.product_id = $1 AND o.is_active AND s.is_supplier AND s.status = 'active'
		ORDER BY is_available DESC, o.price_tiyn ASC, o.id ASC`

	rows, err := r.db.QueryContext(ctx, q, productID)
	if err != nil {
		return nil, fmt.Errorf("list offers: %w", err)
	}
	defer rows.Close()

	out := []Offer{}
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.Supplier.ID, &o.Supplier.Slug, &o.Supplier.Name, &o.Supplier.City, &o.Supplier.IsDemo,
			&o.Price.AmountTiyn, &o.Price.Currency,
			&o.StockQuantity, &o.MinOrderQuantity, &o.IsAvailable,
			&o.DeliveryAvailable, &o.DeliveryDaysMin, &o.DeliveryDaysMax,
			&o.DeliveryTermsKK, &o.DeliveryTermsRU, &o.City, &o.IsDemo, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan offer: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
