package offers

import (
	"context"
	"errors"
)

var ErrProductNotFound = errors.New("product not found")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ForProduct distinguishes "unknown product" (error) from "no offers" (empty list).
func (s *Service) ForProduct(ctx context.Context, productID int64) ([]Offer, error) {
	ok, err := s.repo.ProductVisible(ctx, productID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrProductNotFound
	}
	return s.repo.ListByProduct(ctx, productID)
}
