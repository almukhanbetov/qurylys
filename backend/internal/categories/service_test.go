package categories

import "testing"

func ptr(v int64) *int64 { return &v }

func TestBuildTree(t *testing.T) {
	flat := []Category{
		{ID: 1, Slug: "walls", DirectProductCount: 1},
		{ID: 2, Slug: "metal", DirectProductCount: 0},
		{ID: 3, ParentID: ptr(1), Slug: "bricks", DirectProductCount: 2},
		{ID: 4, ParentID: ptr(1), Slug: "blocks", DirectProductCount: 0},
		{ID: 5, ParentID: ptr(4), Slug: "aerated", DirectProductCount: 3},
		// Parent 99 is not in the list (inactive): this subtree must disappear.
		{ID: 6, ParentID: ptr(99), Slug: "orphan", DirectProductCount: 7},
		{ID: 7, ParentID: ptr(6), Slug: "orphan-child", DirectProductCount: 1},
	}

	roots := BuildTree(flat)

	if len(roots) != 2 || roots[0].Slug != "walls" || roots[1].Slug != "metal" {
		t.Fatalf("roots: got %v, want [walls metal] in input order", slugs(roots))
	}
	walls := roots[0]
	if got := slugs(walls.Children); len(got) != 2 || got[0] != "bricks" || got[1] != "blocks" {
		t.Fatalf("walls children: got %v, want [bricks blocks]", got)
	}
	aerated := walls.Children[1].Children[0]
	if aerated.Slug != "aerated" || aerated.Depth != 2 {
		t.Fatalf("aerated: got slug=%s depth=%d, want aerated depth 2", aerated.Slug, aerated.Depth)
	}
	// 1 own + 2 bricks + 0 blocks + 3 aerated.
	if walls.ProductCount != 6 {
		t.Errorf("walls product count: got %d, want 6 (rolled up from descendants)", walls.ProductCount)
	}
	if walls.Children[1].ProductCount != 3 {
		t.Errorf("blocks product count: got %d, want 3", walls.Children[1].ProductCount)
	}
	if roots[1].Children == nil {
		t.Error("leaf children must be an empty slice so JSON renders [] not null")
	}
}

func TestBuildTreeEmpty(t *testing.T) {
	if got := BuildTree(nil); got == nil || len(got) != 0 {
		t.Fatalf("got %v, want empty non-nil slice", got)
	}
}

func slugs(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Slug
	}
	return out
}
