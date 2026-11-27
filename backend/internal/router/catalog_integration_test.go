package router

// Stage 3, steps 3.1-3.2: catalogue numbers, catalog.moderate and requests for new cards.

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/catalog"
	"github.com/qurylys/qurylys/backend/migrations"
)

var gtinSeq = 200000100000

// newGTIN returns a unique GTIN-13 from the restricted range (test data only).
func newGTIN() string {
	gtinSeq++
	base := fmt.Sprintf("%012d", gtinSeq)
	sum := 0
	for i := range base {
		w := 1
		if i%2 == 1 {
			w = 3
		}
		sum += int(base[i]-'0') * w
	}
	return fmt.Sprintf("%s%d", base, (10-sum%10)%10)
}

func cardFields(t *testing.T) m {
	var cat int64
	must(testDB.QueryRow(`SELECT min(id) FROM categories WHERE is_active`).Scan(&cat))
	return m{"category_id": cat, "name_kk": "Монтаж көбігі 750 мл", "name_ru": "Монтажная пена 750 мл", "brand": "", "unit": "piece",
		"description_kk": "", "description_ru": "", "gtin": newGTIN(), "gtin_source_kind": "packaging", "gtin_source": "ТЕСТ"}
}

func cr(company int64) string { return fmt.Sprintf("/companies/%d/catalog-requests", company) }

func mod(id int64) string { return fmt.Sprintf("/catalog/moderation/requests/%d", id) }

func (c *client) newRequest(company int64, f m) int64 {
	c.t.Helper()
	var x struct{ ID int64 }
	c.do("POST", cr(company), f, 201, "", &x)
	return x.ID
}

// inReview returns an owner with a submitted request.
func inReview(t *testing.T) (owner *client, company, id int64, f m) {
	owner, _ = registered(t)
	company = owner.createCompany()
	f = cardFields(t)
	id = owner.newRequest(company, f)
	owner.do("POST", fmt.Sprintf("%s/%d/submit", cr(company), id), nil, 204, "", nil)
	return
}

func newModerator(t *testing.T) *client {
	c, _ := registered(t)
	_, err := testDB.Exec(`INSERT INTO platform_permissions (user_id, permission) VALUES ($1, 'catalog.moderate')`, c.me().ID)
	must(err)
	return c
}

func (c *client) confirm(id int64, n int, status int, code string) {
	c.t.Helper()
	c.do("POST", mod(id)+"/gtin", m{"submission_no": n, "source_kind": "packaging", "source": "фото упаковки, ТЕСТ"}, status, code, nil)
}

func (c *client) moderate(id int64, n int, decision, comment string, status int, code string) {
	c.t.Helper()
	c.do("POST", mod(id)+"/decision", m{"submission_no": n, "decision": decision, "comment": comment}, status, code, nil)
}

type reqDTO struct {
	ID            int64
	Status        string
	Comment       string
	NameRU        string  `json:"name_ru"`
	GTIN          *string `json:"gtin"`
	GTINConfirmed bool    `json:"gtin_confirmed"`
	ProductID     *int64  `json:"product_id"`
	CatalogNo     *string `json:"catalog_no"`
	SubmissionNo  int     `json:"submission_no"`
	Similar       []struct {
		CatalogNo string `json:"catalog_no"`
		SameGTIN  bool   `json:"same_gtin"`
	}
	SameGTINRequests int `json:"same_gtin_requests"`
	Events           []struct{ Action, Actor, Comment string }
}

func TestCatalogNumbers(t *testing.T) {
	requireDB(t)
	var total, distinct, review, gtins int
	must(testDB.QueryRow(`SELECT count(*), count(DISTINCT catalog_no), count(*) FILTER (WHERE needs_review), count(gtin) FROM products WHERE is_demo`).Scan(&total, &distinct, &review, &gtins))
	if total == 0 || distinct != total || review != total || gtins != 0 {
		t.Fatalf("total %d distinct %d review %d gtins %d", total, distinct, review, gtins)
	}
	for name, q := range map[string]string{
		"change number":         `UPDATE products SET catalog_no = 'X'`,
		"change sequence":       `UPDATE products SET catalog_seq = 1`,
		"gtin without source":   `UPDATE products SET gtin = '02000000000756' WHERE id = (SELECT min(id) FROM products)`,
		"bad check digit":       `UPDATE products SET gtin = '02000000000755', gtin_source_kind = 'packaging', gtin_source = 'x', gtin_confirmed_at = now(), gtin_confirmed_by = 1 WHERE id = (SELECT min(id) FROM products)`,
		"reviewed without gtin": `UPDATE products SET needs_review = FALSE WHERE id = (SELECT min(id) FROM products)`,
		"unknown permission":    `INSERT INTO platform_permissions (user_id, permission) VALUES (1, 'admin')`,
	} {
		if _, err := testDB.Exec(q); err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
	var p page
	get(t, "/api/v1/products", http.StatusOK, &p)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/products", nil))
	if !strings.Contains(w.Body.String(), `"catalog_no":"QB`) || strings.Contains(w.Body.String(), "gtin_source") {
		t.Fatal(w.Body.String())
	}
}

// The SQL and Go implementations of the GTIN rule must agree.
func TestGTINRuleInSQL(t *testing.T) {
	requireDB(t)
	for _, raw := range []string{"2000000000756", "2000000000755", "96385074", "036000291452", "00000000", "10012345678902", "10012345678903"} {
		v, ok := catalog.NormalizeGTIN(raw)
		if v == "" {
			v = strings.Repeat("0", 14)
		}
		var sqlOK bool
		must(testDB.QueryRow(`SELECT gtin14_valid($1)`, v).Scan(&sqlOK))
		if sqlOK != ok {
			t.Errorf("%s: go %v, sql %v", raw, ok, sqlOK)
		}
	}
}

func TestCatalogPermissionGrant(t *testing.T) {
	ctx := context.Background()
	u, _ := registered(t)
	uid := u.me().ID
	mail := newEmail()
	_, err := auth.NewRepository(testDB).CreateAdmin(ctx, "Админ", "email", mail, auth.HashPassword(pass))
	must(err)
	ad := newClient(t)
	ad.do("POST", "/auth/login", m{"contact": mail, "password": pass}, 204, "", nil)
	mrx := newMRX(t)
	path := fmt.Sprintf("/admin/users/%d/catalog-moderate", uid)
	// Neither admin nor mrx implies the permission; only an admin can grant it.
	for _, c := range []*client{ad, mrx, u} {
		c.do("GET", "/catalog/moderation/requests", nil, 403, "forbidden", nil)
	}
	for _, c := range []*client{u, mrx} {
		c.do("PUT", path, m{"enabled": true}, 403, "forbidden", nil)
	}
	newClient(t).do("PUT", path, m{"enabled": true}, 401, "unauthorized", nil)
	ad.do("PUT", "/admin/users/999999999/catalog-moderate", m{"enabled": true}, 404, "not_found", nil)
	ad.do("PUT", path, m{"enabled": true}, 204, "", nil)
	ad.do("PUT", path, m{"enabled": true}, 204, "", nil) // idempotent, one audit entry
	u.do("GET", "/catalog/moderation/requests", nil, 200, "", nil)
	u.do("GET", "/admin/users", nil, 403, "forbidden", nil)  // and nothing else
	u.do("GET", "/mrx/requests", nil, 403, "forbidden", nil) // independent of MRX
	if r := u.me().PlatformRoles; len(r) != 0 {
		t.Fatalf("a permission must not become a role: %v", r)
	}
	ad.do("PUT", path, m{"enabled": false}, 204, "", nil)
	u.do("GET", "/catalog/moderation/requests", nil, 403, "forbidden", nil)
	// Explicit self-grant is allowed and audited; MRX and the permission combine.
	self := ad.me().ID
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/catalog-moderate", self), m{"enabled": true}, 204, "", nil)
	ad.do("GET", "/catalog/moderation/requests", nil, 200, "", nil)
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/catalog-moderate", mrx.me().ID), m{"enabled": true}, 204, "", nil)
	mrx.do("GET", "/catalog/moderation/requests", nil, 200, "", nil)
	mrx.do("GET", "/mrx/requests", nil, 200, "", nil)
	var granted, revoked, selfGrant int
	must(testDB.QueryRow(`SELECT count(*) FILTER (WHERE action = 'platform_permission.granted' AND target_user_id = $1),
		count(*) FILTER (WHERE action = 'platform_permission.revoked' AND target_user_id = $1),
		count(*) FILTER (WHERE action = 'platform_permission.granted' AND target_user_id = $2 AND actor_user_id = $2)
		FROM audit_events`, uid, self).Scan(&granted, &revoked, &selfGrant))
	if granted != 1 || revoked != 1 || selfGrant != 1 {
		t.Fatalf("audit: %d %d %d", granted, revoked, selfGrant)
	}
}

func TestCatalogRequestFullFlow(t *testing.T) {
	owner, _ := registered(t)
	company := owner.createCompany()
	f := cardFields(t)
	for field, bad := range map[string]any{"gtin": "2000000000755", "unit": "box", "gtin_source_kind": "internet", "category_id": 999999, "name_ru": strings.Repeat("я", 201)} {
		x := m{}
		for k, v := range f {
			x[k] = v
		}
		x[field] = bad
		code := "invalid_" + strings.TrimSuffix(strings.TrimSuffix(field, "_kind"), "_id")
		owner.do("POST", cr(company), x, 400, code, nil)
	}
	for _, k := range []string{"status", "gtin_confirmed_at", "product_id", "company_id"} {
		x := cardFields(t)
		x[k] = 1
		owner.do("POST", cr(company), x, 400, "invalid_body", nil)
	}
	// An incomplete draft is fine, but cannot be submitted.
	id := owner.newRequest(company, m{"name_ru": "Пена"})
	path := fmt.Sprintf("%s/%d", cr(company), id)
	for _, field := range []string{"category_id", "name_kk", "unit", "gtin", "gtin_source"} {
		x := m{}
		for k, v := range f {
			x[k] = v
		}
		x[field] = nil
		if field != "category_id" && field != "gtin" {
			x[field] = ""
		}
		owner.do("PUT", path, x, 204, "", nil)
		owner.do("POST", path+"/submit", nil, 400, "invalid_"+strings.TrimSuffix(field, "_id"), nil)
	}
	owner.do("PUT", path, f, 204, "", nil)
	mo := newModerator(t)
	mo.do("GET", mod(id), nil, 404, "not_found", nil) // drafts are private
	owner.do("POST", path+"/submit", nil, 204, "", nil)
	owner.do("PUT", path, f, 409, "catalog_request_not_editable", nil)
	owner.do("POST", path+"/submit", nil, 409, "catalog_request_not_editable", nil)

	var r reqDTO
	mo.do("GET", mod(id), nil, 200, "", &r)
	if r.Status != "in_review" || r.GTINConfirmed || *r.GTIN != "0"+f["gtin"].(string) {
		t.Fatalf("%+v", r)
	}
	mo.moderate(id, 1, "approve", "", 409, "gtin_unconfirmed")
	mo.moderate(id, 1, "reject", " ", 400, "invalid_comment")
	mo.moderate(id, 1, "request_changes", "", 400, "invalid_comment")
	mo.do("POST", mod(id)+"/gtin", m{"submission_no": 1, "source_kind": "packaging", "source": " "}, 400, "invalid_gtin_source", nil)
	mo.confirm(id, 1, 204, "")
	mo.moderate(id, 1, "request_changes", "Уточните название", 204, "")
	mo.confirm(id, 1, 409, "catalog_request_conflict")

	owner.do("GET", path, nil, 200, "", &r)
	if r.Status != "needs_changes" || r.Comment != "Уточните название" || !r.GTINConfirmed || r.Events[0].Actor != "" {
		t.Fatalf("%+v", r)
	}
	// Unfinished edits are invisible to the moderator; changing the GTIN drops its confirmation.
	f["name_ru"], f["gtin"] = "Пена монтажная 750 мл", newGTIN()
	owner.do("PUT", path, f, 204, "", nil)
	mo.do("GET", mod(id), nil, 200, "", &r)
	if r.NameRU != "Монтажная пена 750 мл" {
		t.Fatalf("moderator sees a draft edit: %q", r.NameRU)
	}
	owner.do("POST", path+"/submit", nil, 204, "", nil)
	mo.moderate(id, 1, "approve", "", 409, "catalog_request_conflict") // stale tab
	mo.moderate(id, 2, "approve", "", 409, "gtin_unconfirmed")
	mo.confirm(id, 2, 204, "")
	var offersBefore, offersAfter int
	must(testDB.QueryRow(`SELECT count(*) FROM offers`).Scan(&offersBefore))
	mo.moderate(id, 2, "approve", "", 204, "")
	must(testDB.QueryRow(`SELECT count(*) FROM offers`).Scan(&offersAfter))
	owner.do("GET", path, nil, 200, "", &r)
	if r.Status != "approved" || r.ProductID == nil || !strings.HasPrefix(*r.CatalogNo, "QB") || offersAfter != offersBefore {
		t.Fatalf("%+v offers %d -> %d", r, offersBefore, offersAfter)
	}
	var prod struct {
		CatalogNo string  `json:"catalog_no"`
		GTIN      *string `json:"gtin"`
		IsDemo    bool    `json:"is_demo"`
		Offers    int     `json:"offer_count"`
	}
	get(t, fmt.Sprintf("/api/v1/products/%d", *r.ProductID), 200, &prod)
	if prod.CatalogNo != *r.CatalogNo || *prod.GTIN != "0"+f["gtin"].(string) || prod.IsDemo || prod.Offers != 0 {
		t.Fatalf("%+v", prod)
	}
	// The same GTIN cannot make a second card.
	other, _ := registered(t)
	oc := other.createCompany()
	dup := cardFields(t)
	dup["gtin"] = f["gtin"]
	other.do("POST", cr(oc), dup, 409, "gtin_exists", nil)

	var events, audits int
	must(testDB.QueryRow(`SELECT count(*) FROM catalog_events WHERE request_id = $1`, id).Scan(&events))
	must(testDB.QueryRow(`SELECT count(*) FROM audit_events WHERE actor_user_id = $1 AND action LIKE 'catalog.%'`, mo.me().ID).Scan(&audits))
	if events != 7 || audits != 4 {
		t.Fatalf("events %d audits %d", events, audits)
	}
	defer testDB.Exec(`UPDATE products SET is_active = FALSE WHERE id = $1`, *r.ProductID)
}

func TestCatalogRequestPermissions(t *testing.T) {
	owner, company, id, f := inReview(t)
	path := fmt.Sprintf("%s/%d", cr(company), id)
	emp, mail := registered(t)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", company), m{"contact": mail}, 204, "", nil)
	var inv []struct{ ID int64 }
	emp.do("GET", "/me/invitations", nil, 200, "", &inv)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)
	stranger, strangerCompany, _, _ := inReview(t)
	mrx, admin := newMRX(t), newMRX(t)
	_, err := testDB.Exec(`UPDATE platform_roles SET role = 'admin' WHERE user_id = $1`, admin.me().ID)
	must(err)

	emp.do("GET", cr(company), nil, 403, "forbidden", nil)
	emp.do("GET", path, nil, 403, "forbidden", nil)
	emp.do("POST", cr(company), f, 403, "forbidden", nil)
	emp.do("POST", path+"/submit", nil, 403, "forbidden", nil)
	for _, c := range []*client{stranger, mrx, admin, newModerator(t)} {
		c.do("GET", cr(company), nil, 404, "company_not_found", nil)
		c.do("GET", path, nil, 404, "company_not_found", nil)
		c.do("PUT", path, f, 404, "company_not_found", nil)
		c.do("POST", path+"/submit", nil, 404, "company_not_found", nil)
	}
	stranger.do("GET", fmt.Sprintf("%s/%d", cr(strangerCompany), id), nil, 404, "not_found", nil) // through own company
	stranger.do("PUT", fmt.Sprintf("%s/%d", cr(strangerCompany), id), f, 404, "not_found", nil)
	for _, c := range []*client{owner, emp, stranger, mrx, admin} {
		c.do("GET", "/catalog/moderation/requests", nil, 403, "forbidden", nil)
		c.do("GET", mod(id), nil, 403, "forbidden", nil)
		c.confirm(id, 1, 403, "forbidden")
		c.moderate(id, 1, "approve", "", 403, "forbidden")
	}
	anon := newClient(t)
	for _, p := range []string{"GET " + cr(company), "POST " + cr(company), "GET /catalog/moderation/requests", "POST " + mod(id) + "/decision"} {
		method, pth, _ := strings.Cut(p, " ")
		anon.do(method, pth, m{}, 401, "unauthorized", nil)
	}
	// Only a supplier files requests; its requests survive losing the direction.
	d := details(owner, company)
	d["is_supplier"] = false
	owner.do("PUT", fmt.Sprintf("/companies/%d", company), d, 204, "", nil)
	owner.do("POST", cr(company), cardFields(t), 403, "not_supplier", nil)
	var list []reqDTO
	owner.do("GET", cr(company), nil, 200, "", &list)
	if len(list) != 1 {
		t.Fatalf("%+v", list)
	}
}

func TestModeratorConflictOfInterest(t *testing.T) {
	owner, company, id, _ := inReview(t)
	// The author with the permission.
	_, err := testDB.Exec(`INSERT INTO platform_permissions (user_id, permission) VALUES ($1, 'catalog.moderate')`, owner.me().ID)
	must(err)
	// A member, then a former member.
	mo, mail := registered(t)
	_, err = testDB.Exec(`INSERT INTO platform_permissions (user_id, permission) VALUES ($1, 'catalog.moderate')`, mo.me().ID)
	must(err)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", company), m{"contact": mail}, 204, "", nil)
	var inv []struct{ ID int64 }
	mo.do("GET", "/me/invitations", nil, 200, "", &inv)
	mo.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)
	blocked := func(c *client) {
		t.Helper()
		c.do("GET", mod(id), nil, 403, "conflict_of_interest", nil)
		c.confirm(id, 1, 403, "conflict_of_interest")
		c.moderate(id, 1, "approve", "", 403, "conflict_of_interest")
		c.moderate(id, 1, "reject", "x", 403, "conflict_of_interest")
	}
	blocked(owner)
	blocked(mo)
	mo.do("DELETE", fmt.Sprintf("/companies/%d/members/%d", company, mo.me().ID), nil, 204, "", nil)
	blocked(mo)
	// MRX does not lift the conflict.
	_, err = testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'mrx')`, mo.me().ID)
	must(err)
	blocked(mo)
	var l listDTO
	mo.do("GET", "/catalog/moderation/requests?status=in_review", nil, 200, "", &l)
	for _, i := range l.Items {
		if i.Conflict != (i.ID == id) {
			t.Fatalf("%+v", i)
		}
	}
	// A revoked permission is dead at once.
	free := newModerator(t)
	free.do("GET", mod(id), nil, 200, "", nil)
	_, err = testDB.Exec(`DELETE FROM platform_permissions WHERE user_id = $1`, free.me().ID)
	must(err)
	free.confirm(id, 1, 403, "forbidden")
}

func TestModerationRaces(t *testing.T) {
	fire := func(reqs ...*http.Request) (codes []int) {
		codes = make([]int, len(reqs))
		var wg sync.WaitGroup
		for k, r := range reqs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w := httptest.NewRecorder()
				testRouter.ServeHTTP(w, r)
				codes[k] = w.Code
			}()
		}
		wg.Wait()
		return
	}
	for i := 0; i < 6; i++ {
		_, _, id, _ := inReview(t)
		a, b := newModerator(t), newModerator(t)
		a.confirm(id, 1, 204, "")
		codes := fire(
			a.request("POST", mod(id)+"/decision", m{"decision": "approve", "submission_no": 1}),
			b.request("POST", mod(id)+"/decision", m{"decision": "reject", "comment": "нет", "submission_no": 1}),
			b.request("POST", mod(id)+"/decision", m{"decision": "approve", "submission_no": 1}))
		won := 0
		for _, c := range codes {
			if c == 204 {
				won++
			} else if c != 409 {
				t.Fatal(codes)
			}
		}
		var n int
		must(testDB.QueryRow(`SELECT count(*) FROM catalog_events WHERE request_id = $1 AND action IN ('approved','rejected')`, id).Scan(&n))
		if won != 1 || n != 1 {
			t.Fatalf("%v events %d", codes, n)
		}
	}
	// Equally fast contradictory decisions, held at the starting line by a lock on
	// the request: without serialisation several of them pass the status check.
	for i := 0; i < 5; i++ {
		_, company, id, _ := inReview(t)
		tx, err := testDB.Begin()
		must(err)
		_, err = tx.Exec(`SELECT 1 FROM companies WHERE id = $1 FOR UPDATE`, company)
		must(err)
		_, err = tx.Exec(`SELECT 1 FROM catalog_requests WHERE id = $1 FOR UPDATE`, id)
		must(err)
		reqs := []*http.Request{}
		for k := 0; k < 4; k++ {
			d := []string{"reject", "request_changes"}[k%2]
			reqs = append(reqs, newModerator(t).request("POST", mod(id)+"/decision", m{"decision": d, "comment": "нет", "submission_no": 1}))
		}
		go func() {
			time.Sleep(150 * time.Millisecond)
			_ = tx.Rollback()
		}()
		won := 0
		for _, c := range fire(reqs...) {
			if c == 204 {
				won++
			} else if c != 409 {
				t.Fatalf("status %d: decisions are not serialised", c)
			}
		}
		if won != 1 {
			t.Fatalf("won: %d", won)
		}
	}
	// Two companies declare the same GTIN and are approved at the same moment.
	for i := 0; i < 4; i++ {
		_, _, id1, f := inReview(t)
		o2, _ := registered(t)
		c2 := o2.createCompany()
		f2 := cardFields(t)
		f2["gtin"] = f["gtin"]
		id2 := o2.newRequest(c2, f2)
		o2.do("POST", fmt.Sprintf("%s/%d/submit", cr(c2), id2), nil, 204, "", nil)
		mo := newModerator(t)
		var r reqDTO
		mo.do("GET", mod(id1), nil, 200, "", &r)
		if r.SameGTINRequests != 1 {
			t.Fatalf("%+v", r)
		}
		mo.confirm(id1, 1, 204, "")
		mo.confirm(id2, 1, 204, "")
		codes := fire(mo.request("POST", mod(id1)+"/decision", m{"decision": "approve", "submission_no": 1}),
			mo.request("POST", mod(id2)+"/decision", m{"decision": "approve", "submission_no": 1}))
		if codes[0]+codes[1] != 204+409 {
			t.Fatal(codes)
		}
	}
	_, err := testDB.Exec(`UPDATE products SET is_active = FALSE WHERE NOT is_demo`)
	must(err)
}

func TestCatalogDatabaseGuards(t *testing.T) {
	_, _, id, _ := inReview(t)
	for name, q := range map[string]string{
		"edit under review":    `UPDATE catalog_requests SET name_ru = 'x' WHERE id = %d`,
		"approve unconfirmed":  `UPDATE catalog_requests SET status = 'approved', product_id = (SELECT min(id) FROM products) WHERE id = %d`,
		"reject silently":      `UPDATE catalog_requests SET status = 'rejected', comment = '' WHERE id = %d`,
		"back to draft":        `UPDATE catalog_requests SET status = 'draft' WHERE id = %d`,
		"move to another firm": `UPDATE catalog_requests SET company_id = (SELECT min(id) FROM companies) WHERE id = %d`,
		"edit history":         `UPDATE catalog_events SET comment = 'x' WHERE request_id = %d`,
		"delete history":       `DELETE FROM catalog_events WHERE request_id = %d`,
	} {
		if _, err := testDB.Exec(fmt.Sprintf(q, id)); err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
}

func TestCatalogReport(t *testing.T) {
	requireDB(t)
	owner, email := registered(t)
	company := owner.createCompany()
	card := realCard(t, "piece")
	_, err := testDB.Exec(`UPDATE products SET needs_review = TRUE, gtin = NULL, gtin_source_kind = NULL, gtin_source = NULL, gtin_confirmed_at = NULL, gtin_confirmed_by = NULL WHERE id = $1`, card)
	must(err)
	_, err = testDB.Exec(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_active, owner_state, seller_sku, vat_status) VALUES ($2, $1, 100, FALSE, 'published', 'R-1', 'with_vat')`, company, card)
	must(err)
	defer testDB.Exec(`DELETE FROM offers WHERE supplier_company_id = $1`, company)
	var b bytes.Buffer
	must(catalog.Report(context.Background(), testDB, &b))
	out := b.String()
	for _, want := range []string{"company is not verified", "product has no confirmed GTIN", fmt.Sprintf("/ company %d", company), "real, published"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, email) || strings.Contains(out, "ТОО Тест") {
		t.Error("the report must not contain personal or company data")
	}
}

func TestCatalogMigrationKeepsData(t *testing.T) {
	requireDB(t)
	sum := func() (s string) {
		must(testDB.QueryRow(`SELECT (SELECT md5(string_agg(concat_ws('|', id, slug, name_kk, name_ru, brand, unit, attributes, is_active, is_demo, category_id), ',' ORDER BY id)) FROM products)
			|| (SELECT md5(string_agg(o::text, ',' ORDER BY id)) FROM offers o) || (SELECT md5(string_agg(c::text, ',' ORDER BY id)) FROM companies c)
			|| (SELECT md5(string_agg(u::text, ',' ORDER BY id)) FROM users u)`).Scan(&s))
		return
	}
	before := sum()
	ctx := context.Background()
	goose.SetBaseFS(migrations.FS)
	must(goose.DownToContext(ctx, testDB, ".", 8))
	var b bytes.Buffer
	must(catalog.Report(ctx, testDB, &b)) // the report also works before the migration
	if !strings.Contains(b.String(), "schema version: 8") {
		t.Fatal(b.String())
	}
	must(goose.UpContext(ctx, testDB, "."))
	if sum() != before {
		t.Fatal("data changed")
	}
}
