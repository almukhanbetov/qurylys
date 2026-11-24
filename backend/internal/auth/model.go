package auth

import (
	"net/http"
	"slices"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const (
	RoleMRX   = "mrx"
	RoleAdmin = "admin"

	purposeRegister   = "register"
	purposeReset      = "reset_password"
	purposeAddContact = "add_contact"

	sessionLifetime = 30 * 24 * time.Hour
	sessionIdle     = 7 * 24 * time.Hour
	challengeTTL    = 15 * time.Minute
	maxCodeAttempts = 5
)

var (
	errUnauthorized  = httpx.NewError(http.StatusUnauthorized, "unauthorized", "Sign in to continue")
	errForbidden     = httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	errCredentials   = httpx.NewError(http.StatusUnauthorized, "invalid_credentials", "Wrong contact or password")
	errBadCode       = httpx.NewError(http.StatusBadRequest, "invalid_code", "The code is wrong or has expired")
	errContactTaken  = httpx.NewError(http.StatusConflict, "contact_taken", "This contact is already in use")
	errNoDelivery    = httpx.NewError(http.StatusServiceUnavailable, "delivery_unavailable", "Contact confirmation is not available")
	errChannel       = httpx.NewError(http.StatusBadRequest, "channel_unavailable", "This type of contact cannot be confirmed")
	errNotFound      = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errWrongPassword = httpx.NewError(http.StatusBadRequest, "wrong_password", "The current password is wrong")
)

// Principal is the authenticated user of a request.
type Principal struct {
	UserID          int64
	SessionID       int64
	ActiveCompanyID int64 // 0 = none; always a company the user belongs to
	Roles           []string
}

func (p Principal) Has(role string) bool { return slices.Contains(p.Roles, role) }

type user struct {
	ID           int64
	PasswordHash string
	Status       string
}

type Membership struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	IsBuyer    bool   `json:"is_buyer"`
	IsSupplier bool   `json:"is_supplier"`
	Status     string `json:"status"`
	// Verification: see the company_verification view.
	Verification string `json:"verification"`
}

type Me struct {
	ID                 int64        `json:"id"`
	FullName           string       `json:"full_name"`
	Email              *string      `json:"email"`
	Phone              *string      `json:"phone"`
	PlatformRoles      []string     `json:"platform_roles"`
	Companies          []Membership `json:"companies"`
	ActiveCompanyID    *int64       `json:"active_company_id"`
	PendingInvitations int          `json:"pending_invitations"`
}

type Session struct {
	ID         int64     `json:"id"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

type AdminUser struct {
	ID            int64    `json:"id"`
	FullName      string   `json:"full_name"`
	Email         *string  `json:"email"`
	Phone         *string  `json:"phone"`
	PlatformRoles []string `json:"platform_roles"`
}

type challenge struct {
	ID           int64
	Purpose      string
	ContactType  string
	Contact      string
	UserID       *int64
	FullName     *string
	PasswordHash *string
	CodeHash     []byte
}
