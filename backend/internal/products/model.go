package products

import (
	"encoding/json"
	"time"
)

// Units allowed by the products_unit_valid constraint (migration 00004).
var Units = []string{"piece", "kg", "tonne", "bag", "m", "m2", "m3", "litre", "pack", "roll", "sheet", "pallet"}

// Money is an exact amount in tiyn (1 KZT = 100 tiyn). See docs/money.md.
type Money struct {
	AmountTiyn int64  `json:"amount_tiyn"`
	Currency   string `json:"currency"`
}

type CategoryRef struct {
	ID     int64  `json:"id"`
	Slug   string `json:"slug"`
	NameKK string `json:"name_kk"`
	NameRU string `json:"name_ru"`
}

// ListItem is a product card in the catalogue list.
type ListItem struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	// CatalogNo is the platform article of the card; not the seller's SKU.
	CatalogNo string `json:"catalog_no"`
	// GTIN is shown only when confirmed.
	GTIN   *string `json:"gtin"`
	NameKK string  `json:"name_kk"`
	NameRU string  `json:"name_ru"`
	Brand  *string `json:"brand"`
	// BrandID is set only for cards explicitly linked to the brand directory.
	BrandID  *int64      `json:"brand_id"`
	Unit     string      `json:"unit"`
	IsDemo   bool        `json:"is_demo"`
	Category CategoryRef `json:"category"`
	// MinPrice is the lowest price among available offers; null when there are none.
	// The exact rule is documented in docs/catalog.md.
	MinPrice *Money `json:"min_price"`
	// OfferCount counts available offers (same rule as MinPrice).
	OfferCount int `json:"offer_count"`
}

type Attribute struct {
	Key    string `json:"key"`
	NameKK string `json:"name_kk"`
	NameRU string `json:"name_ru"`
	Value  string `json:"value"`
}

// Detail is the full product card for GET /products/:id.
type Detail struct {
	ListItem
	DescriptionKK string      `json:"description_kk"`
	DescriptionRU string      `json:"description_ru"`
	Attributes    []Attribute `json:"attributes"`
	// CategoryPath goes from the root category down to the product's category.
	CategoryPath []CategoryRef `json:"category_path"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Sort is a whitelisted sort key; see sortClauses in repository.go.
type Sort string

const (
	SortNewest    Sort = "newest"
	SortPriceAsc  Sort = "price_asc"
	SortPriceDesc Sort = "price_desc"
	SortOffers    Sort = "offers"
)

const (
	DefaultPageSize = 12
	MaxPageSize     = 48
	MaxQueryLength  = 100
	MaxSearchTerms  = 5
)

// ListFilter is the validated input of the list query.
type ListFilter struct {
	// Terms are search words; a product matches when every term is found
	// in its Kazakh name, Russian name or brand.
	Terms        []string
	CategorySlug string
	// BrandID: only cards linked to this brand (0 = any).
	BrandID  int64
	Sort     Sort
	Page     int
	PageSize int
}

type Page struct {
	Items      []ListItem `json:"items"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	Total      int        `json:"total"`
	TotalPages int        `json:"total_pages"`
}

func decodeAttributes(raw []byte) ([]Attribute, error) {
	attrs := []Attribute{}
	if len(raw) == 0 {
		return attrs, nil
	}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}
