// Package catalog handles suppliers' requests for new shared product cards and
// their moderation. Offers are not created or published here.
package catalog

import (
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const (
	StatusDraft        = "draft"
	StatusInReview     = "in_review"
	StatusNeedsChanges = "needs_changes"
	StatusApproved     = "approved"
	StatusRejected     = "rejected"

	maxOpen    = 50 // unfinished requests per company
	maxComment = 2000
	pageSize   = 20
)

var (
	errNotFound    = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errNotSupplier = httpx.NewError(http.StatusForbidden, "not_supplier", "The company is not a supplier")
	errNotEditable = httpx.NewError(http.StatusConflict, "catalog_request_not_editable", "The request cannot be changed now")
	errTooMany     = httpx.NewError(http.StatusConflict, "too_many_requests_open", "Too many unfinished requests")
	errStale       = httpx.NewError(http.StatusConflict, "catalog_request_conflict", "The request has changed or is already decided")
	errConflict    = httpx.NewError(http.StatusForbidden, "conflict_of_interest", "You are or were a member of this company")
	errGTINExists  = httpx.NewError(http.StatusConflict, "gtin_exists", "A catalogue card with this GTIN already exists")
	errUnconfirmed = httpx.NewError(http.StatusConflict, "gtin_unconfirmed", "Confirm the GTIN first")
	errCategory    = httpx.Invalid("category", "unknown category")
)

// Fields are filled in by the supplier.
type Fields struct {
	CategoryID    *int64  `json:"category_id"`
	NameKK        string  `json:"name_kk"`
	NameRU        string  `json:"name_ru"`
	Brand         string  `json:"brand"`
	Unit          string  `json:"unit"`
	DescriptionKK string  `json:"description_kk"`
	DescriptionRU string  `json:"description_ru"`
	GTIN          *string `json:"gtin"`
	SourceKind    *string `json:"gtin_source_kind"`
	Source        string  `json:"gtin_source"`
}

type Request struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	SubmissionNo int    `json:"submission_no"`
	Fields
	CategoryKK    string     `json:"category_kk"`
	CategoryRU    string     `json:"category_ru"`
	GTINConfirmed bool       `json:"gtin_confirmed"`
	Comment       string     `json:"comment"`
	ProductID     *int64     `json:"product_id"`
	CatalogNo     *string    `json:"catalog_no"`
	SubmittedAt   *time.Time `json:"submitted_at"`
	DecidedAt     *time.Time `json:"decided_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	companyID   int64
	companyName string
	authorID    int64
}

type Event struct {
	ID           int64     `json:"id"`
	SubmissionNo int       `json:"submission_no"`
	Action       string    `json:"action"`
	Actor        string    `json:"actor"` // empty for moderators in the company's view
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type CompanyRequest struct {
	Request
	Events []Event `json:"events"`
}

type ListItem struct {
	Request
	CompanyName string `json:"company_name"`
	Conflict    bool   `json:"conflict"`
}

type ListPage struct {
	Items      []ListItem `json:"items"`
	Page       int        `json:"page"`
	Total      int        `json:"total"`
	TotalPages int        `json:"total_pages"`
}

type SimilarProduct struct {
	ID        int64   `json:"id"`
	CatalogNo string  `json:"catalog_no"`
	NameKK    string  `json:"name_kk"`
	NameRU    string  `json:"name_ru"`
	Brand     *string `json:"brand"`
	GTIN      *string `json:"gtin"`
	SameGTIN  bool    `json:"same_gtin"`
	IsDemo    bool    `json:"is_demo"`
}

type Review struct {
	Request
	CompanyName     string           `json:"company_name"`
	ConfirmedKind   *string          `json:"confirmed_source_kind"`
	ConfirmedSource *string          `json:"confirmed_source"`
	Similar         []SimilarProduct `json:"similar"`
	// SameGTINRequests counts other requests under review with the same GTIN.
	SameGTINRequests int     `json:"same_gtin_requests"`
	Events           []Event `json:"events"`
}
