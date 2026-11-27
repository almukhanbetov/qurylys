package categories

import (
	"net/http"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

var (
	errNotFound    = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errParent      = httpx.Invalid("parent", "unknown category")
	errSlugTaken   = httpx.NewError(http.StatusConflict, "category_slug_taken", "This address is already used by another category")
	errCycle       = httpx.NewError(http.StatusConflict, "category_cycle", "A category cannot be placed under itself or its descendant")
	errStale       = httpx.NewError(http.StatusConflict, "category_conflict", "The category has changed, reload it")
	errHasChildren = httpx.NewError(http.StatusConflict, "category_has_children", "The category has subcategories")
	errHasProducts = httpx.NewError(http.StatusConflict, "category_has_products", "The category has product cards")
	errHasRequests = httpx.NewError(http.StatusConflict, "category_has_requests", "Catalogue requests refer to the category")
)

// Category is a flat row as stored in PostgreSQL.
type Category struct {
	ID        int64
	ParentID  *int64
	Slug      string
	NameKK    string
	NameRU    string
	SortOrder int
	// DirectProductCount counts active products assigned to this exact category.
	DirectProductCount int
}

// Node is a category with its children, as returned by GET /categories/tree.
type Node struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Slug     string `json:"slug"`
	NameKK   string `json:"name_kk"`
	NameRU   string `json:"name_ru"`
	Depth    int    `json:"depth"`
	// ProductCount includes products of all descendant categories.
	ProductCount int     `json:"product_count"`
	Children     []*Node `json:"children"`
}

// AdminCategory is a row of the full tree for category managers, including
// inactive categories and what blocks deletion.
type AdminCategory struct {
	ID        int64  `json:"id"`
	ParentID  *int64 `json:"parent_id"`
	Slug      string `json:"slug"`
	NameKK    string `json:"name_kk"`
	NameRU    string `json:"name_ru"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
	Version   int    `json:"version"`
	// Direct links that prevent deletion: every card (demo, blocked, inactive
	// included), child categories and catalogue requests.
	Products int `json:"product_count"`
	Children int `json:"child_count"`
	Requests int `json:"request_count"`
}

// Input is what a category manager edits. The slug is set once: it is part of
// public catalogue addresses.
type Input struct {
	ParentID  *int64 `json:"parent_id"`
	Slug      string `json:"slug"`
	NameKK    string `json:"name_kk"`
	NameRU    string `json:"name_ru"`
	SortOrder int    `json:"sort_order"`
}
