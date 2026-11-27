// Package brands is the brand / manufacturer directory (field 05 of the product
// form). Holders of catalog.brands maintain it. Old cards keep their brand text
// until someone links that exact text to a brand: similar spellings are never
// merged automatically. Logos wait for the product files decision (N5).
package brands

import (
	"net/http"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const pageSize = 30

var (
	errNotFound  = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errForbidden = httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	errExists    = httpx.NewError(http.StatusConflict, "brand_exists", "A brand with this name already exists")
	errStale     = httpx.NewError(http.StatusConflict, "brand_conflict", "The brand has changed, reload it")
	errInUse     = httpx.NewError(http.StatusConflict, "brand_in_use", "Product cards refer to the brand")
	errNoCards   = httpx.NewError(http.StatusConflict, "brand_text_not_found", "No unlinked cards have this brand text")
)

type Brand struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Products int    `json:"product_count"`
	Version  int    `json:"version"`
}

type Page struct {
	Items      []Brand `json:"items"`
	Page       int     `json:"page"`
	Total      int     `json:"total"`
	TotalPages int     `json:"total_pages"`
}

// LegacyText is a brand text of real cards that are not linked to a brand yet.
type LegacyText struct {
	Text     string `json:"text"`
	Products int    `json:"product_count"`
	// Match is the brand whose name equals the text ignoring case and spaces.
	// It is only a hint: linking is always an explicit action.
	Match *Brand `json:"match"`
}
