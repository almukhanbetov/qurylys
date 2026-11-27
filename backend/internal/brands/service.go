package brands

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
)

type Service struct {
	repo  *Repository
	limit *ratelimit.Limiter
}

func NewService(repo *Repository, limit *ratelimit.Limiter) *Service {
	return &Service{repo: repo, limit: limit}
}

// Normalize trims and collapses spaces; the database stores names in this form.
func Normalize(raw string) string { return strings.Join(strings.Fields(raw), " ") }

func name(raw string) (string, error) {
	v := Normalize(raw)
	if v == "" || utf8.RuneCountInString(v) > 100 || strings.ContainsFunc(v, unicode.IsControl) {
		return "", httpx.Invalid("brand_name", "is required, up to 100 characters")
	}
	return v, nil
}

func (s *Service) hit(ctx context.Context, p auth.Principal) error {
	return s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("brands:write:%d", p.UserID), Max: 600, Window: time.Hour})
}

func (s *Service) Create(ctx context.Context, p auth.Principal, raw string) (int64, error) {
	n, err := name(raw)
	if err != nil {
		return 0, err
	}
	if err := s.hit(ctx, p); err != nil {
		return 0, err
	}
	return s.repo.create(ctx, p, n)
}

func (s *Service) Rename(ctx context.Context, p auth.Principal, id int64, version int, raw string) error {
	n, err := name(raw)
	if err != nil {
		return err
	}
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.rename(ctx, p, id, version, n)
}

func (s *Service) Delete(ctx context.Context, p auth.Principal, id int64, version int) error {
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.remove(ctx, p, id, version)
}

// Link takes the brand text exactly as stored on the cards.
func (s *Service) Link(ctx context.Context, p auth.Principal, id int64, text string) (int64, error) {
	if text == "" || utf8.RuneCountInString(text) > 200 || !utf8.ValidString(text) {
		return 0, httpx.Invalid("text", "is required")
	}
	if err := s.hit(ctx, p); err != nil {
		return 0, err
	}
	return s.repo.link(ctx, p, id, text)
}
