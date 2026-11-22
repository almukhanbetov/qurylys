package categories

import "context"

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
