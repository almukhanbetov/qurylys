package cards

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/catalog"
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

func text(field, raw string, max int, multiline, required bool) (string, error) {
	v := strings.TrimSpace(raw)
	if !multiline {
		v = strings.Join(strings.Fields(v), " ")
	}
	bad := strings.ContainsFunc(v, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))
	})
	if bad || utf8.RuneCountInString(v) > max || (required && v == "") {
		return "", httpx.Invalid(field, "is required or too long")
	}
	return v, nil
}

func (f Fields) clean() (Fields, error) {
	var err error
	if f.CategoryID < 1 {
		return f, errCategory
	}
	if f.BrandID != nil && *f.BrandID < 1 {
		return f, errBrand
	}
	for _, x := range []struct {
		field    string
		v        *string
		max      int
		multi    bool
		required bool
	}{
		{"name_kk", &f.NameKK, 200, false, true}, {"name_ru", &f.NameRU, 200, false, true},
		{"description_kk", &f.DescriptionKK, 2000, true, false}, {"description_ru", &f.DescriptionRU, 2000, true, false},
	} {
		if *x.v, err = text(x.field, *x.v, x.max, x.multi, x.required); err != nil {
			return f, err
		}
	}
	if !slices.Contains(products.Units, f.Unit) {
		return f, httpx.Invalid("unit", "unknown unit")
	}
	return f, nil
}

func (g GTIN) clean() (GTIN, error) {
	v, ok := catalog.NormalizeGTIN(g.GTIN)
	if !ok {
		return g, errGTINUnknown
	}
	g.GTIN = v
	if !slices.Contains(catalog.SourceKinds, g.SourceKind) {
		return g, httpx.Invalid("gtin_source", "unknown source")
	}
	var err error
	if g.Source, err = text("gtin_source", g.Source, 500, false, true); err != nil {
		return g, err
	}
	return g, nil
}

func reason(raw string, required bool) (string, error) {
	v, err := text("reason", raw, 2000, true, required)
	if err != nil {
		return "", errReason
	}
	return v, nil
}

var numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// sameNumbers guards N3: while suppliers offer the card, the numbers in its name
// (volume, size, grade) stay as they are; "750 мл" cannot become "500 мл".
func sameNumbers(a, b string) bool {
	norm := func(s string) []string {
		out := numberRe.FindAllString(s, -1)
		for i := range out {
			out[i] = strings.ReplaceAll(out[i], ",", ".")
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(norm(a), norm(b))
}

func (s *Service) hit(ctx context.Context, p auth.Principal) error {
	return s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("cards:write:%d", p.UserID), Max: 600, Window: time.Hour})
}

func (s *Service) Create(ctx context.Context, p auth.Principal, f Fields, g GTIN) (int64, error) {
	f, err := f.clean()
	if err != nil {
		return 0, err
	}
	if g, err = g.clean(); err != nil {
		return 0, err
	}
	if err := s.hit(ctx, p); err != nil {
		return 0, err
	}
	return s.repo.create(ctx, p, f, g)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, id int64, version int, f Fields, why string) error {
	f, err := f.clean()
	if err != nil {
		return err
	}
	if why, err = reason(why, true); err != nil {
		return err
	}
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.update(ctx, p, id, version, f, why)
}

func (s *Service) ConfirmGTIN(ctx context.Context, p auth.Principal, id int64, version int, g GTIN, why string) error {
	g, err := g.clean()
	if err != nil {
		return err
	}
	if why, err = reason(why, false); err != nil {
		return err
	}
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.confirmGTIN(ctx, p, id, version, g, why)
}

func (s *Service) SetBlocked(ctx context.Context, p auth.Principal, id int64, version int, block bool, why string) error {
	why, err := reason(why, block)
	if err != nil {
		return err
	}
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.setBlocked(ctx, p, id, version, block, why)
}
