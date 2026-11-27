// Package cards is the administration of shared product cards for holders of
// catalog.moderate (R11): list, create, edit, confirm GTIN, block and unblock.
// Offers are never read for prices or changed here: a card and the suppliers'
// offers stay separate (S3.1). Rules: docs/catalog.md, step 3.5 of the plan.
package cards

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const pageSize = 20

var (
	errNotFound    = httpx.NewError(http.StatusNotFound, "not_found", "Not found")
	errForbidden   = httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	errConflict    = httpx.NewError(http.StatusForbidden, "conflict_of_interest", "You are or were a member of a company linked to this card")
	errStale       = httpx.NewError(http.StatusConflict, "card_conflict", "The card has changed, reload it")
	errDemo        = httpx.NewError(http.StatusConflict, "card_demo", "Demo cards are maintained by the seed only")
	errLocked      = httpx.NewError(http.StatusConflict, "card_locked", "The card has supplier offers: unit, GTIN, category and brand cannot change. Create a new card for another product")
	errIdentity    = httpx.NewError(http.StatusConflict, "card_identity_change", "Numbers in the name cannot change while the card has supplier offers. Create a new card for another product")
	errGTINExists  = httpx.NewError(http.StatusConflict, "gtin_exists", "A catalogue card with this GTIN already exists")
	errBlocked     = httpx.NewError(http.StatusConflict, "card_already_blocked", "The card is already blocked")
	errNotBlocked  = httpx.NewError(http.StatusConflict, "card_not_blocked", "The card is not blocked")
	errCategory    = httpx.Invalid("category", "unknown category")
	errBrand       = httpx.Invalid("brand", "unknown brand")
	errReason      = httpx.Invalid("reason", "is required")
	errNoChange    = httpx.NewError(http.StatusConflict, "card_unchanged", "Nothing has changed")
	errGTINUnknown = httpx.Invalid("gtin", "must be a GTIN with a correct check digit")
)

// Filter values come from fixed sets (see the handler).
type Filter struct {
	Q          string
	CategoryID int64
	Brand      string // "", "none" or a brand id
	State      string
	Page       int
}

type Item struct {
	ID          int64   `json:"id"`
	CatalogNo   string  `json:"catalog_no"`
	NameKK      string  `json:"name_kk"`
	NameRU      string  `json:"name_ru"`
	Brand       *string `json:"brand"`
	BrandID     *int64  `json:"brand_id"`
	CategoryID  int64   `json:"category_id"`
	CategoryKK  string  `json:"category_kk"`
	CategoryRU  string  `json:"category_ru"`
	Unit        string  `json:"unit"`
	GTIN        *string `json:"gtin"`
	NeedsReview bool    `json:"needs_review"`
	IsDemo      bool    `json:"is_demo"`
	Blocked     bool    `json:"blocked"`
	// Offers counts real supplier offers of any state; prices are not read.
	Offers int `json:"offer_count"`
}

type Page struct {
	Items      []Item `json:"items"`
	Page       int    `json:"page"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}

// Fields are what an editor changes. The catalogue number is assigned by the
// database; the GTIN changes only through ConfirmGTIN.
type Fields struct {
	CategoryID    int64  `json:"category_id"`
	NameKK        string `json:"name_kk"`
	NameRU        string `json:"name_ru"`
	DescriptionKK string `json:"description_kk"`
	DescriptionRU string `json:"description_ru"`
	BrandID       *int64 `json:"brand_id"`
	Unit          string `json:"unit"`
}

// GTIN is a confirmation: the number and the source it was checked against.
type GTIN struct {
	GTIN       string `json:"gtin"`
	SourceKind string `json:"source_kind"`
	Source     string `json:"source"`
}

type OfferStats struct {
	Total     int `json:"total"`
	Draft     int `json:"draft"`
	Published int `json:"published"`
	Hidden    int `json:"hidden"`
	Demo      int `json:"demo"`
}

type Event struct {
	ID        int64           `json:"id"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor"`
	Comment   string          `json:"comment"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

// Card embeds only Item: embedding Fields as well would make the shared JSON
// names ambiguous, and encoding/json drops ambiguous fields silently.
type Card struct {
	Item
	DescriptionKK   string          `json:"description_kk"`
	DescriptionRU   string          `json:"description_ru"`
	Attributes      json.RawMessage `json:"attributes"`
	GTINSourceKind  *string         `json:"gtin_source_kind"`
	GTINSource      *string         `json:"gtin_source"`
	GTINConfirmedAt *time.Time      `json:"gtin_confirmed_at"`
	BlockedAt       *time.Time      `json:"blocked_at"`
	BlockReason     *string         `json:"block_reason"`
	Version         int             `json:"version"`
	UpdatedAt       time.Time       `json:"updated_at"`
	OfferStats      OfferStats      `json:"offer_stats"`
	RequestID       *int64          `json:"request_id"`
	// Conflict: the viewer is or was in a company linked to the card and may
	// look but not act (R5, plan section 2).
	Conflict bool    `json:"conflict"`
	Events   []Event `json:"events"`
}
