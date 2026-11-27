package router

// Stage 3, step 3.3: the supplier's offers cabinet.

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/qurylys/qurylys/backend/internal/quantity"
	"github.com/qurylys/qurylys/backend/migrations"
)

// realCard inserts an approved card with a test GTIN and removes it afterwards.
func realCard(t *testing.T, unit string) (id int64) {
	t.Helper()
	requireDB(t)
	must(testDB.QueryRow(`INSERT INTO products (category_id, slug, name_kk, name_ru, unit, gtin, gtin_source_kind, gtin_source, gtin_confirmed_at, gtin_confirmed_by, needs_review)
		SELECT min(id), 't-' || $1, 'Тест карточка', 'Тестовая карточка', $2, '0' || $1, 'packaging', 'ТЕСТ', now(), 1, FALSE FROM categories WHERE is_active RETURNING id`,
		newGTIN(), unit).Scan(&id))
	t.Cleanup(func() {
		_, _ = testDB.Exec(`DELETE FROM offers WHERE product_id = $1`, id)
		_, _ = testDB.Exec(`DELETE FROM products WHERE id = $1`, id)
	})
	return id
}

func so(company int64) string { return fmt.Sprintf("/companies/%d/offers", company) }

func lvl(price any, mode, min, step string) m {
	return m{"price_tiyn": price, "sale_mode": mode, "min_quantity": min, "step_quantity": step}
}

// foam is the reference example: 12 per box, 1500 retail, 1150 from 5 boxes, stock 120.
func foam(sku string) m {
	return m{"seller_sku": sku, "vat_status": "with_vat", "pack_size": "12", "stock_quantity": "120", "city": "Алматы",
		"retail": lvl(150000, "piece", "1", "1"), "wholesale": lvl(115000, "pack", "60", "12"), "large_wholesale": nil}
}

func with(f m, kv ...any) m {
	out := m{}
	for k, v := range f {
		out[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

type levelDTO struct {
	Price    *int64 `json:"price_tiyn"`
	Pack     *int64 `json:"pack_price_tiyn"`
	MinTotal *int64 `json:"min_total_tiyn"`
	Min      string `json:"min_quantity"`
	InStock  bool   `json:"in_stock"`
}

type sellerOfferDTO struct {
	ID         int64
	OwnerState string `json:"owner_state"`
	SKU        string `json:"seller_sku"`
	Stock      string `json:"stock_quantity"`
	Version    int
	Reasons    []string
	CanManage  bool `json:"can_manage"`
	Retail     levelDTO
	Wholesale  *levelDTO
	Card       struct {
		ID   int64
		Unit string
	}
}

func verifiedSupplier(t *testing.T) (*client, int64) {
	owner, _, company := submitted(t)
	newMRX(t).decide(requestID(t, company), 1, "approve", "", 204, "")
	return owner, company
}

func (c *client) newOffer(company, card int64, f m) (id int64) {
	c.t.Helper()
	var x struct{ ID int64 }
	c.do("POST", so(company), with(f, "product_id", card), 201, "", &x)
	return x.ID
}

func (c *client) offer(company, id int64) (v sellerOfferDTO) {
	c.t.Helper()
	c.do("GET", fmt.Sprintf("%s/%d", so(company), id), nil, 200, "", &v)
	return
}

func (c *client) act(company, id int64, action string, version, status int, code string) {
	c.t.Helper()
	c.do("POST", fmt.Sprintf("%s/%d/%s", so(company), id, action), m{"version": version}, status, code, nil)
}

func TestSellerOfferReferenceExample(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card := realCard(t, "piece")
	id := owner.newOffer(company, card, foam("A-FOAM-750"))
	o := owner.offer(company, id)
	if o.OwnerState != "draft" || len(o.Reasons) != 0 || !o.CanManage || o.Card.Unit != "piece" || o.Version != 1 {
		t.Fatalf("%+v", o)
	}
	if *o.Retail.Pack != 1800000 || *o.Retail.MinTotal != 150000 || *o.Wholesale.Pack != 1380000 || *o.Wholesale.MinTotal != 6900000 || !o.Wholesale.InStock || !o.Retail.InStock {
		t.Fatalf("%+v %+v", o.Retail, o.Wholesale)
	}
	// Wholesale out of stock does not hide retail.
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id), with(foam("A-FOAM-750"), "stock_quantity", "59", "version", 1), 204, "", nil)
	o = owner.offer(company, id)
	if o.Wholesale.InStock || !o.Retail.InStock || o.Version != 2 {
		t.Fatalf("%+v", o)
	}
	owner.act(company, id, "publish", 1, 409, "offer_conflict") // stale tab
	owner.act(company, id, "hide", 2, 409, "offer_conflict")    // a draft cannot be hidden
	owner.act(company, id, "publish", 2, 204, "")
	owner.act(company, id, "hide", 3, 204, "")
	owner.act(company, id, "publish", 4, 204, "")
	if o = owner.offer(company, id); o.OwnerState != "published" || o.Version != 5 {
		t.Fatalf("%+v", o)
	}
	// A published offer stays complete.
	for _, f := range []m{with(foam(""), "version", 5), with(foam("A-FOAM-750"), "vat_status", "", "version", 5),
		with(foam("A-FOAM-750"), "retail", lvl(nil, "piece", "1", "1"), "version", 5)} {
		owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id), f, 409, "published_offer_incomplete", nil)
	}
	var events, audits, leaked int
	must(testDB.QueryRow(`SELECT count(*) FROM offer_events WHERE offer_id = $1`, id).Scan(&events))
	must(testDB.QueryRow(`SELECT count(*), count(*) FILTER (WHERE details::text LIKE '%115000%') FROM audit_events WHERE company_id = $1 AND action LIKE 'offer.%'`, company).Scan(&audits, &leaked))
	if events != 5 || audits != 5 || leaked != 0 {
		t.Fatalf("events %d audits %d leaked %d", events, audits, leaked)
	}
	var old string
	must(testDB.QueryRow(`SELECT details -> 'old' ->> 'stock_quantity' FROM offer_events WHERE offer_id = $1 AND action = 'updated'`, id).Scan(&old))
	if old != "120" {
		t.Fatal(old)
	}
}

// Published by the owner is not public: nothing about the offer leaves the cabinet yet.
func TestPublishedOfferIsNotPublicYet(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card := realCard(t, "piece")
	f := with(foam("SECRET-SKU-77"), "retail", lvl(1234567, "piece", "1", "1"), "wholesale", lvl(7654321, "pack", "60", "12"))
	id := owner.newOffer(company, card, f)
	owner.act(company, id, "publish", 1, 204, "")
	for _, path := range []string{fmt.Sprintf("/products/%d", card), fmt.Sprintf("/products/%d/offers", card), "/products?page_size=48",
		"/products?q=SECRET-SKU-77", "/products?sort=offers", "/categories/tree"} {
		for _, c := range []*client{newClient(t), owner} {
			w := c.send(c.request("GET", path, nil))
			if w.Code != 200 {
				t.Fatalf("%s: %d", path, w.Code)
			}
			for _, secret := range []string{"1234567", "7654321", "SECRET-SKU-77", "wholesale"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Errorf("%s leaks %s", path, secret)
				}
			}
		}
	}
	var legacy int
	must(testDB.QueryRow(`SELECT count(*) FROM offers WHERE NOT is_demo AND is_active`).Scan(&legacy))
	if legacy != 0 {
		t.Fatal("a previous release would show a real offer")
	}
	w := owner.send(owner.request("GET", so(company), nil))
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cabinet answers must not be cached")
	}
}

func TestSellerOfferPermissions(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card := realCard(t, "piece")
	id := owner.newOffer(company, card, foam("P-1"))
	path := fmt.Sprintf("%s/%d", so(company), id)
	emp, mail := registered(t)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", company), m{"contact": mail}, 204, "", nil)
	var inv []struct{ ID int64 }
	emp.do("GET", "/me/invitations", nil, 200, "", &inv)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)

	// Employee: everything readable, nothing writable.
	o := emp.offer(company, id)
	if o.CanManage || o.Wholesale == nil || o.Stock != "120" {
		t.Fatalf("%+v", o)
	}
	var list struct {
		Items     []sellerOfferDTO
		CanCreate bool `json:"can_create"`
	}
	emp.do("GET", so(company), nil, 200, "", &list)
	if len(list.Items) != 1 || list.CanCreate {
		t.Fatalf("%+v", list)
	}
	writes := func(c *client, status int, code string) {
		t.Helper()
		c.do("POST", so(company), with(foam("X"), "product_id", card), status, code, nil)
		c.do("PUT", path, with(foam("X"), "version", 1), status, code, nil)
		c.act(company, id, "publish", 1, status, code)
		c.act(company, id, "hide", 1, status, code)
		c.do("PUT", so(company)+"/999999999", with(foam("X"), "version", 1), status, code, nil) // ids reveal nothing
	}
	writes(emp, 403, "forbidden")
	emp.do("GET", fmt.Sprintf("/companies/%d/offer-cards", company), nil, 403, "forbidden", nil)

	stranger, strangerCompany := verifiedSupplier(t)
	admin := newMRX(t)
	_, err := testDB.Exec(`UPDATE platform_roles SET role = 'admin' WHERE user_id = $1`, admin.me().ID)
	must(err)
	mrx, mod := newMRX(t), newModerator(t)
	for _, c := range []*client{stranger, admin, mod} {
		c.do("GET", so(company), nil, 404, "company_not_found", nil)
		c.do("GET", path, nil, 404, "company_not_found", nil)
		writes(c, 404, "company_not_found")
	}
	mrx.do("GET", so(company), nil, 404, "company_not_found", nil)
	writes(mrx, 403, "forbidden") // BR-13
	mrx.do("POST", "/companies/999999999/offers", with(foam("X"), "product_id", card), 403, "forbidden", nil)
	stranger.do("GET", fmt.Sprintf("%s/%d", so(strangerCompany), id), nil, 404, "not_found", nil)
	stranger.do("PUT", fmt.Sprintf("%s/%d", so(strangerCompany), id), with(foam("X"), "version", 1), 404, "not_found", nil)
	stranger.act(strangerCompany, id, "publish", 1, 404, "not_found")
	anon := newClient(t)
	anon.do("GET", so(company), nil, 401, "unauthorized", nil)
	writes(anon, 401, "unauthorized")
	// Rights belong to the selected company.
	second := owner.createCompany()
	owner.do("GET", so(company), nil, 409, "company_context_mismatch", nil)
	owner.do("PUT", "/me/active-company", m{"company_id": company}, 204, "", nil)
	_ = second
	// Rights are independent: an owner who is also a reviewer manages own offers.
	_, err = testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'mrx')`, owner.me().ID)
	must(err)
	owner.do("PUT", path, with(foam("P-1"), "version", 1), 204, "", nil)
	// A removed employee loses access at once.
	owner.do("DELETE", fmt.Sprintf("/companies/%d/members/%d", company, emp.me().ID), nil, 204, "", nil)
	emp.do("GET", path, nil, 404, "company_not_found", nil)
	// Smuggled fields.
	for _, k := range []string{"is_demo", "owner_state", "supplier_company_id", "is_active", "unit"} {
		owner.do("PUT", path, with(foam("P-1"), "version", 2, k, "x"), 400, "invalid_body", nil)
	}
	if v := owner.offer(company, id); v.Version != 2 || v.SKU != "P-1" {
		t.Fatalf("%+v", v)
	}
}

func TestSellerOfferValidation(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card := realCard(t, "piece")
	base := foam("V-1")
	bad := map[string]m{
		"invalid_wholesale_quantity":       with(base, "wholesale", lvl(115000, "pack", "50", "12")),
		"invalid_wholesale_quantity ":      with(base, "wholesale", lvl(115000, "pack", "60", "10")),
		"invalid_retail_quantity":          with(base, "retail", lvl(150000, "piece", "1.5", "1")),
		"invalid_retail_quantity ":         with(base, "retail", lvl(150000, "piece", "2", "2")),
		"invalid_retail_quantity  ":        with(base, "retail", lvl(150000, "piece", "0", "1")),
		"invalid_retail_quantity   ":       with(base, "retail", lvl(150000, "piece", "1e1", "1")),
		"invalid_retail_price":             with(base, "retail", lvl(0, "piece", "1", "1")),
		"invalid_retail_price ":            with(base, "retail", lvl(-5, "piece", "1", "1")),
		"invalid_retail_price  ":           with(base, "retail", lvl(9007199254740992, "piece", "1", "1")),
		"invalid_wholesale_price":          with(base, "wholesale", lvl(nil, "pack", "60", "12")),
		"invalid_wholesale_price ":         with(base, "wholesale", lvl(9007199254740991, "pack", "60", "12")),
		"invalid_large_wholesale_quantity": with(base, "large_wholesale", lvl(95000, "pack", "601", "12")),
		"invalid_large_wholesale_mode":     with(base, "large_wholesale", lvl(95000, "box", "600", "12")),
		"invalid_pack_size":                with(base, "pack_size", "1.5"),
		"invalid_pack_size ":               with(base, "pack_size", ""),
		"invalid_pack_size  ":              with(base, "pack_size", "0"),
		"invalid_stock_quantity":           with(base, "stock_quantity", "10.5"),
		"invalid_stock_quantity ":          with(base, "stock_quantity", "-1"),
		"invalid_vat_status":               with(base, "vat_status", "20%"),
		"invalid_seller_sku":               with(base, "seller_sku", strings.Repeat("x", 65)),
	}
	for code, f := range bad {
		owner.do("POST", so(company), with(f, "product_id", card), 400, strings.TrimSpace(code), nil)
	}
	// A price is an integer number of tiyn: fractions are not accepted at all.
	owner.do("POST", so(company), with(base, "product_id", card, "retail", lvl(1500.5, "piece", "1", "1")), 400, "invalid_body", nil)

	// Measured goods take exact decimals.
	kg, m3 := realCard(t, "kg"), realCard(t, "m3")
	a := owner.newOffer(company, kg, m{"seller_sku": "KG", "vat_status": "without_vat", "pack_size": "", "stock_quantity": "1500.5", "city": "",
		"retail": lvl(8437, "piece", "500", "0.5"), "wholesale": nil, "large_wholesale": nil})
	if o := owner.offer(company, a); *o.Retail.MinTotal != 4218500 || o.Stock != "1500.5" || o.Retail.Pack != nil {
		t.Fatalf("%+v", o)
	}
	b := owner.newOffer(company, m3, m{"seller_sku": "M3", "vat_status": "without_vat", "pack_size": "0.6", "stock_quantity": "6", "city": "",
		"retail": lvl(83333, "pack", "0.6", "0.6"), "wholesale": lvl(80000, "pack", "1.2", "0.6"), "large_wholesale": nil})
	if o := owner.offer(company, b); *o.Retail.Pack != 50000 || *o.Wholesale.MinTotal != 96000 { // 49 999.8 rounds half up
		t.Fatalf("%+v %+v", o.Retail, o.Wholesale)
	}
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), b), m{"version": 1, "seller_sku": "M3", "vat_status": "", "pack_size": "0.6", "stock_quantity": "6", "city": "",
		"retail": lvl(83333, "pack", "1", "0.6"), "wholesale": nil, "large_wholesale": nil}, 400, "invalid_retail_quantity", nil)
	// Changing the pack replaces the tiers in one step.
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), b), m{"version": 1, "seller_sku": "M3", "vat_status": "", "pack_size": "0.5", "stock_quantity": "6", "city": "",
		"retail": lvl(nil, "pack", "0.5", "0.5"), "wholesale": lvl(80000, "pack", "1.5", "0.5"), "large_wholesale": nil}, 204, "", nil)
}

func TestSellerOfferDuplicates(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card, card2 := realCard(t, "piece"), realCard(t, "piece")
	id := owner.newOffer(company, card, foam("Dup-1"))
	owner.do("POST", so(company), with(foam("Other"), "product_id", card), 409, "offer_exists", nil)
	owner.do("POST", so(company), with(foam("dUP-1"), "product_id", card2), 409, "seller_sku_taken", nil)
	id2 := owner.newOffer(company, card2, foam("")) // several drafts may lack an article
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id2), with(foam("DUP-1"), "version", 1), 409, "seller_sku_taken", nil)
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id), with(foam("Dup-1"), "version", 1), 204, "", nil) // own article
	other, otherCompany := verifiedSupplier(t)
	other.newOffer(otherCompany, card, foam("Dup-1")) // another company, same card and article
	var cards []struct {
		ID       int64
		HasOffer bool `json:"has_offer"`
	}
	owner.do("GET", fmt.Sprintf("/companies/%d/offer-cards?q=Тестовая", company), nil, 200, "", &cards)
	seen := 0
	for _, c := range cards {
		if c.ID == card || c.ID == card2 {
			seen++
			if !c.HasOffer {
				t.Fatalf("%+v", c)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("%+v", cards)
	}
	// Parallel creation makes one offer.
	card3 := realCard(t, "piece")
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for k := range codes {
		r := owner.request("POST", so(company), with(foam(fmt.Sprintf("PAR-%d", k)), "product_id", card3))
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, r)
			codes[k] = w.Code
		}()
	}
	wg.Wait()
	won := 0
	for _, c := range codes {
		if c == 201 {
			won++
		} else if c != 409 {
			t.Fatal(codes)
		}
	}
	if won != 1 {
		t.Fatal(codes)
	}
	// Parallel edits of one version: one wins.
	for k := range codes {
		r := owner.request("PUT", fmt.Sprintf("%s/%d", so(company), id), with(foam("Dup-1"), "version", 2, "stock_quantity", fmt.Sprint(k)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, r)
			codes[k] = w.Code
		}()
	}
	wg.Wait()
	won = 0
	for _, c := range codes {
		if c == 204 {
			won++
		}
	}
	if won != 1 {
		t.Fatal(codes)
	}
}

func TestPublicationConditions(t *testing.T) {
	// Unverified: drafts yes, publication no.
	owner, _ := registered(t)
	company := owner.createCompany()
	card := realCard(t, "piece")
	id := owner.newOffer(company, card, with(foam(""), "vat_status", "", "retail", lvl(nil, "piece", "1", "1")))
	o := owner.offer(company, id)
	if strings.Join(o.Reasons, ",") != "company_not_verified,price_missing,seller_sku_missing,vat_missing" {
		t.Fatal(o.Reasons)
	}
	owner.act(company, id, "publish", 1, 409, "offer_not_publishable")
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id), with(foam("C-1"), "version", 1), 204, "", nil)
	owner.act(company, id, "publish", 2, 409, "offer_not_publishable")
	owner.do("POST", ver(company), nil, 204, "", nil)
	owner.upload(ver(company)+"/documents?kind=registration", "t.pdf", pdfFile, 204, "")
	owner.do("POST", ver(company)+"/submit", nil, 204, "", nil)
	newMRX(t).decide(requestID(t, company), 1, "approve", "", 204, "")
	if o = owner.offer(company, id); len(o.Reasons) != 0 {
		t.Fatal(o.Reasons)
	}
	// Each condition on its own.
	for want, q := range map[string][2]string{
		"company_suspended":     {`UPDATE companies SET status = 'suspended' WHERE id = $1`, `UPDATE companies SET status = 'active' WHERE id = $1`},
		"card_inactive":         {`UPDATE products SET is_active = FALSE WHERE id = (SELECT product_id FROM offers WHERE supplier_company_id = $1)`, `UPDATE products SET is_active = TRUE WHERE id = (SELECT product_id FROM offers WHERE supplier_company_id = $1)`},
		"card_needs_review":     {`UPDATE products SET needs_review = TRUE WHERE id = (SELECT product_id FROM offers WHERE supplier_company_id = $1)`, `UPDATE products SET needs_review = FALSE WHERE id = (SELECT product_id FROM offers WHERE supplier_company_id = $1)`},
		"company_not_supplier":  {`UPDATE companies SET is_supplier = FALSE WHERE id = $1`, ""},
		"card_gtin_unconfirmed": {"", ""},
	} {
		if q[0] == "" {
			continue // the linked GTIN cannot be removed, see TestSellerOfferDatabaseGuards
		}
		if want == "company_not_supplier" {
			continue // last, below: turning it back on requires a new verification
		}
		_, err := testDB.Exec(q[0], company)
		must(err)
		if o = owner.offer(company, id); strings.Join(o.Reasons, ",") != want {
			t.Errorf("%s: %v", want, o.Reasons)
		}
		code, status := "offer_not_publishable", 409
		if want == "company_suspended" {
			code, status = "company_suspended", 403
		}
		owner.act(company, id, "publish", o.Version, status, code)
		_, err = testDB.Exec(q[1], company)
		must(err)
	}
	owner.act(company, id, "publish", 2, 204, "")
	// A key change drops the verification: the owner's choice stays, the reason appears.
	d := details(owner, company)
	d["legal_address"] = "новый"
	owner.do("PUT", fmt.Sprintf("/companies/%d", company), d, 204, "", nil)
	if o = owner.offer(company, id); o.OwnerState != "published" || strings.Join(o.Reasons, ",") != "company_not_verified" {
		t.Fatalf("%+v", o)
	}
	owner.act(company, id, "hide", 3, 204, "") // hiding is always possible
	owner.act(company, id, "publish", 4, 409, "offer_not_publishable")
	_, err := testDB.Exec(`UPDATE companies SET is_supplier = FALSE WHERE id = $1`, company)
	must(err)
	owner.do("PUT", fmt.Sprintf("%s/%d", so(company), id), with(foam("C-1"), "version", 4), 403, "not_supplier", nil)
	owner.act(company, id, "publish", 4, 403, "not_supplier")
	owner.do("POST", so(company), with(foam("C-2"), "product_id", realCard(t, "piece")), 403, "not_supplier", nil)
	// Cards that cannot be offered.
	s, sc := verifiedSupplier(t)
	var demo int64
	must(testDB.QueryRow(`SELECT min(id) FROM products WHERE is_demo`).Scan(&demo))
	unreviewed := realCard(t, "piece")
	_, err = testDB.Exec(`UPDATE products SET needs_review = TRUE WHERE id = $1`, unreviewed)
	must(err)
	for _, p := range []int64{demo, unreviewed, 999999999} {
		s.do("POST", so(sc), with(foam("Z"), "product_id", p), 409, "card_unavailable", nil)
	}
}

func TestSellerOfferDatabaseGuards(t *testing.T) {
	owner, company := verifiedSupplier(t)
	card := realCard(t, "piece")
	id := owner.newOffer(company, card, foam("G-1"))
	var demoProduct, demoCompany int64
	must(testDB.QueryRow(`SELECT min(id) FROM products WHERE is_demo`).Scan(&demoProduct))
	must(testDB.QueryRow(`SELECT min(id) FROM companies WHERE is_demo`).Scan(&demoCompany))
	for name, q := range map[string]string{
		"legacy public":          fmt.Sprintf(`UPDATE offers SET is_active = TRUE WHERE id = %d`, id),
		"become demo":            fmt.Sprintf(`UPDATE offers SET is_demo = TRUE WHERE id = %d`, id),
		"company becomes demo":   fmt.Sprintf(`UPDATE companies SET is_demo = TRUE WHERE id = %d`, company),
		"demo card becomes real": fmt.Sprintf(`UPDATE products SET is_demo = FALSE WHERE id = %d`, demoProduct),
		"real offer, demo card":  fmt.Sprintf(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_active) VALUES (%d, %d, 1, FALSE)`, demoProduct, company),
		"demo offer, real firm":  fmt.Sprintf(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_demo) VALUES (%d, %d, 1, TRUE)`, demoProduct, company),
		"real offer, demo firm":  fmt.Sprintf(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_active) VALUES (%d, %d, 1, FALSE)`, card, demoCompany),
		"publish without sku":    fmt.Sprintf(`UPDATE offers SET owner_state = 'published', seller_sku = NULL WHERE id = %d`, id),
		"min not multiple":       fmt.Sprintf(`UPDATE offer_price_tiers SET min_quantity = 50 WHERE offer_id = %d`, id),
		"pack breaks tiers":      fmt.Sprintf(`UPDATE offers SET pack_size = 7 WHERE id = %d`, id),
		"fractional stock":       fmt.Sprintf(`UPDATE offers SET stock_quantity = 1.5 WHERE id = %d`, id),
		"change base unit":       fmt.Sprintf(`UPDATE products SET unit = 'kg' WHERE id = %d`, card),
		"remove linked gtin":     fmt.Sprintf(`UPDATE products SET needs_review = TRUE, gtin = NULL, gtin_source_kind = NULL, gtin_source = NULL, gtin_confirmed_at = NULL, gtin_confirmed_by = NULL WHERE id = %d`, card),
		"move to another card":   fmt.Sprintf(`UPDATE offers SET product_id = %d WHERE id = %d`, realCard(t, "piece"), id),
		"same sku, other case":   fmt.Sprintf(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_active, seller_sku) VALUES (%d, %d, 1, FALSE, 'g-1')`, realCard(t, "piece"), company),
		"tier for a demo offer":  `INSERT INTO offer_price_tiers SELECT min(id), 'wholesale', 1, 'piece', 1, 1 FROM offers WHERE is_demo`,
		"edit history":           `UPDATE offer_events SET version = 0`,
		"delete history":         `DELETE FROM offer_events`,
	} {
		if _, err := testDB.Exec(q); err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
}

// Go and PostgreSQL must agree on money rounding and quantity rules.
func TestMoneyRuleInSQL(t *testing.T) {
	requireDB(t)
	parse := func(s string) quantity.Q { v, _ := quantity.Parse(s); return v }
	for _, c := range []struct {
		price int64
		qty   string
	}{{150000, "12"}, {115000, "60"}, {8437, "500"}, {83333, "0.6"}, {1, "0.5"}, {1, "0.499"}, {3, "0.5"}, {83333, "1.2"}} {
		want, _ := quantity.Total(c.price, parse(c.qty))
		var got int64
		must(testDB.QueryRow(`SELECT round($1::numeric * $2::numeric)::bigint`, c.price, c.qty).Scan(&got))
		if got != want {
			t.Errorf("%d × %s: sql %d go %d", c.price, c.qty, got, want)
		}
	}
	// A line is rounded once: 2 × 0.5 × 1 tiyn is 1, not 2.
	if v, _ := quantity.Total(1, 2*parse("0.5")); v != 1 {
		t.Error(v)
	}
	for _, unit := range []string{"piece", "kg", "m3", "pallet", "litre"} {
		for _, mode := range []string{"piece", "pack"} {
			for _, pack := range []string{"0", "12", "0.6", "1.5"} {
				for _, min := range []string{"1", "1.2", "50", "60", "0.6"} {
					for _, step := range []string{"1", "12", "0.6", "0.5", "24"} {
						var got bool
						must(testDB.QueryRow(`SELECT sale_rule_ok($1, $2, NULLIF($3::numeric, 0), $4::numeric, $5::numeric)`, unit, mode, pack, min, step).Scan(&got))
						if want := quantity.SaleRuleOK(unit, mode, parse(pack), parse(min), parse(step)); got != want {
							t.Errorf("%s %s pack %s min %s step %s: sql %v go %v", unit, mode, pack, min, step, got, want)
						}
					}
				}
			}
		}
	}
}

// Upgrading 9 -> 10: demo data is unchanged; a legacy real offer is kept but hidden.
func TestOffersMigration(t *testing.T) {
	requireDB(t)
	ctx := context.Background()
	owner, _ := registered(t)
	company := owner.createCompany()
	card := realCard(t, "piece")
	sum := func() (s string) {
		must(testDB.QueryRow(`SELECT (SELECT md5(string_agg(concat_ws('|', id, product_id, supplier_company_id, price_tiyn, stock_quantity, min_order_quantity, is_active, is_demo, city), ',' ORDER BY id)) FROM offers WHERE is_demo)
			|| (SELECT md5(string_agg((to_jsonb(p) - '{version,brand_id,blocked_at,block_reason,blocked_by}'::text[])::text, ',' ORDER BY id)) FROM products p) || (SELECT md5(string_agg(c::text, ',' ORDER BY id)) FROM companies c)
			|| (SELECT COALESCE(md5(string_agg(r::text, ',' ORDER BY id)), '') FROM catalog_requests r)`).Scan(&s))
		return
	}
	before := sum()
	goose.SetBaseFS(migrations.FS)
	must(goose.DownToContext(ctx, testDB, ".", 9))
	_, err := testDB.Exec(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, stock_quantity, min_order_quantity) VALUES ($1, $2, 777, 3, 0.5)`, card, company)
	must(err)
	must(goose.UpContext(ctx, testDB, "."))
	var state string
	var active bool
	var price int64
	var minQ string
	var sku sql.NullString
	must(testDB.QueryRow(`SELECT owner_state, is_active, price_tiyn, min_order_quantity::text, seller_sku FROM offers WHERE product_id = $1`, card).Scan(&state, &active, &price, &minQ, &sku))
	if state != "hidden" || active || price != 777 || minQ != "0.500" || sku.Valid {
		t.Fatalf("%s %v %d %s %v", state, active, price, minQ, sku)
	}
	if sum() != before {
		t.Fatal("data changed")
	}
	get(t, "/api/v1/products", http.StatusOK, nil)
}
