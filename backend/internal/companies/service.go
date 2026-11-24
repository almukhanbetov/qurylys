package companies

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
	"github.com/qurylys/qurylys/backend/internal/validate"
)

type Service struct {
	repo  *Repository
	limit *ratelimit.Limiter
}

func NewService(repo *Repository, limit *ratelimit.Limiter) *Service {
	return &Service{repo: repo, limit: limit}
}

// clean validates and normalises company details.
func clean(d Details) (Details, error) {
	var err error
	for _, f := range []struct {
		field string
		value *string
		max   int
	}{
		{"name", &d.Name, 200}, {"city", &d.City, 100},
		{"legal_address", &d.LegalAddress, 300}, {"actual_address", &d.ActualAddress, 300},
		{"representative_name", &d.Representative, 120},
	} {
		if *f.value, err = validate.Text(f.field, *f.value, f.max); err != nil {
			return d, err
		}
	}
	if d.BIN, err = validate.BIN(d.BIN); err != nil {
		return d, err
	}
	if d.WorkPhone, err = validate.PhoneNumber("work_phone", d.WorkPhone); err != nil {
		return d, err
	}
	if !d.IsBuyer && !d.IsSupplier {
		return d, httpx.Invalid("company_type", "choose buyer, supplier or both")
	}
	return d, nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, d Details) (int64, error) {
	d, err := clean(d)
	if err != nil {
		return 0, err
	}
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("company:create:%d", p.UserID), Max: 20, Window: 24 * time.Hour}); err != nil {
		return 0, err
	}
	// The slug is a random technical key: it must not depend on editable names.
	return s.repo.create(ctx, p, "c-"+strings.ToLower(rand.Text()), d)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, id int64, d Details) error {
	// Authorise before validating, so a stranger learns nothing from the answer.
	if _, err := Authorize(ctx, s.repo.db, p, id, true, false); err != nil {
		return err
	}
	d, err := clean(d)
	if err != nil {
		return err
	}
	return s.repo.update(ctx, p, id, d)
}

func (s *Service) SetRole(ctx context.Context, p auth.Principal, id, userID int64, role string) error {
	if role != RoleOwner && role != RoleEmployee {
		return httpx.Invalid("role", "must be owner or employee")
	}
	return s.repo.setRole(ctx, p, id, userID, role)
}

func (s *Service) Invite(ctx context.Context, p auth.Principal, id int64, contact string) error {
	kind, contact, err := validate.Contact(contact)
	if err != nil {
		return err
	}
	if err := s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("invite:%d", p.UserID), Max: 100, Window: 24 * time.Hour}); err != nil {
		return err
	}
	return s.repo.invite(ctx, p, id, kind, contact)
}
