package categories

// Category is a flat row as stored in PostgreSQL.
type Category struct {
	ID        int64
	ParentID  *int64
	Slug      string
	NameKK    string
	NameRU    string
	SortOrder int
	// DirectProductCount counts active products assigned to this exact category.
	DirectProductCount int
}

// Node is a category with its children, as returned by GET /categories/tree.
type Node struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Slug     string `json:"slug"`
	NameKK   string `json:"name_kk"`
	NameRU   string `json:"name_ru"`
	Depth    int    `json:"depth"`
	// ProductCount includes products of all descendant categories.
	ProductCount int     `json:"product_count"`
	Children     []*Node `json:"children"`
}
