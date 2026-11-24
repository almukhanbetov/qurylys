package companies

import (
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const (
	RoleOwner    = "owner"
	RoleEmployee = "employee"

	statusActive       = "active"
	invitationLifetime = 14 * 24 * time.Hour
	maxPendingInvites  = 50
)

var (
	errNotFound       = httpx.NewError(http.StatusNotFound, "company_not_found", "Company not found")
	errMemberNotFound = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errForbidden      = httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	errContext        = httpx.NewError(http.StatusConflict, "company_context_mismatch", "Another company is selected")
	errSuspended      = httpx.NewError(http.StatusForbidden, "company_suspended", "The company is suspended")
	errBINTaken       = httpx.NewError(http.StatusConflict, "bin_taken", "A company with this BIN is already registered")
	errLastOwner      = httpx.NewError(http.StatusConflict, "last_owner", "The company must keep at least one owner")
	errHasOffers      = httpx.NewError(http.StatusConflict, "supplier_has_offers", "The company has offers and must stay a supplier")
	errInvited        = httpx.NewError(http.StatusConflict, "already_invited", "This contact is already invited or the limit is reached")
	errAlreadyMember  = httpx.NewError(http.StatusConflict, "already_member", "Already a member")
)

// Details are the fields a company owner fills in.
type Details struct {
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

type Company struct {
	ID int64 `json:"id"`
	Details
	Status string `json:"status"`
	// Role of the requesting user.
	Role string `json:"role"`
}

type Member struct {
	UserID   int64  `json:"user_id"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	// Contacts are shown to owners only.
	Email *string `json:"email,omitempty"`
	Phone *string `json:"phone,omitempty"`
	IsMe  bool    `json:"is_me"`
}

type Invitation struct {
	ID        int64     `json:"id"`
	Contact   string    `json:"contact"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MyInvitation struct {
	ID          int64     `json:"id"`
	CompanyName string    `json:"company_name"`
	BIN         *string   `json:"bin"`
	InvitedBy   *string   `json:"invited_by"`
	ExpiresAt   time.Time `json:"expires_at"`
}
