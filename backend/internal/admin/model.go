// Package admin is the administrator's read-only view of users, platform staff
// and company membership. Grants are changed in package auth; companies manage
// their own members (package companies). No passwords, hashes, tokens or codes
// are ever selected here.
package admin

import (
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const pageSize = 20

var errNotFound = httpx.NewError(http.StatusNotFound, "not_found", "Not found")

// UserFilter values come from fixed sets (see the handler); q is a parameter.
type UserFilter struct {
	Q          string
	Access     string // "", staff, none, admin, mrx or a grantable permission
	Membership string // "", with, without
	Status     string // "", active, blocked
	Page       int
}

type UserItem struct {
	ID            int64     `json:"id"`
	FullName      string    `json:"full_name"`
	Email         *string   `json:"email"`
	Phone         *string   `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	PlatformRoles []string  `json:"platform_roles"`
	Permissions   []string  `json:"platform_permissions"`
	CompanyCount  int       `json:"company_count"`
}

type Page[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Grant is a role or permission held now.
type Grant struct {
	Kind      string    `json:"kind"` // role | permission
	Value     string    `json:"value"`
	GrantedAt time.Time `json:"granted_at"`
}

type GrantEvent struct {
	Action    string    `json:"action"` // platform_role.granted, platform_permission.revoked, ...
	Value     string    `json:"value"`
	Actor     string    `json:"actor"` // empty: operator command
	CreatedAt time.Time `json:"created_at"`
}

type Membership struct {
	CompanyID    int64     `json:"company_id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	IsDemo       bool      `json:"is_demo"`
	Verification string    `json:"verification"`
	JoinedAt     time.Time `json:"joined_at"`
}

// User is the profile an administrator sees. Contacts are always confirmed:
// an unconfirmed contact is never stored on the user.
type User struct {
	ID              int64        `json:"id"`
	FullName        string       `json:"full_name"`
	Email           *string      `json:"email"`
	EmailVerifiedAt *time.Time   `json:"email_verified_at"`
	Phone           *string      `json:"phone"`
	PhoneVerifiedAt *time.Time   `json:"phone_verified_at"`
	Status          string       `json:"status"`
	CreatedAt       time.Time    `json:"created_at"`
	LastSeenAt      *time.Time   `json:"last_seen_at"`
	Grants          []Grant      `json:"grants"`
	History         []GrantEvent `json:"history"`
	Memberships     []Membership `json:"memberships"`
}

type CompanyItem struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	BIN          *string `json:"bin"`
	City         *string `json:"city"`
	IsBuyer      bool    `json:"is_buyer"`
	IsSupplier   bool    `json:"is_supplier"`
	Status       string  `json:"status"`
	IsDemo       bool    `json:"is_demo"`
	Verification string  `json:"verification"`
	MemberCount  int     `json:"member_count"`
}

// Member is what an administrator may see of a company's staff: no contacts,
// invitations or company documents.
type Member struct {
	UserID   int64     `json:"user_id"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type CompanyMembers struct {
	Company CompanyItem `json:"company"`
	Members []Member    `json:"members"`
}
