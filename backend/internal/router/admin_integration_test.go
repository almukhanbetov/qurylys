package router

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/qurylys/qurylys/backend/internal/auth"
)

func newAdmin(t *testing.T) *client {
	t.Helper()
	mail := newEmail()
	_, err := auth.NewRepository(testDB).CreateAdmin(context.Background(), "Админ", "email", mail, auth.HashPassword(pass))
	must(err)
	ad := newClient(t)
	ad.do("POST", "/auth/login", m{"contact": mail, "password": pass}, 204, "", nil)
	return ad
}

func withPermission(t *testing.T, perms ...string) *client {
	c, _ := registered(t)
	for _, p := range perms {
		_, err := testDB.Exec(`INSERT INTO platform_permissions (user_id, permission) VALUES ($1, $2)`, c.me().ID, p)
		must(err)
	}
	return c
}

// hideRealCards keeps the cards a test made out of the public catalogue tests,
// as the other catalogue tests do.
func hideRealCards(t *testing.T) {
	t.Cleanup(func() { _, _ = testDB.Exec(`UPDATE products SET is_active = FALSE WHERE NOT is_demo`) })
}

// dropTestCategories removes categories with the t- prefix after the test.
func dropTestCategories(t *testing.T) {
	t.Cleanup(func() {
		const test = `(SELECT id FROM categories WHERE slug LIKE 't-%')`
		_, _ = testDB.Exec(`DELETE FROM catalog_requests WHERE category_id IN ` + test)
		_, _ = testDB.Exec(`UPDATE products SET is_active = FALSE,
			category_id = (SELECT min(id) FROM categories WHERE slug NOT LIKE 't-%') WHERE category_id IN ` + test)
		for {
			res, err := testDB.Exec(`DELETE FROM categories c WHERE slug LIKE 't-%' AND NOT EXISTS (SELECT 1 FROM categories ch WHERE ch.parent_id = c.id)`)
			if n, _ := res.RowsAffected(); err != nil || n == 0 {
				return
			}
		}
	})
}

// raw returns the response body of a successful GET.
func (c *client) raw(path string) string {
	c.t.Helper()
	w := c.send(c.request("GET", path, nil))
	if w.Code != 200 {
		c.t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
	}
	return w.Body.String()
}

type adminPage struct {
	Items []struct {
		ID            int64
		FullName      string   `json:"full_name"`
		PlatformRoles []string `json:"platform_roles"`
		Permissions   []string `json:"platform_permissions"`
		CompanyCount  int      `json:"company_count"`
	}
	Page, Total int
	TotalPages  int `json:"total_pages"`
}

func TestAdminUsersAndGrants(t *testing.T) {
	ad := newAdmin(t)
	u, mail := registered(t)
	uid := u.me().ID
	company := u.createCompany()
	mrx := newMRX(t)
	mod := withPermission(t, auth.PermCatalogModerate)

	// Only administrators: not users, MRX or permission holders.
	for _, c := range []*client{u, mrx, mod} {
		for _, path := range []string{"/admin/users", fmt.Sprintf("/admin/users/%d", uid), "/admin/companies", fmt.Sprintf("/admin/companies/%d/members", company)} {
			c.do("GET", path, nil, 403, "forbidden", nil)
		}
		c.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/catalog.categories", uid), m{"enabled": true}, 403, "forbidden", nil)
	}
	newClient(t).do("GET", "/admin/users", nil, 401, "unauthorized", nil)

	// Search, filters, pagination.
	var pg adminPage
	ad.do("GET", "/admin/users?q="+mail, nil, 200, "", &pg)
	if pg.Total != 1 || pg.Items[0].ID != uid || pg.Items[0].CompanyCount != 1 {
		t.Fatalf("search: %+v", pg)
	}
	ad.do("GET", "/admin/users?access=mrx&q="+strings.TrimSuffix(mail, "@example.kz"), nil, 200, "", &pg)
	if pg.Total != 0 {
		t.Fatalf("mrx filter: %+v", pg)
	}
	ad.do("GET", "/admin/users?membership=with&status=active&page=1", nil, 200, "", &pg)
	if pg.Page != 1 || len(pg.Items) > 20 || pg.TotalPages < 1 {
		t.Fatalf("page: %+v", pg)
	}
	for _, q := range []string{"access=superadmin", "membership=x", "status=deleted", "page=0"} {
		ad.do("GET", "/admin/users?"+q, nil, 400, "invalid_"+strings.Split(q, "=")[0], nil)
	}

	// Grants: only the listed permissions; roles are not grantable here.
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/catalog.categories", uid), m{"enabled": true}, 204, "", nil)
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/catalog.brands", uid), m{"enabled": true}, 204, "", nil)
	for _, p := range []string{"admin", "mrx", "catalog.everything"} {
		ad.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/%s", uid, p), m{"enabled": true}, 404, "not_found", nil)
	}
	ad.do("GET", "/admin/users?access=catalog.brands&q="+mail, nil, 200, "", &pg)
	if pg.Total != 1 || strings.Join(pg.Items[0].Permissions, ",") != "catalog.brands,catalog.categories" || len(pg.Items[0].PlatformRoles) != 0 {
		t.Fatalf("grant: %+v", pg)
	}
	// A permission holder cannot grant anything, also not to themselves.
	u.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/catalog.moderate", uid), m{"enabled": true}, 403, "forbidden", nil)
	u.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", uid), m{"enabled": true}, 403, "forbidden", nil)

	// The profile: confirmed contacts, memberships, grants and their history; no secrets.
	detail := ad.raw(fmt.Sprintf("/admin/users/%d", uid))
	for _, secret := range []string{"password", "hash", "token", "code", "challenge"} {
		if strings.Contains(strings.ToLower(detail), secret) {
			t.Fatalf("user detail contains %q: %s", secret, detail)
		}
	}
	var d struct {
		Email           string
		EmailVerifiedAt *string `json:"email_verified_at"`
		Grants          []struct{ Kind, Value string }
		History         []struct{ Action, Value, Actor string }
		Memberships     []struct {
			CompanyID int64 `json:"company_id"`
			Role      string
		}
	}
	ad.do("GET", fmt.Sprintf("/admin/users/%d", uid), nil, 200, "", &d)
	if d.Email != mail || d.EmailVerifiedAt == nil || len(d.Grants) != 2 || len(d.History) != 2 || d.History[0].Actor != "Админ" ||
		len(d.Memberships) != 1 || d.Memberships[0].CompanyID != company || d.Memberships[0].Role != "owner" {
		t.Fatalf("detail: %+v", d)
	}
	ad.do("GET", "/admin/users/999999999", nil, 404, "not_found", nil)

	// Revocation works at once and is audited.
	u.do("GET", "/admin/categories", nil, 200, "", nil)
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/catalog.categories", uid), m{"enabled": false}, 204, "", nil)
	u.do("GET", "/admin/categories", nil, 403, "forbidden", nil)
	var n int
	must(testDB.QueryRow(`SELECT count(*) FROM audit_events WHERE target_user_id = $1 AND action LIKE 'platform_permission.%'`, uid).Scan(&n))
	if n != 3 {
		t.Fatalf("audit: %d", n)
	}
}

func TestAdminSectionsNeedTheirOwnPermission(t *testing.T) {
	ad := newAdmin(t)
	mrx := newMRX(t)
	mod := withPermission(t, auth.PermCatalogModerate)
	cats := withPermission(t, auth.PermCatalogCategories)
	brandsOnly := withPermission(t, auth.PermCatalogBrands)
	// admin and MRX get nothing of the catalogue by their role.
	for _, c := range []*client{ad, mrx} {
		for _, path := range []string{"/admin/categories", "/admin/brands", "/admin/products"} {
			c.do("GET", path, nil, 403, "forbidden", nil)
		}
	}
	// catalog.moderate does not give categories or brand management, and vice versa.
	mod.do("GET", "/admin/categories", nil, 403, "forbidden", nil)
	mod.do("POST", "/admin/brands", m{"name": "X"}, 403, "forbidden", nil)
	mod.do("GET", "/admin/brands", nil, 200, "", nil) // reads the list to choose a brand for a card
	cats.do("GET", "/admin/products", nil, 403, "forbidden", nil)
	cats.do("GET", "/admin/brands", nil, 403, "forbidden", nil)
	brandsOnly.do("GET", "/admin/products", nil, 403, "forbidden", nil)
	brandsOnly.do("GET", "/admin/categories", nil, 403, "forbidden", nil)
	// None of them is an administrator.
	for _, c := range []*client{mod, cats, brandsOnly} {
		c.do("GET", "/admin/users", nil, 403, "forbidden", nil)
	}
}

func TestAdminCompanyStaff(t *testing.T) {
	ad := newAdmin(t)
	owner, ownerMail := registered(t)
	company := owner.createCompany()
	emp, empMail := registered(t)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", company), m{"contact": empMail}, 204, "", nil)
	var inv []struct{ ID int64 }
	emp.do("GET", "/me/invitations", nil, 200, "", &inv)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)

	var list struct {
		Items []struct {
			ID          int64
			MemberCount int `json:"member_count"`
		}
		Total int
	}
	ad.do("GET", "/admin/companies?q="+fmt.Sprint(binSeq), nil, 200, "", &list)
	if list.Total != 1 || list.Items[0].ID != company || list.Items[0].MemberCount != 2 {
		t.Fatalf("companies: %+v", list)
	}
	body := ad.raw(fmt.Sprintf("/admin/companies/%d/members", company))
	if strings.Contains(body, ownerMail) || strings.Contains(body, empMail) || strings.Contains(body, "work_phone") || strings.Contains(body, "invitation") {
		t.Fatalf("members leak contacts or company data: %s", body)
	}
	var mm struct {
		Members []struct{ Role string }
	}
	ad.do("GET", fmt.Sprintf("/admin/companies/%d/members", company), nil, 200, "", &mm)
	if len(mm.Members) != 2 || mm.Members[0].Role != "owner" || mm.Members[1].Role != "employee" {
		t.Fatalf("members: %+v", mm)
	}
	// The administrator still has no access to the company itself.
	ad.do("GET", fmt.Sprintf("/companies/%d", company), nil, 404, "company_not_found", nil)
	ad.do("GET", fmt.Sprintf("/companies/%d/members", company), nil, 404, "company_not_found", nil)
	ad.do("POST", fmt.Sprintf("/companies/%d/invitations", company), m{"contact": newEmail()}, 404, "company_not_found", nil)
	ad.do("GET", "/admin/companies/999999999/members", nil, 404, "not_found", nil)
}

type adminCategory struct {
	ID       int64
	ParentID *int64 `json:"parent_id"`
	Slug     string
	NameRU   string `json:"name_ru"`
	Version  int
	Products int `json:"product_count"`
	Children int `json:"child_count"`
	Requests int `json:"request_count"`
}

func (c *client) category(id int64) (v adminCategory) {
	c.t.Helper()
	var all []adminCategory
	c.do("GET", "/admin/categories", nil, 200, "", &all)
	for _, x := range all {
		if x.ID == id {
			return x
		}
	}
	c.t.Fatalf("category %d not found", id)
	return
}

func catInput(parent any, slug, name string) m {
	return m{"parent_id": parent, "slug": slug, "name_kk": name + " kk", "name_ru": name, "sort_order": 10}
}

func TestCategoryManagement(t *testing.T) {
	dropTestCategories(t)
	hideRealCards(t)
	mgr := withPermission(t, auth.PermCatalogCategories)
	var root, child, grand struct{ ID int64 }
	mgr.do("POST", "/admin/categories", catInput(nil, "t-root", "Корень"), 201, "", &root)
	mgr.do("POST", "/admin/categories", catInput(root.ID, "t-child", "Дочерняя"), 201, "", &child)
	mgr.do("POST", "/admin/categories", catInput(child.ID, "t-grand", "Внучка"), 201, "", &grand)
	mgr.do("POST", "/admin/categories", catInput(nil, "t-root", "Дубль"), 409, "category_slug_taken", nil)
	mgr.do("POST", "/admin/categories", catInput(nil, "Т-кириллица", "Плохой адрес"), 400, "invalid_slug", nil)
	mgr.do("POST", "/admin/categories", catInput(999999999, "t-orphan", "Сирота"), 400, "invalid_parent", nil)
	mgr.do("POST", "/admin/categories", m{"parent_id": nil, "slug": "t-empty", "name_kk": " ", "name_ru": "Есть", "sort_order": 0}, 400, "invalid_name_kk", nil)

	// Rename, reorder; the slug cannot change.
	c := mgr.category(child.ID)
	in := catInput(root.ID, "", "Дочерняя исправленная")
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", child.ID), with(in, "version", c.Version), 204, "", nil)
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", child.ID), with(in, "version", c.Version, "name_ru", "Опоздавшая"), 409, "category_conflict", nil)
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", child.ID), with(in, "version", c.Version+1, "slug", "t-new"), 400, "invalid_slug", nil)
	if c = mgr.category(child.ID); c.NameRU != "Дочерняя исправленная" || c.Slug != "t-child" {
		t.Fatalf("rename: %+v", c)
	}

	// No cycles: not under itself, not under a descendant.
	r := mgr.category(root.ID)
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", root.ID), with(catInput(root.ID, "", "Корень"), "version", r.Version), 409, "category_cycle", nil)
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", root.ID), with(catInput(grand.ID, "", "Корень"), "version", r.Version), 409, "category_cycle", nil)
	// Moving a subtree keeps ids and links.
	card := realCard(t, "piece")
	_, err := testDB.Exec(`UPDATE products SET category_id = $1 WHERE id = $2`, grand.ID, card)
	must(err)
	g := mgr.category(grand.ID)
	mgr.do("PUT", fmt.Sprintf("/admin/categories/%d", grand.ID), with(catInput(nil, "", "Внучка"), "version", g.Version), 204, "", nil)
	var cat int64
	must(testDB.QueryRow(`SELECT category_id FROM products WHERE id = $1`, card).Scan(&cat))
	if g = mgr.category(grand.ID); g.ParentID != nil || cat != grand.ID || g.Products != 1 {
		t.Fatalf("move: %+v %d", g, cat)
	}

	// Deletion only when unused, with the reason.
	r = mgr.category(root.ID)
	mgr.do("DELETE", fmt.Sprintf("/admin/categories/%d", root.ID), m{"version": r.Version}, 409, "category_has_children", nil)
	g = mgr.category(grand.ID)
	mgr.do("DELETE", fmt.Sprintf("/admin/categories/%d", grand.ID), m{"version": g.Version}, 409, "category_has_products", nil)
	owner, _ := registered(t)
	company := owner.createCompany()
	owner.newRequest(company, with(cardFields(t), "category_id", child.ID))
	c = mgr.category(child.ID)
	mgr.do("DELETE", fmt.Sprintf("/admin/categories/%d", child.ID), m{"version": c.Version}, 409, "category_has_requests", nil)
	var empty struct{ ID int64 }
	mgr.do("POST", "/admin/categories", catInput(root.ID, "t-empty", "Пустая"), 201, "", &empty)
	mgr.do("DELETE", fmt.Sprintf("/admin/categories/%d", empty.ID), m{"version": 1}, 204, "", nil)
	mgr.do("DELETE", fmt.Sprintf("/admin/categories/%d", empty.ID), m{"version": 1}, 404, "not_found", nil)

	var n int
	must(testDB.QueryRow(`SELECT count(*) FROM audit_events WHERE action LIKE 'catalog.category_%' AND actor_user_id = $1`, mgr.me().ID).Scan(&n))
	if n != 7 { // 4 created, 2 updated, 1 deleted
		t.Fatalf("audit: %d", n)
	}
	// The public tree still works with the new categories.
	get(t, "/api/v1/categories/tree", 200, nil)
}

func TestCategoryConcurrentMovesCannotFormACycle(t *testing.T) {
	dropTestCategories(t)
	hideRealCards(t)
	mgr := withPermission(t, auth.PermCatalogCategories)
	var a, b struct{ ID int64 }
	mgr.do("POST", "/admin/categories", catInput(nil, "t-race-a", "А"), 201, "", &a)
	mgr.do("POST", "/admin/categories", catInput(nil, "t-race-b", "Б"), 201, "", &b)
	done := make(chan int, 2)
	for _, pair := range [][2]int64{{a.ID, b.ID}, {b.ID, a.ID}} {
		go func(id, parent int64) {
			w := mgr.send(mgr.request("PUT", fmt.Sprintf("/admin/categories/%d", id), with(catInput(parent, "", "Гонка"), "version", 1)))
			done <- w.Code
		}(pair[0], pair[1])
	}
	codes := []int{<-done, <-done}
	if !(codes[0] == 204) == !(codes[1] == 204) {
		t.Fatalf("exactly one move must win: %v", codes)
	}
}

type brandDTO struct {
	ID       int64
	Name     string
	Products int `json:"product_count"`
	Version  int
}

func TestBrandDirectory(t *testing.T) {
	hideRealCards(t)
	mgr := withPermission(t, auth.PermCatalogBrands)
	var b struct{ ID int64 }
	mgr.do("POST", "/admin/brands", m{"name": "  Тест   Бренд "}, 201, "", &b)
	mgr.do("POST", "/admin/brands", m{"name": "тест бренд"}, 409, "brand_exists", nil)
	mgr.do("POST", "/admin/brands", m{"name": " "}, 400, "invalid_brand_name", nil)

	// Old texts are not merged: only the exact text is linked, on request.
	exact, variant, demo := realCard(t, "piece"), realCard(t, "piece"), int64(0)
	_, err := testDB.Exec(`UPDATE products SET brand = 'Тест Бренд' WHERE id = $1`, exact)
	must(err)
	_, err = testDB.Exec(`UPDATE products SET brand = 'ТЕСТ БРЕНД' WHERE id = $1`, variant)
	must(err)
	must(testDB.QueryRow(`SELECT min(id) FROM products WHERE is_demo`).Scan(&demo))
	var legacy []struct {
		Text     string
		Products int `json:"product_count"`
		Match    *brandDTO
	}
	mgr.do("GET", "/admin/brands/unlinked?q="+url.QueryEscape("тест бренд"), nil, 200, "", &legacy)
	if len(legacy) != 2 || legacy[0].Match == nil || legacy[0].Match.ID != b.ID || legacy[1].Match == nil {
		t.Fatalf("legacy: %+v", legacy)
	}
	var linked struct{ Linked int }
	mgr.do("POST", fmt.Sprintf("/admin/brands/%d/link", b.ID), m{"text": "Тест Бренд"}, 200, "", &linked)
	mgr.do("POST", fmt.Sprintf("/admin/brands/%d/link", b.ID), m{"text": "Тест Бренд"}, 409, "brand_text_not_found", nil)
	var brandID *int64
	var text string
	must(testDB.QueryRow(`SELECT brand_id, brand FROM products WHERE id = $1`, variant).Scan(&brandID, &text))
	if linked.Linked != 1 || brandID != nil || text != "ТЕСТ БРЕНД" {
		t.Fatalf("variant must stay unlinked: %d %v %q", linked.Linked, brandID, text)
	}
	// Demo cards are never linked.
	if _, err := testDB.Exec(`UPDATE products SET brand_id = $1 WHERE id = $2`, b.ID, demo); err == nil {
		t.Fatal("a demo card must not get a brand")
	}

	// Renaming corrects the shown text of linked cards.
	var list struct{ Items []brandDTO }
	mgr.do("GET", "/admin/brands?q=тест", nil, 200, "", &list)
	if len(list.Items) != 1 || list.Items[0].Products != 1 {
		t.Fatalf("list: %+v", list)
	}
	v := list.Items[0].Version
	mgr.do("PUT", fmt.Sprintf("/admin/brands/%d", b.ID), m{"version": v, "name": "Тест-Бренд"}, 204, "", nil)
	mgr.do("PUT", fmt.Sprintf("/admin/brands/%d", b.ID), m{"version": v, "name": "Поздно"}, 409, "brand_conflict", nil)
	must(testDB.QueryRow(`SELECT brand FROM products WHERE id = $1`, exact).Scan(&text))
	if text != "Тест-Бренд" {
		t.Fatalf("rename: %q", text)
	}
	// A used brand cannot be deleted; an unused one can.
	mgr.do("DELETE", fmt.Sprintf("/admin/brands/%d", b.ID), m{"version": v + 1}, 409, "brand_in_use", nil)
	var unused struct{ ID int64 }
	mgr.do("POST", "/admin/brands", m{"name": "Неиспользуемый"}, 201, "", &unused)
	mgr.do("DELETE", fmt.Sprintf("/admin/brands/%d", unused.ID), m{"version": 1}, 204, "", nil)

	var events int
	must(testDB.QueryRow(`SELECT count(*) FROM catalog_events WHERE product_id = $1 AND action = 'card_edited' AND details->'old'->>'brand' = 'Тест Бренд'`, exact).Scan(&events))
	if events != 1 {
		t.Fatal("the old brand text must stay in the card's journal")
	}
}

func TestApprovalLinksAnExistingBrand(t *testing.T) {
	hideRealCards(t)
	mgr := withPermission(t, auth.PermCatalogBrands)
	var b struct{ ID int64 }
	mgr.do("POST", "/admin/brands", m{"name": "Одобрение Бренд"}, 201, "", &b)
	_, _, id, _ := inReviewWith(t, "brand", " одобрение   бренд ")
	moder := newModerator(t)
	moder.confirm(id, 1, 204, "")
	moder.moderate(id, 1, "approve", "", 204, "")
	var brandID int64
	var text string
	must(testDB.QueryRow(`SELECT p.brand_id, p.brand FROM products p JOIN catalog_requests r ON r.product_id = p.id WHERE r.id = $1`, id).Scan(&brandID, &text))
	if brandID != b.ID || text != "Одобрение Бренд" {
		t.Fatalf("approval: %d %q", brandID, text)
	}
}

func inReviewWith(t *testing.T, kv ...any) (owner *client, company, id int64, f m) {
	owner, _ = registered(t)
	company = owner.createCompany()
	f = with(cardFields(t), kv...)
	id = owner.newRequest(company, f)
	owner.do("POST", fmt.Sprintf("%s/%d/submit", cr(company), id), nil, 204, "", nil)
	return
}

type cardDTO struct {
	ID          int64
	CatalogNo   string `json:"catalog_no"`
	NameRU      string `json:"name_ru"`
	CategoryID  int64  `json:"category_id"`
	Unit        string
	GTIN        *string
	NeedsReview bool `json:"needs_review"`
	Blocked     bool
	Version     int
	Conflict    bool
	OfferStats  struct{ Total, Draft, Published int } `json:"offer_stats"`
	Events      []struct{ Action, Comment string }
}

func (c *client) card(id int64) (v cardDTO) {
	c.t.Helper()
	c.do("GET", fmt.Sprintf("/admin/products/%d", id), nil, 200, "", &v)
	return
}

func cardEdit(v cardDTO, reason string, kv ...any) m {
	return with(m{"category_id": v.CategoryID, "name_kk": "Тест карточка", "name_ru": v.NameRU, "description_kk": "", "description_ru": "",
		"brand_id": nil, "unit": v.Unit, "version": v.Version, "reason": reason}, kv...)
}

func TestCardAdministration(t *testing.T) {
	hideRealCards(t)
	mod := withPermission(t, auth.PermCatalogModerate)
	var cat int64
	must(testDB.QueryRow(`SELECT min(id) FROM categories WHERE is_active`).Scan(&cat))
	gtin := newGTIN()
	fields := m{"category_id": cat, "name_kk": "Шуруп 4,2×75", "name_ru": "Саморез 4,2×75", "description_kk": "", "description_ru": "",
		"brand_id": nil, "unit": "piece"}
	mod.do("POST", "/admin/products", with(fields, "gtin", m{"gtin": gtin, "source_kind": "manufacturer", "source": ""}), 400, "invalid_gtin_source", nil)
	mod.do("POST", "/admin/products", with(fields, "gtin", m{"gtin": "2000000000755", "source_kind": "manufacturer", "source": "ТЕСТ"}), 400, "invalid_gtin", nil)
	var created struct{ ID int64 }
	mod.do("POST", "/admin/products", with(fields, "gtin", m{"gtin": gtin, "source_kind": "manufacturer", "source": "ТЕСТ"}), 201, "", &created)
	mod.do("POST", "/admin/products", with(fields, "gtin", m{"gtin": gtin, "source_kind": "manufacturer", "source": "ТЕСТ"}), 409, "gtin_exists", nil)
	t.Cleanup(func() { _, _ = testDB.Exec(`DELETE FROM offers WHERE product_id = $1`, created.ID) })
	c := mod.card(created.ID)
	if !strings.HasPrefix(c.CatalogNo, "QB") || c.NeedsReview || c.GTIN == nil || *c.GTIN != "0"+gtin {
		t.Fatalf("created: %+v", c)
	}

	// Search by name, catalogue number and GTIN; filters.
	var pg struct {
		Items []struct{ ID int64 }
		Total int
	}
	for _, q := range []string{"саморез 4,2", strings.ToLower(c.CatalogNo), gtin} {
		mod.do("GET", "/admin/products?state=real&q="+url.QueryEscape(q), nil, 200, "", &pg)
		if pg.Total < 1 || pg.Items[0].ID != created.ID {
			t.Fatalf("search %q: %+v", q, pg)
		}
	}
	mod.do("GET", "/admin/products?state=demo&q="+gtin, nil, 200, "", &pg)
	if pg.Total != 0 {
		t.Fatal("state filter")
	}
	for _, q := range []string{"state=gone", "brand=abc", "category_id=-1", "page=0"} {
		mod.do("GET", "/admin/products?"+q, nil, 400, "invalid_"+strings.Split(q, "=")[0], nil)
	}

	// Without offers: a correction may change unit and category, with a reason.
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), cardEdit(c, "", "unit", "pack"), 400, "invalid_reason", nil)
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), cardEdit(c, "ошибка единицы", "name_kk", "Шуруп 4,2×75", "unit", "pack"), 204, "", nil)
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), cardEdit(c, "старая версия", "name_kk", "Шуруп 4,2×75"), 409, "card_conflict", nil)
	c = mod.card(created.ID)
	if c.Unit != "pack" || c.Events[0].Action != "card_edited" || c.Events[0].Comment != "ошибка единицы" {
		t.Fatalf("correction: %+v", c)
	}

	// With a supplier's offer: only cosmetic changes (N3).
	owner, company := verifiedSupplier(t)
	offer := owner.newOffer(company, created.ID, with(foam("A-1"), "pack_size", "1", "wholesale", nil))
	_ = offer
	c = mod.card(created.ID)
	base := cardEdit(c, "опечатка", "name_kk", "Шуруп 4,2×75")
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), with(base, "name_ru", "Саморез 4,2×75 мм"), 204, "", nil)
	c = mod.card(created.ID)
	base = cardEdit(c, "замена", "name_kk", "Шуруп 4,2×75")
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), with(base, "name_ru", "Саморез 4,2×90 мм"), 409, "card_identity_change", nil)
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), with(base, "unit", "piece"), 409, "card_locked", nil)
	var other int64
	must(testDB.QueryRow(`SELECT max(id) FROM categories WHERE is_active AND id <> $1`, cat).Scan(&other))
	mod.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), with(base, "category_id", other), 409, "card_locked", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/gtin", created.ID), m{"version": c.Version, "gtin": newGTIN(), "source_kind": "manufacturer", "source": "ТЕСТ", "reason": ""}, 409, "card_locked", nil)
	// The database refuses the same, whatever the path.
	if _, err := testDB.Exec(`UPDATE products SET category_id = $1 WHERE id = $2`, other, created.ID); err == nil {
		t.Fatal("trigger must lock the category of a card with offers")
	}
	// The editor response never carries prices or seller articles.
	body := mod.raw(fmt.Sprintf("/admin/products/%d", created.ID))
	for _, leak := range []string{"price", "150000", "A-1", "seller_sku", "stock"} {
		if strings.Contains(body, leak) {
			t.Fatalf("card view leaks %q", leak)
		}
	}

	// Blocking (R6): reason required; offers cannot be published; public pages hide the card.
	mod.do("POST", fmt.Sprintf("/admin/products/%d/block", created.ID), m{"version": c.Version, "reason": " "}, 400, "invalid_reason", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/block", created.ID), m{"version": c.Version, "reason": "дубль карточки"}, 204, "", nil)
	get(t, fmt.Sprintf("/api/v1/products/%d", created.ID), 404, nil)
	get(t, fmt.Sprintf("/api/v1/products/%d/offers", created.ID), 404, nil)
	o := owner.offer(company, offer)
	if !strings.Contains(strings.Join(o.Reasons, ","), "card_blocked") {
		t.Fatalf("reasons: %v", o.Reasons)
	}
	owner.act(company, offer, "publish", o.Version, 409, "offer_not_publishable")
	var cards []struct{ ID int64 }
	owner.do("GET", fmt.Sprintf("/companies/%d/offer-cards?q=%s", company, gtin), nil, 200, "", &cards)
	if len(cards) != 0 {
		t.Fatal("a blocked card cannot be chosen for a new offer")
	}
	c = mod.card(created.ID)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/block", created.ID), m{"version": c.Version, "reason": "ещё раз"}, 409, "card_already_blocked", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/unblock", created.ID), m{"version": c.Version, "reason": ""}, 204, "", nil)
	get(t, fmt.Sprintf("/api/v1/products/%d", created.ID), 200, nil)
	if o = owner.offer(company, offer); strings.Contains(strings.Join(o.Reasons, ","), "card_blocked") {
		t.Fatal("unblocking must lift the reason")
	}

	// Conflict of interest: a moderator who is or was in the offering company.
	both := withPermission(t, auth.PermCatalogModerate)
	_, err := testDB.Exec(`INSERT INTO company_members (company_id, user_id, role) VALUES ($1, $2, 'employee')`, company, both.me().ID)
	must(err)
	c = both.card(created.ID)
	if !c.Conflict {
		t.Fatal("conflict flag")
	}
	both.do("POST", fmt.Sprintf("/admin/products/%d/block", created.ID), m{"version": c.Version, "reason": "в пользу своей"}, 403, "conflict_of_interest", nil)
	// A former member stays in conflict (R5): leaving is recorded in the audit.
	_, err = testDB.Exec(`INSERT INTO audit_events (actor_user_id, company_id, action) VALUES ($1, $2, 'company.member_joined')`, both.me().ID, company)
	must(err)
	_, err = testDB.Exec(`DELETE FROM company_members WHERE company_id = $1 AND user_id = $2`, company, both.me().ID)
	must(err)
	both.do("PUT", fmt.Sprintf("/admin/products/%d", created.ID), with(cardEdit(c, "опечатка", "name_kk", "Шуруп 4,2×75"), "description_ru", "Описание"), 403, "conflict_of_interest", nil)

	// Audit and journal.
	var n int
	must(testDB.QueryRow(`SELECT count(*) FROM audit_events WHERE action LIKE 'catalog.card_%' AND details->>'product_id' = $1`, fmt.Sprint(created.ID)).Scan(&n))
	if n != 5 { // created, 2 edits, blocked, unblocked
		t.Fatalf("audit: %d", n)
	}
}

func TestLegacyCardGetsGTINOnlyFromASource(t *testing.T) {
	hideRealCards(t)
	mod := withPermission(t, auth.PermCatalogModerate)
	var legacy int64
	must(testDB.QueryRow(`INSERT INTO products (category_id, slug, name_kk, name_ru, unit)
		SELECT min(id), 't-legacy-' || $1, 'Ескі', 'Старая карточка', 'piece' FROM categories WHERE is_active RETURNING id`, newGTIN()).Scan(&legacy))
	c := mod.card(legacy)
	if !c.NeedsReview || c.GTIN != nil {
		t.Fatalf("legacy: %+v", c)
	}
	taken := realCard(t, "piece")
	var takenGTIN string
	must(testDB.QueryRow(`SELECT gtin FROM products WHERE id = $1`, taken).Scan(&takenGTIN))
	mod.do("POST", fmt.Sprintf("/admin/products/%d/gtin", legacy), m{"version": c.Version, "gtin": takenGTIN, "source_kind": "manufacturer", "source": "ТЕСТ", "reason": ""}, 409, "gtin_exists", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/gtin", legacy), m{"version": c.Version, "gtin": newGTIN(), "source_kind": "guess", "source": "ТЕСТ", "reason": ""}, 400, "invalid_gtin_source", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/gtin", legacy), m{"version": c.Version, "gtin": newGTIN(), "source_kind": "packaging", "source": "упаковка, фото", "reason": "сверено"}, 204, "", nil)
	if c = mod.card(legacy); c.NeedsReview || c.GTIN == nil || c.Events[0].Action != "card_gtin_confirmed" {
		t.Fatalf("confirmed: %+v", c)
	}
	// Demo cards are maintained by the seed only.
	var demo int64
	must(testDB.QueryRow(`SELECT min(id) FROM products WHERE is_demo`).Scan(&demo))
	d := mod.card(demo)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/block", demo), m{"version": d.Version, "reason": "x"}, 409, "card_demo", nil)
	mod.do("POST", fmt.Sprintf("/admin/products/%d/gtin", demo), m{"version": d.Version, "gtin": newGTIN(), "source_kind": "packaging", "source": "x", "reason": ""}, 409, "card_demo", nil)
}
