package catalog

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/products"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
)

type Service struct {
	repo  *Repository
	limit *ratelimit.Limiter
}

func NewService(repo *Repository, limit *ratelimit.Limiter) *Service {
	return &Service{repo: repo, limit: limit}
}

var sourceKinds = []string{"manufacturer", "packaging", "official_catalog"}

// text allows an empty value (drafts); multiline keeps line breaks.
func text(field, raw string, max int, multiline bool) (string, error) {
	v := strings.TrimSpace(raw)
	bad := strings.ContainsFunc(v, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))
	})
	if bad || utf8.RuneCountInString(v) > max {
		return "", httpx.Invalid(field, "invalid or too long")
	}
	return v, nil
}

// clean validates what is filled in; completeness is checked on submission.
func clean(f Fields) (Fields, error) {
	var err error
	for _, x := range []struct {
		field string
		value *string
		max   int
		multi bool
	}{
		{"name_kk", &f.NameKK, 200, false}, {"name_ru", &f.NameRU, 200, false}, {"brand", &f.Brand, 100, false},
		{"description_kk", &f.DescriptionKK, 2000, true}, {"description_ru", &f.DescriptionRU, 2000, true},
		{"gtin_source", &f.Source, 500, false},
	} {
		if *x.value, err = text(x.field, *x.value, x.max, x.multi); err != nil {
			return f, err
		}
	}
	if f.Unit != "" && !slices.Contains(products.Units, f.Unit) {
		return f, httpx.Invalid("unit", "unknown unit")
	}
	if f.GTIN != nil && strings.TrimSpace(*f.GTIN) == "" {
		f.GTIN = nil
	}
	if f.GTIN != nil {
		v, ok := NormalizeGTIN(*f.GTIN)
		if !ok {
			return f, httpx.Invalid("gtin", "must be a GTIN with a correct check digit")
		}
		f.GTIN = &v
	}
	if f.SourceKind != nil && *f.SourceKind == "" {
		f.SourceKind = nil
	}
	if f.SourceKind != nil && !slices.Contains(sourceKinds, *f.SourceKind) {
		return f, httpx.Invalid("gtin_source", "unknown source")
	}
	return f, nil
}

// complete lists what a moderator needs. The GTIN is declared, not yet confirmed.
func (f Fields) complete() error {
	switch {
	case f.CategoryID == nil:
		return errCategory
	case f.NameKK == "":
		return httpx.Invalid("name_kk", "is required")
	case f.NameRU == "":
		return httpx.Invalid("name_ru", "is required")
	case f.Unit == "":
		return httpx.Invalid("unit", "is required")
	case f.GTIN == nil:
		return httpx.Invalid("gtin", "is required")
	case f.SourceKind == nil || f.Source == "":
		return httpx.Invalid("gtin_source", "is required")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, companyID int64, f Fields) (int64, error) {
	f, err := clean(f)
	if err != nil {
		return 0, err
	}
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("catalog:create:%d", p.UserID), Max: 100, Window: 24 * time.Hour}); err != nil {
		return 0, err
	}
	return s.repo.create(ctx, p, companyID, f)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, companyID, id int64, f Fields) error {
	f, err := clean(f)
	if err != nil {
		return err
	}
	return s.repo.update(ctx, p, companyID, id, f)
}

func (s *Service) ConfirmGTIN(ctx context.Context, p auth.Principal, id int64, submissionNo int, kind, source string) error {
	source, err := text("gtin_source", source, 500, false)
	if err != nil {
		return err
	}
	if source == "" || !slices.Contains(sourceKinds, kind) {
		return httpx.Invalid("gtin_source", "the verified source is required")
	}
	return s.repo.confirmGTIN(ctx, p, id, submissionNo, kind, source)
}

var decisions = map[string]struct{ status, action string }{
	"approve":         {StatusApproved, "approved"},
	"request_changes": {StatusNeedsChanges, "changes_requested"},
	"reject":          {StatusRejected, "rejected"},
}

func (s *Service) Decide(ctx context.Context, p auth.Principal, id int64, submissionNo int, decision, comment string) error {
	d, ok := decisions[decision]
	if !ok {
		return httpx.Invalid("decision", "must be approve, request_changes or reject")
	}
	comment, err := text("comment", comment, maxComment, true)
	if err != nil {
		return err
	}
	if comment == "" && d.status != StatusApproved {
		return httpx.Invalid("comment", "is required")
	}
	return s.repo.decide(ctx, p, id, submissionNo, d.status, d.action, comment)
}
