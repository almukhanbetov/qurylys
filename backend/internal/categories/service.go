package categories

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Tree(ctx context.Context) ([]*Node, error) {
	flat, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return BuildTree(flat), nil
}

// BuildTree turns a flat, ordered list into a forest. Input order is preserved
// among siblings. A category whose parent is missing from the list (for example
// an inactive parent) is dropped together with its subtree, because it is not
// reachable from any visible root.
func BuildTree(flat []Category) []*Node {
	nodes := make(map[int64]*Node, len(flat))
	for _, c := range flat {
		nodes[c.ID] = &Node{
			ID:           c.ID,
			ParentID:     c.ParentID,
			Slug:         c.Slug,
			NameKK:       c.NameKK,
			NameRU:       c.NameRU,
			ProductCount: c.DirectProductCount,
			Children:     []*Node{},
		}
	}

	roots := []*Node{}
	for _, c := range flat {
		n := nodes[c.ID]
		if c.ParentID == nil {
			roots = append(roots, n)
			continue
		}
		if parent, ok := nodes[*c.ParentID]; ok {
			parent.Children = append(parent.Children, n)
		}
	}

	for _, r := range roots {
		finalize(r, 0)
	}
	return roots
}

// finalize sets depth and rolls product counts up from descendants.
func finalize(n *Node, depth int) int {
	n.Depth = depth
	total := n.ProductCount
	for _, ch := range n.Children {
		total += finalize(ch, depth+1)
	}
	n.ProductCount = total
	return total
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func name(field, raw string) (string, error) {
	v := strings.Join(strings.Fields(raw), " ")
	if v == "" || utf8.RuneCountInString(v) > 200 || strings.ContainsFunc(v, unicode.IsControl) {
		return "", httpx.Invalid(field, "is required, up to 200 characters")
	}
	return v, nil
}

func (in Input) clean(create bool) (Input, error) {
	var err error
	if in.NameKK, err = name("name_kk", in.NameKK); err != nil {
		return in, err
	}
	if in.NameRU, err = name("name_ru", in.NameRU); err != nil {
		return in, err
	}
	if in.SortOrder < -100000 || in.SortOrder > 100000 {
		return in, httpx.Invalid("sort_order", "must be -100000..100000")
	}
	if create && (!slugRe.MatchString(in.Slug) || len(in.Slug) > 80) {
		return in, httpx.Invalid("slug", "latin letters, digits and single hyphens, up to 80")
	}
	if in.ParentID != nil && *in.ParentID < 1 {
		return in, errParent
	}
	return in, nil
}

func (s *Service) Create(ctx context.Context, p auth.Principal, in Input) (int64, error) {
	in, err := in.clean(true)
	if err != nil {
		return 0, err
	}
	return s.repo.Create(ctx, p, in)
}

func (s *Service) Update(ctx context.Context, p auth.Principal, id int64, version int, in Input) error {
	if in.Slug != "" {
		return httpx.Invalid("slug", "cannot be changed")
	}
	in, err := in.clean(false)
	if err != nil {
		return err
	}
	return s.repo.Update(ctx, p, id, version, in)
}
