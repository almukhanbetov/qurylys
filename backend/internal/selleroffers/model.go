// Package selleroffers is the supplier's cabinet: drafts, prices, stock and the
// owner's decision to publish. It does not make anything publicly visible.
package selleroffers

import (
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const (
	StateDraft     = "draft"
	StatePublished = "published"
	StateHidden    = "hidden"

	LevelWholesale      = "wholesale"
	LevelLargeWholesale = "large_wholesale"
)

var (
	errNotFound       = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errForbidden      = httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	errNotSupplier    = httpx.NewError(http.StatusForbidden, "not_supplier", "The company is not a supplier")
	errCard           = httpx.NewError(http.StatusConflict, "card_unavailable", "The card is not available for offers")
	errOfferExists    = httpx.NewError(http.StatusConflict, "offer_exists", "The company already has an offer for this card")
	errSKUTaken       = httpx.NewError(http.StatusConflict, "seller_sku_taken", "The company already uses this article")
	errStale          = httpx.NewError(http.StatusConflict, "offer_conflict", "The offer has changed, reload it")
	errNotPublishable = httpx.NewError(http.StatusConflict, "offer_not_publishable", "Publication conditions are not met")
	errIncomplete     = httpx.NewError(http.StatusConflict, "published_offer_incomplete", "A published offer must keep its price, article and VAT status; hide it first")
)

// Level is one price level. Quantities are exact decimal strings in base units.
type Level struct {
	PriceTiyn *int64 `json:"price_tiyn"`
	SaleMode  string `json:"sale_mode"`
	Min       string `json:"min_quantity"`
	Step      string `json:"step_quantity"`
}

// Input is what the owner fills in. The base unit comes from the card.
type Input struct {
	SellerSKU      string `json:"seller_sku"`
	VAT            string `json:"vat_status"`
	PackSize       string `json:"pack_size"`
	Stock          string `json:"stock_quantity"`
	City           string `json:"city"`
	Retail         Level  `json:"retail"`
	Wholesale      *Level `json:"wholesale"`
	LargeWholesale *Level `json:"large_wholesale"`
}

type Card struct {
	ID        int64  `json:"id"`
	CatalogNo string `json:"catalog_no"`
	NameKK    string `json:"name_kk"`
	NameRU    string `json:"name_ru"`
	Unit      string `json:"unit"`
	GTIN      string `json:"gtin"`
	HasOffer  bool   `json:"has_offer"`
}

// LevelView adds calculated amounts (half up, tiyn) and stock availability.
type LevelView struct {
	Level
	PackPriceTiyn *int64 `json:"pack_price_tiyn"`
	MinTotalTiyn  *int64 `json:"min_total_tiyn"`
	// InStock: the stock covers the minimum. One level never hides another.
	InStock bool `json:"in_stock"`
}

type Offer struct {
	ID             int64      `json:"id"`
	Card           Card       `json:"card"`
	OwnerState     string     `json:"owner_state"`
	SellerSKU      string     `json:"seller_sku"`
	VAT            string     `json:"vat_status"`
	PackSize       string     `json:"pack_size"`
	Stock          string     `json:"stock_quantity"`
	City           string     `json:"city"`
	Retail         LevelView  `json:"retail"`
	Wholesale      *LevelView `json:"wholesale"`
	LargeWholesale *LevelView `json:"large_wholesale"`
	Version        int        `json:"version"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// Reasons why the conditions of publication are not met; empty = met.
	Reasons []string `json:"reasons"`
}

type View struct {
	Offer
	CanManage bool `json:"can_manage"`
}

type ListView struct {
	Items     []Offer `json:"items"`
	CanManage bool    `json:"can_manage"`
	// CanCreate: the owner of an active supplier company.
	CanCreate bool `json:"can_create"`
}
