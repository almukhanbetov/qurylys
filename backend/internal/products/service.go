package products

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidationError is returned for bad client input; handlers map it to 400.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

var ErrCategoryNotFound = errors.New("category not found")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// RawListParams are untrusted query-string values.
type RawListParams struct {
	Q        string
	Category string
	Brand    string
	Sort     string
	Page     string
	PageSize string
}

// ParseListParams validates and normalises catalogue query parameters.
func ParseListParams(raw RawListParams) (ListFilter, error) {
	f := ListFilter{Sort: SortNewest, Page: 1, PageSize: DefaultPageSize}

	q := strings.TrimSpace(raw.Q)
	if utf8.RuneCountInString(q) > MaxQueryLength {
		return f, &ValidationError{Field: "q", Message: "must be at most 100 characters"}
	}
	f.Terms = splitTerms(q)

	if s := strings.TrimSpace(raw.Category); s != "" {
		if !isSlug(s) {
			return f, &ValidationError{Field: "category", Message: "must be a category slug"}
		}
		f.CategorySlug = s
	}

	if s := strings.TrimSpace(raw.Brand); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 1 {
			return f, &ValidationError{Field: "brand", Message: "must be a brand id"}
		}
		f.BrandID = n
	}

	if s := strings.TrimSpace(raw.Sort); s != "" {
		if _, ok := sortClauses[Sort(s)]; !ok {
			return f, &ValidationError{Field: "sort", Message: "must be one of newest, price_asc, price_desc, offers"}
		}
		f.Sort = Sort(s)
	}

	if s := strings.TrimSpace(raw.Page); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 10000 {
			return f, &ValidationError{Field: "page", Message: "must be an integer from 1 to 10000"}
		}
		f.Page = n
	}

	if s := strings.TrimSpace(raw.PageSize); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxPageSize {
			return f, &ValidationError{Field: "page_size", Message: "must be an integer from 1 to 48"}
		}
		f.PageSize = n
	}

	return f, nil
}

// splitTerms splits the query by whitespace, drops duplicates and keeps at most
// MaxSearchTerms words.
func splitTerms(q string) []string {
	var terms []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(q) {
		key := strings.ToLower(w)
		if seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, w)
		if len(terms) == MaxSearchTerms {
			break
		}
	}
	return terms
}

func isSlug(s string) bool {
	if len(s) > 100 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' && i > 0 && i < len(s)-1:
		default:
			return false
		}
	}
	return true
}

func (s *Service) List(ctx context.Context, f ListFilter) (*Page, error) {
	if f.CategorySlug != "" {
		ok, err := s.repo.CategoryExists(ctx, f.CategorySlug)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrCategoryNotFound
		}
	}

	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return &Page{
		Items:      items,
		Page:       f.Page,
		PageSize:   f.PageSize,
		Total:      total,
		TotalPages: (total + f.PageSize - 1) / f.PageSize,
	}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Detail, error) {
	return s.repo.Get(ctx, id)
}

// ParseID validates a positive integer path parameter.
func ParseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, &ValidationError{Field: "id", Message: "must be a positive integer"}
	}
	return id, nil
}
