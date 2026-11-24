// Package verification handles company verification: applications, private
// documents and reviewer (MRX) decisions.
package verification

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

	MaxFileSize = 5 << 20
	// maxDocuments is per submission; maxStored also counts replaced files kept for history.
	maxDocuments = 10
	maxStored    = 40
	maxComment   = 2000
	pageSize     = 20
)

var (
	errNotFound    = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errNotEditable = httpx.NewError(http.StatusConflict, "verification_not_editable", "The application cannot be changed now")
	errOpen        = httpx.NewError(http.StatusConflict, "verification_exists", "An application already exists")
	errVerified    = httpx.NewError(http.StatusConflict, "already_verified", "The company is already verified")
	errIncomplete  = httpx.NewError(http.StatusConflict, "company_incomplete", "Fill in all company details first")
	errDocRequired = httpx.NewError(http.StatusConflict, "document_required", "Attach a registration document")
	errTooMany     = httpx.NewError(http.StatusConflict, "too_many_documents", "Too many documents")
	errFileType    = httpx.NewError(http.StatusUnsupportedMediaType, "file_type", "Only PDF, JPEG and PNG files are accepted")
	errFileSize    = httpx.NewError(http.StatusRequestEntityTooLarge, "file_too_large", "The file is too large")
	errStale       = httpx.NewError(http.StatusConflict, "verification_conflict", "The application has changed or is already decided")
	errConflict    = httpx.NewError(http.StatusForbidden, "conflict_of_interest", "You are or were a member of this company")
	errSuspended   = httpx.NewError(http.StatusForbidden, "company_suspended", "The company is suspended")
)

// Requisites are the company details covered by an application.
type Requisites struct {
	Name           string `json:"name"`
	BIN            string `json:"bin"`
	City           string `json:"city"`
	LegalAddress   string `json:"legal_address"`
	ActualAddress  string `json:"actual_address"`
	WorkPhone      string `json:"work_phone"`
	Representative string `json:"representative_name"`
	IsBuyer        bool   `json:"is_buyer"`
	IsSupplier     bool   `json:"is_supplier"`
}

func (r Requisites) complete() bool {
	return r.Name != "" && r.BIN != "" && r.City != "" && r.LegalAddress != "" && r.ActualAddress != "" &&
		r.WorkPhone != "" && r.Representative != ""
}

type Document struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int       `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

type Event struct {
	ID           int64  `json:"id"`
	SubmissionNo int    `json:"submission_no"`
	Action       string `json:"action"`
	// Actor is hidden from the company when the actor is a reviewer.
	Actor     string    `json:"actor"`
	Comment   string    `json:"comment"`
	Changed   []Change  `json:"changed"`
	CreatedAt time.Time `json:"created_at"`
}

type Change struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type request struct {
	ID           int64
	CompanyID    int64
	Status       string
	SubmissionNo int
	Comment      string
	Snapshot     *Requisites
	SubmittedAt  *time.Time
	DecidedAt    *time.Time
}

// CompanyView is what company members see.
type CompanyView struct {
	// Status: none | draft | in_review | needs_changes | approved | rejected | reverification
	Status       string     `json:"status"`
	Comment      string     `json:"comment"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	DecidedAt    *time.Time `json:"decided_at"`
	CanEdit      bool       `json:"can_edit"`
	Requisites   Requisites `json:"requisites"`
	Documents    []Document `json:"documents"` // owners only
	Events       []Event    `json:"events"`
	MaxFileSize  int        `json:"max_file_size"`
	MaxDocuments int        `json:"max_documents"`
}

type ListItem struct {
	ID           int64      `json:"id"`
	CompanyName  string     `json:"company_name"`
	BIN          string     `json:"bin"`
	City         string     `json:"city"`
	Status       string     `json:"status"`
	SubmissionNo int        `json:"submission_no"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	DecidedAt    *time.Time `json:"decided_at"`
	Conflict     bool       `json:"conflict"`
}

type ListPage struct {
	Items      []ListItem `json:"items"`
	Page       int        `json:"page"`
	Total      int        `json:"total"`
	TotalPages int        `json:"total_pages"`
}

type ReviewView struct {
	ID           int64      `json:"id"`
	Status       string     `json:"status"`
	SubmissionNo int        `json:"submission_no"`
	Comment      string     `json:"comment"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	DecidedAt    *time.Time `json:"decided_at"`
	Requisites   Requisites `json:"requisites"`
	Documents    []Document `json:"documents"`
	Events       []Event    `json:"events"`
}
