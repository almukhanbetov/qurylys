package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// rawGet returns status, Cache-Control and body of an anonymous GET.
func rawGet(t *testing.T, path string) (int, string, string) {
	t.Helper()
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil))
	return w.Code, w.Header().Get("Cache-Control"), w.Body.String()
}

type pubBrandPage struct {
	Items []struct {
		ID       int64
		Name     string
		Products int `json:"product_count"`
	}
	Total int
}

func TestPublicBrands(t *testing.T) {
	hideRealCards(t)
	var brand, empty int64
	must(testDB.QueryRow(`INSERT INTO brands (name) VALUES ('Публичный Бренд') RETURNING id`).Scan(&brand))
	must(testDB.QueryRow(`INSERT INTO brands (name) VALUES ('Бренд Без Товаров') RETURNING id`).Scan(&empty))
	shown, blocked, unlinked := realCard(t, "piece"), realCard(t, "piece"), realCard(t, "piece")
	_, err := testDB.Exec(`UPDATE products SET brand_id = $1 WHERE id IN ($2, $3)`, brand, shown, blocked)
	must(err)
	_, err = testDB.Exec(`UPDATE products SET blocked_at = now(), block_reason = 'тест', blocked_by = 1 WHERE id = $1`, blocked)
	must(err)
	// The same text without a link is not merged into the brand.
	_, err = testDB.Exec(`UPDATE products SET brand = 'Публичный Бренд' WHERE id = $1`, unlinked)
	must(err)

	var pg pubBrandPage
	get(t, "/api/v1/brands?q="+url.QueryEscape("публичный"), 200, &pg)
	if pg.Total != 1 || pg.Items[0].ID != brand || pg.Items[0].Products != 1 {
		t.Fatalf("brands: %+v", pg)
	}
	get(t, "/api/v1/brands?q="+url.QueryEscape("без товаров"), 200, &pg)
	if pg.Total != 0 {
		t.Fatal("a brand without public cards must not be listed")
	}
	get(t, fmt.Sprintf("/api/v1/brands/%d", empty), 404, nil)
	get(t, "/api/v1/brands?page=0", 400, nil)
	_, _, body := rawGet(t, fmt.Sprintf("/brands/%d", brand))
	if strings.Contains(body, "version") || strings.Contains(body, "unlinked") {
		t.Fatalf("public brand leaks admin fields: %s", body)
	}

	// Catalogue filter: only the linked, public card.
	var products page
	get(t, fmt.Sprintf("/api/v1/products?brand=%d", brand), 200, &products)
	if products.Total != 1 || products.Items[0].ID != shown {
		t.Fatalf("brand filter: %+v", products)
	}
	get(t, "/api/v1/products?brand=abc", 400, nil)
	var detail struct {
		BrandID *int64 `json:"brand_id"`
		Brand   *string
	}
	get(t, fmt.Sprintf("/api/v1/products/%d", shown), 200, &detail)
	if detail.BrandID == nil || *detail.BrandID != brand {
		t.Fatalf("product brand link: %+v", detail)
	}
	get(t, fmt.Sprintf("/api/v1/products/%d", unlinked), 200, &detail)
	if detail.BrandID != nil {
		t.Fatal("an unlinked text brand has no brand page")
	}

	// Hiding the last public card hides the brand.
	_, err = testDB.Exec(`UPDATE products SET is_active = FALSE WHERE id = $1`, shown)
	must(err)
	get(t, fmt.Sprintf("/api/v1/brands/%d", brand), 404, nil)
}

// publicSupplier makes a verified supplier with the given name and city.
func publicSupplier(t *testing.T, name, city string) (*client, int64) {
	owner, _ := registered(t)
	var x struct{ ID int64 }
	owner.do("POST", "/companies", with(company(), "name", name, "city", city), 201, "", &x)
	owner.do("POST", ver(x.ID), nil, 204, "", nil)
	owner.upload(ver(x.ID)+"/documents?kind=registration", "талон.pdf", pdfFile, 204, "")
	owner.do("POST", ver(x.ID)+"/submit", nil, 204, "", nil)
	newMRX(t).decide(requestID(t, x.ID), 1, "approve", "", 204, "")
	return owner, x.ID
}

type pubSupplierPage struct {
	Items []struct {
		ID       int64
		Name     string
		City     *string
		Verified bool
	}
	Total  int
	Cities []string
}

func TestPublicSuppliers(t *testing.T) {
	owner, id := publicSupplier(t, "ТОО Публичный Поставщик", "Шымкент")
	_, hidden := publicSupplier(t, "ТОО Публичный Покупатель", "Шымкент")
	_, err := testDB.Exec(`UPDATE companies SET is_supplier = FALSE WHERE id = $1`, hidden)
	must(err)
	unverified, _ := registered(t)
	draft := unverified.createCompany()

	var pg pubSupplierPage
	get(t, "/api/v1/suppliers?q="+url.QueryEscape("публичный"), 200, &pg)
	if pg.Total != 1 || pg.Items[0].ID != id || !pg.Items[0].Verified || *pg.Items[0].City != "Шымкент" {
		t.Fatalf("suppliers: %+v", pg)
	}
	get(t, "/api/v1/suppliers?city="+url.QueryEscape("Шымкент"), 200, &pg)
	if pg.Total != 1 || !strings.Contains(strings.Join(pg.Cities, ","), "Шымкент") {
		t.Fatalf("city filter: %+v", pg)
	}
	get(t, "/api/v1/suppliers?city="+url.QueryEscape("Нигде"), 200, &pg)
	if pg.Total != 0 {
		t.Fatal("city filter must be exact")
	}
	// Demo, unverified and buyer-only companies are not public.
	var demo int64
	must(testDB.QueryRow(`SELECT min(id) FROM companies WHERE is_demo`).Scan(&demo))
	for _, x := range []int64{demo, draft, hidden, 999999999} {
		get(t, fmt.Sprintf("/api/v1/suppliers/%d", x), 404, nil)
	}
	get(t, "/api/v1/suppliers", 200, &pg)
	for _, it := range pg.Items {
		if it.ID == demo || it.ID == draft || it.ID == hidden {
			t.Fatalf("not a public supplier: %d", it.ID)
		}
	}

	// Only the agreed fields; never cached.
	for _, path := range []string{"/suppliers", fmt.Sprintf("/suppliers/%d", id)} {
		code, cache, body := rawGet(t, path)
		if code != 200 || cache != "no-store" {
			t.Fatalf("%s: %d %q", path, code, cache)
		}
		for _, leak := range []string{"bin", "address", "phone", "representative", "email", "@example.kz", "document", "comment", "member", "price", "verification_", "submission"} {
			if strings.Contains(strings.ToLower(body), leak) {
				t.Fatalf("%s leaks %q: %s", path, leak, body)
			}
		}
	}

	// Losing the verification removes the company at once.
	d := details(owner, id)
	d["legal_address"] = "ул. Новая, 1"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	get(t, fmt.Sprintf("/api/v1/suppliers/%d", id), 404, nil)
	get(t, "/api/v1/suppliers?q="+url.QueryEscape("публичный"), 200, &pg)
	if pg.Total != 0 {
		t.Fatal("an unverified company must disappear from the list")
	}
}
