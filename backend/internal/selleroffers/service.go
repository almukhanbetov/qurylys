package selleroffers

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/quantity"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
)

type Service struct {
	repo  *Repository
	limit *ratelimit.Limiter
}

func NewService(repo *Repository, limit *ratelimit.Limiter) *Service {
	return &Service{repo: repo, limit: limit}
}

func text(field, raw string, max int) (string, error) {
	v := strings.TrimSpace(raw)
	if utf8.RuneCountInString(v) > max || strings.ContainsFunc(v, unicode.IsControl) {
		return "", httpx.Invalid(field, "invalid or too long")
	}
	return v, nil
}

// validate checks and normalises the input for the card's base unit. Nothing is
// rounded or converted silently: wrong values are refused.
func (in Input) validate(unit string) (Input, error) {
	var err error
	if in.SellerSKU, err = text("seller_sku", in.SellerSKU, 64); err != nil {
		return in, err
	}
	if in.City, err = text("city", in.City, 100); err != nil {
		return in, err
	}
	if in.VAT != "" && in.VAT != "with_vat" && in.VAT != "without_vat" {
		return in, httpx.Invalid("vat_status", "unknown")
	}
	var pack quantity.Q
	if in.PackSize != "" {
		var ok bool
		if pack, ok = quantity.Parse(in.PackSize); !ok || pack <= 0 || (quantity.Discrete(unit) && !pack.Whole()) {
			return in, httpx.Invalid("pack_size", "does not fit the unit")
		}
		in.PackSize = pack.String()
	}
	stock, ok := quantity.Parse(in.Stock)
	if !ok || (quantity.Discrete(unit) && !stock.Whole()) {
		return in, httpx.Invalid("stock_quantity", "does not fit the unit")
	}
	in.Stock = stock.String()
	for _, x := range []struct {
		field    string
		level    *Level
		priceReq bool
	}{{"retail", &in.Retail, false}, {"wholesale", in.Wholesale, true}, {"large_wholesale", in.LargeWholesale, true}} {
		if x.level == nil {
			continue
		}
		l := *x.level // do not modify the caller's value
		if err := l.validate(x.field, unit, pack, stock, x.priceReq); err != nil {
			return in, err
		}
		switch x.field {
		case "retail":
			in.Retail = l
		case "wholesale":
			in.Wholesale = &l
		default:
			in.LargeWholesale = &l
		}
	}
	return in, nil
}

func (l *Level) validate(field, unit string, pack, stock quantity.Q, priceRequired bool) error {
	if l.PriceTiyn == nil && priceRequired {
		return httpx.Invalid(field+"_price", "is required")
	}
	if l.PriceTiyn != nil && (*l.PriceTiyn < 1 || *l.PriceTiyn > quantity.MaxMoney) {
		return httpx.Invalid(field+"_price", "must be a positive amount")
	}
	if l.SaleMode != "piece" && l.SaleMode != "pack" {
		return httpx.Invalid(field+"_mode", "must be piece or pack")
	}
	if l.SaleMode == "pack" && pack == 0 {
		return httpx.Invalid("pack_size", "is required for sale by pack")
	}
	minQ, ok1 := quantity.Parse(l.Min)
	step, ok2 := quantity.Parse(l.Step)
	if !ok1 || !ok2 || !quantity.SaleRuleOK(unit, l.SaleMode, pack, minQ, step) {
		return httpx.Invalid(field+"_quantity", "the minimum and step do not fit the unit or the pack")
	}
	l.Min, l.Step = minQ.String(), step.String()
	if l.PriceTiyn != nil {
		for _, q := range []quantity.Q{minQ, pack} {
			if _, ok := quantity.Total(*l.PriceTiyn, q); !ok {
				return httpx.Invalid(field+"_price", "the amount is too large")
			}
		}
	}
	return nil
}

func (s *Service) hit(ctx context.Context, p auth.Principal) error {
	return s.limit.Hit(ctx, ratelimit.Rule{Key: fmt.Sprintf("offers:write:%d", p.UserID), Max: 600, Window: time.Hour})
}

func (s *Service) Create(ctx context.Context, p auth.Principal, companyID, productID int64, in Input) (int64, error) {
	if err := s.hit(ctx, p); err != nil {
		return 0, err
	}
	return s.repo.create(ctx, p, companyID, productID, in)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, companyID, id int64, version int, in Input) error {
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.update(ctx, p, companyID, id, version, in)
}

func (s *Service) SetState(ctx context.Context, p auth.Principal, companyID, id int64, version int, state string) error {
	if err := s.hit(ctx, p); err != nil {
		return err
	}
	return s.repo.setState(ctx, p, companyID, id, version, state)
}
