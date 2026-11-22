package router

// End-to-end tests: HTTP request -> router -> handler -> service -> repository -> PostgreSQL.
// They need TEST_DATABASE_URL pointing at a disposable database whose name ends
// in "_test" (see `make test`). The schema is rebuilt from the migrations and
// the demo seed is loaded before the tests run. Skipped when the variable is unset.

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/qurylys/qurylys/backend/internal/config"
	"github.com/qurylys/qurylys/backend/internal/seed"
	"github.com/qurylys/qurylys/backend/migrations"
)

var (
	testDB     *sql.DB
	testRouter *gin.Engine
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		os.Exit(m.Run()) // integration tests call requireDB and skip
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil {
		panic(err)
	}
	if !strings.HasSuffix(name, "_test") {
		panic("refusing to reset database " + name + ": TEST_DATABASE_URL must point at a *_test database")
	}

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	must(goose.SetDialect("postgres"))
	// Start from an empty schema, then exercise every Up and Down section:
	// up -> reset (all downs) -> up.
	_, err = db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	must(err)
	must(goose.UpContext(ctx, db, "."))
	must(goose.ResetContext(ctx, db, "."))
	must(goose.UpContext(ctx, db, "."))
	must(seed.LoadDemo(ctx, db))
	// Seed twice: the second run must not fail or duplicate rows.
	must(seed.LoadDemo(ctx, db))

	gin.SetMode(gin.TestMode)
	testDB = db
	testRouter = New(config.Config{AppEnv: config.EnvTest}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func requireDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
}

// --- response shapes used by the tests ---

type node struct {
	Slug         string `json:"slug"`
	Depth        int    `json:"depth"`
	ProductCount int    `json:"product_count"`
	Children     []node `json:"children"`
}

type item struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	MinPrice *struct {
		AmountTiyn int64  `json:"amount_tiyn"`
		Currency   string `json:"currency"`
	} `json:"min_price"`
	OfferCount int `json:"offer_count"`
}

type page struct {
	Items      []item `json:"items"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}

type offer struct {
	Supplier struct {
		Slug string `json:"slug"`
	} `json:"supplier"`
	Price struct {
		AmountTiyn int64 `json:"amount_tiyn"`
	} `json:"price"`
	StockQuantity    string `json:"stock_quantity"`
	MinOrderQuantity string `json:"min_order_quantity"`
	IsAvailable      bool   `json:"is_available"`
}

func get(t *testing.T, path string, wantStatus int, out any) {
	t.Helper()
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != wantStatus {
		t.Fatalf("GET %s: status %d, want %d; body: %s", path, w.Code, wantStatus, w.Body.String())
	}
	if out == nil {
		return
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("GET %s: decode: %v", path, err)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("GET %s: decode data: %v", path, err)
	}
}

func listProducts(t *testing.T, params url.Values) page {
	t.Helper()
	var p page
	get(t, "/api/v1/products?"+params.Encode(), http.StatusOK, &p)
	return p
}

func itemSlugs(items []item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Slug
	}
	return out
}

func productID(t *testing.T, slug string) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow("SELECT id FROM products WHERE slug = $1", slug).Scan(&id); err != nil {
		t.Fatalf("product %s: %v", slug, err)
	}
	return id
}

func find(nodes []node, slug string) *node {
	for i := range nodes {
		if nodes[i].Slug == slug {
			return &nodes[i]
		}
		if n := find(nodes[i].Children, slug); n != nil {
			return n
		}
	}
	return nil
}

// --- tests ---

func TestHealthAndReady(t *testing.T) {
	requireDB(t)
	get(t, "/api/v1/health", http.StatusOK, nil)
	get(t, "/api/v1/ready", http.StatusOK, nil)
}

func TestCategoryTree(t *testing.T) {
	requireDB(t)
	var tree []node
	get(t, "/api/v1/categories/tree", http.StatusOK, &tree)

	if len(tree) != 6 {
		t.Fatalf("roots: got %d, want 6", len(tree))
	}
	if tree[0].Slug != "cement-dry-mixes" || tree[5].Slug != "drywall" {
		t.Errorf("roots must follow sort_order: got first=%s last=%s", tree[0].Slug, tree[5].Slug)
	}

	walls := find(tree, "wall-materials")
	aerated := find(tree, "aerated-blocks")
	if walls == nil || aerated == nil {
		t.Fatal("wall-materials or aerated-blocks missing")
	}
	if aerated.Depth != 2 {
		t.Errorf("aerated-blocks depth: got %d, want 2", aerated.Depth)
	}
	// 2 bricks + 2 aerated blocks (two levels down).
	if walls.ProductCount != 4 {
		t.Errorf("wall-materials product_count: got %d, want 4 including nested subcategories", walls.ProductCount)
	}
	if find(tree, "blocks").ProductCount != 2 {
		t.Errorf("blocks product_count must include aerated-blocks")
	}
}

func TestInactiveCategoryHidesSubtree(t *testing.T) {
	requireDB(t)
	if _, err := testDB.Exec("UPDATE categories SET is_active = FALSE WHERE slug = 'blocks'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testDB.Exec("UPDATE categories SET is_active = TRUE WHERE slug = 'blocks'") })

	var tree []node
	get(t, "/api/v1/categories/tree", http.StatusOK, &tree)
	if find(tree, "blocks") != nil || find(tree, "aerated-blocks") != nil {
		t.Error("inactive category and its descendants must not appear in the tree")
	}
	if got := find(tree, "wall-materials").ProductCount; got != 2 {
		t.Errorf("wall-materials product_count: got %d, want 2 (bricks only)", got)
	}

	p := listProducts(t, url.Values{"category": {"wall-materials"}})
	if p.Total != 2 {
		t.Errorf("wall-materials products with inactive blocks: got %v", itemSlugs(p.Items))
	}
	get(t, "/api/v1/products?category=blocks", http.StatusNotFound, nil)
}

func TestFilterByCategoryIncludesDescendants(t *testing.T) {
	requireDB(t)
	cases := map[string]int{
		"wall-materials":   4, // bricks(2) + blocks -> aerated-blocks(2)
		"blocks":           2, // no own products, only the nested level
		"aerated-blocks":   2,
		"cement-dry-mixes": 5,
		"cement":           2,
		"metal":            3,
	}
	for slug, want := range cases {
		t.Run(slug, func(t *testing.T) {
			p := listProducts(t, url.Values{"category": {slug}, "page_size": {"48"}})
			if p.Total != want || len(p.Items) != want {
				t.Errorf("total=%d items=%d, want %d: %v", p.Total, len(p.Items), want, itemSlugs(p.Items))
			}
		})
	}
	get(t, "/api/v1/products?category=does-not-exist", http.StatusNotFound, nil)
}

func TestSearch(t *testing.T) {
	requireDB(t)
	cases := []struct {
		q    string
		want []string
	}{
		// Case-insensitive Kazakh (uppercase Kazakh-specific letters).
		{"КІРПІШ", []string{"ceramic-brick-red", "silicate-brick"}},
		// Lowercase query against capitalised stored name.
		{"кірпіш", []string{"ceramic-brick-red", "silicate-brick"}},
		// Uppercase Қ, Ұ, Ғ, І against lowercase stored text.
		{"ҚҰБЫР", []string{"profile-pipe-40x20"}},
		{"МАҚТА", []string{"mineral-wool-50mm"}},
		{"ПАРАҒЫ", []string{"drywall-12-5mm"}},
		{"ЖЕЛІМІ", []string{"tile-adhesive-25kg"}},
		// Russian name, lowercase and mixed case.
		{"кирпич", []string{"ceramic-brick-red", "silicate-brick"}},
		{"МинеральнАЯ ВАТА", []string{"mineral-wool-50mm"}},
		// All terms must match, in any order.
		{"d600 газоблок", []string{"aerated-block-d600"}},
		{"Портландцемент М500", []string{"portland-cement-m500-50kg"}},
		// LIKE wildcards in input are literal characters, not patterns.
		{"%", nil},
		{"_", nil},
		{"несуществующий товар", nil},
	}
	for _, tc := range cases {
		t.Run(tc.q, func(t *testing.T) {
			p := listProducts(t, url.Values{"q": {tc.q}, "sort": {"price_asc"}})
			got := itemSlugs(p.Items)
			if p.Total != len(tc.want) {
				t.Fatalf("q=%q: got %v, want %v", tc.q, got, tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(strings.Join(got, ","), w) {
					t.Errorf("q=%q: %s missing from %v", tc.q, w, got)
				}
			}
		})
	}

	// Search combined with category filter.
	p := listProducts(t, url.Values{"q": {"газоблок"}, "category": {"metal"}})
	if p.Total != 0 {
		t.Errorf("search+category: got %v, want none", itemSlugs(p.Items))
	}
}

// TestDatabaseCaseFolding guards the database locale: search relies on
// PostgreSQL lower()/ILIKE folding Kazakh and Russian letters. A database with
// LC_CTYPE=C (or musl's C locale) does not fold Cyrillic and fails here first,
// with a clearer message than the search tests.
func TestDatabaseCaseFolding(t *testing.T) {
	requireDB(t)
	const upper = "ӘҒҚҢӨҰҮҺІАБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ"
	const lower = "әғқңөұүһіабвгдеёжзийклмнопрстуфхцчшщъыьэюя"

	var ctype, provider string
	if err := testDB.QueryRow(
		"SELECT datctype, datlocprovider::text FROM pg_database WHERE datname = current_database()",
	).Scan(&ctype, &provider); err != nil {
		t.Fatal(err)
	}

	ur, lr := []rune(upper), []rune(lower)
	for i := range ur {
		u, l := string(ur[i]), string(lr[i])
		var gotLower, gotUpper string
		var ilike bool
		if err := testDB.QueryRow("SELECT lower($1), upper($2), $1 ILIKE $2", u, l).
			Scan(&gotLower, &gotUpper, &ilike); err != nil {
			t.Fatal(err)
		}
		if gotLower != l || gotUpper != u || !ilike {
			t.Errorf("%s/%s: lower=%q upper=%q ilike=%v (datlocprovider=%s, datctype=%s)",
				u, l, gotLower, gotUpper, ilike, provider, ctype)
		}
	}
}

func TestMinPriceRule(t *testing.T) {
	requireDB(t)
	var d item
	get(t, "/api/v1/products/"+itoa(productID(t, "portland-cement-m400-50kg")), http.StatusOK, &d)

	// Offers: 2750 ₸ (in stock), 2890 ₸ (in stock), 2690 ₸ (stock 0 < MOQ).
	if d.MinPrice == nil || d.MinPrice.AmountTiyn != 275000 || d.MinPrice.Currency != "KZT" {
		t.Errorf("min_price: got %+v, want 275000 tiyn KZT (out-of-stock offer excluded)", d.MinPrice)
	}
	if d.OfferCount != 2 {
		t.Errorf("offer_count: got %d, want 2 available offers", d.OfferCount)
	}

	// Suspending a supplier removes its offers from the min price.
	if _, err := testDB.Exec("UPDATE companies SET status = 'suspended' WHERE slug = 'demo-supplier-1'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testDB.Exec("UPDATE companies SET status = 'active' WHERE slug = 'demo-supplier-1'") })
	get(t, "/api/v1/products/"+itoa(productID(t, "portland-cement-m400-50kg")), http.StatusOK, &d)
	if d.MinPrice == nil || d.MinPrice.AmountTiyn != 289000 || d.OfferCount != 1 {
		t.Errorf("after suspending supplier 1: got %+v count=%d, want 289000 / 1", d.MinPrice, d.OfferCount)
	}

	var none item
	get(t, "/api/v1/products/"+itoa(productID(t, "rebar-a500c-10mm")), http.StatusOK, &none)
	if none.MinPrice != nil || none.OfferCount != 0 {
		t.Errorf("product without offers: got %+v / %d, want null / 0", none.MinPrice, none.OfferCount)
	}
}

func TestSortByPrice(t *testing.T) {
	requireDB(t)
	for _, sort := range []string{"price_asc", "price_desc"} {
		t.Run(sort, func(t *testing.T) {
			p := listProducts(t, url.Values{"sort": {sort}, "page_size": {"48"}})
			if p.Total != 17 {
				t.Fatalf("total: got %d, want 17", p.Total)
			}
			last := p.Items[len(p.Items)-1]
			if last.Slug != "rebar-a500c-10mm" || last.MinPrice != nil {
				t.Errorf("product without price must be last for %s, got %s", sort, last.Slug)
			}
			priced := p.Items[:len(p.Items)-1]
			for i := 1; i < len(priced); i++ {
				a, b := priced[i-1].MinPrice.AmountTiyn, priced[i].MinPrice.AmountTiyn
				if (sort == "price_asc" && a > b) || (sort == "price_desc" && a < b) {
					t.Fatalf("%s order broken at %d: %d then %d", sort, i, a, b)
				}
			}
		})
	}
}

func TestPagination(t *testing.T) {
	requireDB(t)
	seen := map[string]bool{}
	for pg := 1; pg <= 4; pg++ {
		p := listProducts(t, url.Values{"page": {itoa(int64(pg))}, "page_size": {"5"}, "sort": {"price_asc"}})
		if p.Total != 17 || p.TotalPages != 4 || p.Page != pg {
			t.Fatalf("page %d: total=%d pages=%d page=%d", pg, p.Total, p.TotalPages, p.Page)
		}
		wantLen := 5
		if pg == 4 {
			wantLen = 2
		}
		if len(p.Items) != wantLen {
			t.Errorf("page %d: %d items, want %d", pg, len(p.Items), wantLen)
		}
		for _, it := range p.Items {
			if seen[it.Slug] {
				t.Errorf("%s appears on two pages", it.Slug)
			}
			seen[it.Slug] = true
		}
	}
	if len(seen) != 17 {
		t.Errorf("pages covered %d products, want 17", len(seen))
	}
	if p := listProducts(t, url.Values{"page": {"99"}}); len(p.Items) != 0 || p.Total != 17 {
		t.Errorf("page beyond the end: got %d items total %d", len(p.Items), p.Total)
	}
}

func TestInvalidParams(t *testing.T) {
	requireDB(t)
	for _, q := range []string{
		"sort=price_asc%3BDROP%20TABLE%20products", "sort=name", "page=0", "page_size=1000", "category=%27--",
	} {
		get(t, "/api/v1/products?"+q, http.StatusBadRequest, nil)
	}
	get(t, "/api/v1/products/abc", http.StatusBadRequest, nil)
	get(t, "/api/v1/products/0", http.StatusBadRequest, nil)
	get(t, "/api/v1/products/999999", http.StatusNotFound, nil)
}

func TestOffersForProduct(t *testing.T) {
	requireDB(t)
	var list []offer
	get(t, "/api/v1/products/"+itoa(productID(t, "portland-cement-m400-50kg"))+"/offers", http.StatusOK, &list)

	if len(list) != 3 {
		t.Fatalf("offers: got %d, want 3 from different suppliers", len(list))
	}
	// Available first by price, out-of-stock last even though it is cheapest.
	want := []struct {
		supplier string
		price    int64
		avail    bool
	}{
		{"demo-supplier-1", 275000, true},
		{"demo-supplier-2", 289000, true},
		{"demo-supplier-3", 269000, false},
	}
	for i, w := range want {
		o := list[i]
		if o.Supplier.Slug != w.supplier || o.Price.AmountTiyn != w.price || o.IsAvailable != w.avail {
			t.Errorf("offer %d: got %s %d %v, want %s %d %v", i, o.Supplier.Slug, o.Price.AmountTiyn, o.IsAvailable, w.supplier, w.price, w.avail)
		}
	}

	var frac []offer
	get(t, "/api/v1/products/"+itoa(productID(t, "rebar-a500c-12mm"))+"/offers", http.StatusOK, &frac)
	if frac[0].StockQuantity != "12.75" || frac[0].MinOrderQuantity != "0.5" {
		t.Errorf("fractional quantities: got %q / %q, want exact decimals 12.75 / 0.5", frac[0].StockQuantity, frac[0].MinOrderQuantity)
	}

	var empty []offer
	get(t, "/api/v1/products/"+itoa(productID(t, "rebar-a500c-10mm"))+"/offers", http.StatusOK, &empty)
	if empty == nil || len(empty) != 0 {
		t.Errorf("product without offers: want empty array, got %v", empty)
	}
	get(t, "/api/v1/products/999999/offers", http.StatusNotFound, nil)
}

func TestDatabaseConstraints(t *testing.T) {
	requireDB(t)
	cases := map[string]string{
		"offer from a buyer-only company": `
			WITH b AS (INSERT INTO companies (slug, name, is_buyer) VALUES ('buyer-only', 'Buyer', TRUE) RETURNING id)
			INSERT INTO offers (product_id, supplier_company_id, price_tiyn)
			SELECT (SELECT id FROM products LIMIT 1), b.id, 100 FROM b`,
		"non-positive price": `UPDATE offers SET price_tiyn = 0 WHERE id = (SELECT MIN(id) FROM offers)`,
		"duplicate offer for supplier": `
			INSERT INTO offers (product_id, supplier_company_id, price_tiyn)
			SELECT product_id, supplier_company_id, 100 FROM offers LIMIT 1`,
		"company without role": `INSERT INTO companies (slug, name) VALUES ('no-role', 'X')`,
		"category self parent": `UPDATE categories SET parent_id = id WHERE slug = 'metal'`,
		"category cycle":       `UPDATE categories SET parent_id = (SELECT id FROM categories WHERE slug = 'aerated-blocks') WHERE slug = 'wall-materials'`,
		"unknown unit":         `UPDATE products SET unit = 'bucket' WHERE slug = 'silicate-brick'`,
		"delete used category": `DELETE FROM categories WHERE slug = 'cement'`,
		"invalid BIN":          `UPDATE companies SET bin = '123' WHERE slug = 'demo-supplier-1'`,
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			tx, err := testDB.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback() //nolint:errcheck
			if _, err := tx.Exec(stmt); err == nil {
				t.Errorf("statement succeeded, want constraint violation")
			}
		})
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// TestMigrationLock checks that `migrate` serialises concurrent runs: while
// another session holds Goose's advisory lock, Up must fail instead of applying
// migrations in parallel, and succeed once the lock is released. It runs last
// and leaves the schema fully migrated.
func TestMigrationLock(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	// Goose skips locking when nothing is pending, so roll back the last
	// migration to give Up real work. It is re-applied at the end of the test.
	must(goose.DownContext(ctx, testDB, "."))

	holder, err := testDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatal(err)
	}

	p, err := migrations.NewProvider(testDB, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err == nil {
		t.Fatal("Up succeeded while another session held the migration lock")
	}

	if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lock.DefaultLockID); err != nil {
		t.Fatal(err)
	}
	res, err := p.Up(ctx)
	if err != nil {
		t.Fatalf("Up after unlock: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected exactly the rolled-back migration to be applied, applied %d", len(res))
	}
}
