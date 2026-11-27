// Package suppliers is the public list of suppliers. It publishes an explicit,
// short set of fields: the company's name, its city and the fact that the
// platform verified it. No contacts, addresses, BIN, members, documents,
// comments, verification history, offers or ERP data: there is no agreed
// rule to publish them yet (docs/catalog.md).
package suppliers

import (
	"net/http"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const pageSize = 24

var errNotFound = httpx.NewError(http.StatusNotFound, "supplier_not_found", "Supplier not found")

// Supplier is the whole public record. Add a field only with a decision that
// it may be published.
type Supplier struct {
	ID   int64   `json:"id"`
	Name string  `json:"name"`
	City *string `json:"city"`
	// Verified is always true for a listed supplier: unverified companies are
	// not public at all. It confirms the company check, not goods or deals.
	Verified bool `json:"verified"`
}

type Page struct {
	Items      []Supplier `json:"items"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	Total      int        `json:"total"`
	TotalPages int        `json:"total_pages"`
	// Cities of public suppliers, for the filter.
	Cities []string `json:"cities"`
}
