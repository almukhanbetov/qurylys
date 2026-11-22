package products

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseListParamsDefaults(t *testing.T) {
	f, err := ParseListParams(RawListParams{})
	if err != nil {
		t.Fatal(err)
	}
	want := ListFilter{Sort: SortNewest, Page: 1, PageSize: DefaultPageSize}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("got %+v, want %+v", f, want)
	}
}

func TestParseListParamsValid(t *testing.T) {
	f, err := ParseListParams(RawListParams{
		Q: "  Цемент   м500 цемент ", Category: "cement-dry-mixes", Sort: "price_desc", Page: "3", PageSize: "48",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Terms, []string{"Цемент", "м500"}) {
		t.Errorf("terms: got %q, want duplicates removed case-insensitively", f.Terms)
	}
	if f.CategorySlug != "cement-dry-mixes" || f.Sort != SortPriceDesc || f.Page != 3 || f.PageSize != 48 {
		t.Errorf("got %+v", f)
	}
}

func TestParseListParamsRejects(t *testing.T) {
	cases := map[string]RawListParams{
		"sql in sort":       {Sort: "price_asc; DROP TABLE products"},
		"unknown sort":      {Sort: "name"},
		"column name sort":  {Sort: "p.id"},
		"page zero":         {Page: "0"},
		"page not number":   {Page: "1abc"},
		"page size too big": {PageSize: "49"},
		"negative size":     {PageSize: "-1"},
		"slug with quote":   {Category: "cement'--"},
		"slug upper":        {Category: "Cement"},
		"slug edge dash":    {Category: "-cement"},
		"query too long":    {Q: strings.Repeat("ә", MaxQueryLength+1)},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseListParams(raw)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("got %v, want ValidationError", err)
			}
		})
	}
}

func TestSplitTermsLimit(t *testing.T) {
	got := splitTerms("a b c d e f g")
	if len(got) != MaxSearchTerms {
		t.Fatalf("got %d terms, want %d", len(got), MaxSearchTerms)
	}
}

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"100%":     `100\%`,
		"a_b":      `a\_b`,
		`back\sl`:  `back\\sl`,
		"газоблок": "газоблок",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildWhereIsParameterised(t *testing.T) {
	f := ListFilter{Terms: []string{"x'; DROP TABLE products; --"}, CategorySlug: "cement"}
	cte, where, b := buildWhere(f)
	sql := cte + where
	if strings.Contains(sql, "DROP") || strings.Contains(sql, "cement") {
		t.Fatalf("user input leaked into SQL text: %s", sql)
	}
	if len(b.args) != 2 {
		t.Fatalf("got %d args, want 2", len(b.args))
	}
	if b.args[1] != `%x'; DROP TABLE products; --%` {
		t.Errorf("search arg: got %q", b.args[1])
	}
}

func TestEverySortHasClause(t *testing.T) {
	for _, s := range []Sort{SortNewest, SortPriceAsc, SortPriceDesc, SortOffers} {
		if _, ok := sortClauses[s]; !ok {
			t.Errorf("sort %q has no ORDER BY clause", s)
		}
	}
}
