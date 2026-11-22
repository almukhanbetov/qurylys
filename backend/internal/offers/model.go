package offers

import "time"

// AvailableCondition is the single definition of an "available" offer.
// It expects the offer aliased as o and the supplier company as s.
// It drives products' min_price/offer_count and each offer's is_available flag.
// Documented in docs/catalog.md — change both together.
const AvailableCondition = `o.is_active
	AND s.is_supplier AND s.status = 'active'
	AND o.stock_quantity >= o.min_order_quantity`

type Money struct {
	AmountTiyn int64  `json:"amount_tiyn"`
	Currency   string `json:"currency"`
}

type Supplier struct {
	ID     int64   `json:"id"`
	Slug   string  `json:"slug"`
	Name   string  `json:"name"`
	City   *string `json:"city"`
	IsDemo bool    `json:"is_demo"`
}

type Offer struct {
	ID       int64    `json:"id"`
	Supplier Supplier `json:"supplier"`
	// Price is per one unit of the product (products.unit).
	Price Money `json:"price"`
	// Quantities are decimal strings (NUMERIC in PostgreSQL) to stay exact.
	StockQuantity     string    `json:"stock_quantity"`
	MinOrderQuantity  string    `json:"min_order_quantity"`
	IsAvailable       bool      `json:"is_available"`
	DeliveryAvailable bool      `json:"delivery_available"`
	DeliveryDaysMin   *int      `json:"delivery_days_min"`
	DeliveryDaysMax   *int      `json:"delivery_days_max"`
	DeliveryTermsKK   string    `json:"delivery_terms_kk"`
	DeliveryTermsRU   string    `json:"delivery_terms_ru"`
	City              *string   `json:"city"`
	IsDemo            bool      `json:"is_demo"`
	UpdatedAt         time.Time `json:"updated_at"`
}
