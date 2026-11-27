package router

import (
	"fmt"
	"strings"
	"testing"

	"github.com/qurylys/qurylys/backend/internal/auth"
)

type rolesDTO struct {
	Roles []struct {
		Code        string
		Contour     string
		Scope       string
		BranchScope string `json:"branch_scope"`
		Status      string
		Via         string
		Assignable  bool
		Holders     int
	}
	Mechanisms []struct {
		Key     string
		Holders int
	}
}

func (d rolesDTO) role(t *testing.T, code string) (i int) {
	for i, r := range d.Roles {
		if r.Code == code {
			return i
		}
	}
	t.Fatalf("role %s missing", code)
	return -1
}

func TestRolesDirectory(t *testing.T) {
	ad := newAdmin(t)
	u, _ := registered(t)
	u.createCompany()
	mrx := newMRX(t)
	cat := withPermission(t, auth.PermCatalogModerate, auth.PermCatalogCategories, auth.PermCatalogBrands)
	// Only administrators read it; nobody can write it through the API.
	for _, c := range []*client{u, mrx, cat} {
		c.do("GET", "/admin/roles", nil, 403, "forbidden", nil)
	}
	for _, meth := range []string{"POST", "PUT", "DELETE"} {
		ad.do(meth, "/admin/roles", m{"code": "XX"}, 404, "route_not_found", nil)
	}

	var d rolesDTO
	ad.do("GET", "/admin/roles", nil, 200, "", &d)
	codes := []string{}
	for _, r := range d.Roles {
		codes = append(codes, r.Code)
	}
	if strings.Join(codes, ",") != "MP,SL,MRX,GD,BD,ACC,WH,DR,SR,OP,SV,LG,PR,CA,AU,HR,TS,CC,SA" {
		t.Fatalf("codes: %v", codes)
	}
	for _, r := range d.Roles {
		switch {
		case r.Contour == "erp" && r.Scope != "organization":
			t.Errorf("%s: ERP roles act inside one organisation", r.Code)
		case r.Code == "MRX" && (!r.Assignable || r.Status != "partial" || r.Via != "platform_role"):
			t.Errorf("MRX: %+v", r)
		case r.Code != "MRX" && r.Assignable:
			t.Errorf("%s must not be assignable yet", r.Code)
		case (r.Code == "MP" || r.Code == "SL") && (r.Status != "partial" || r.Via != "company_direction"):
			t.Errorf("%s: %+v", r.Code, r)
		case r.Code != "MP" && r.Code != "SL" && r.Code != "MRX" && (r.Status != "planned" || r.Via != "none" || r.Holders != 0):
			t.Errorf("%s must be planned with no holders: %+v", r.Code, r)
		}
	}
	if d.Roles[d.role(t, "BD")].BranchScope != "assigned" || d.Roles[d.role(t, "GD")].BranchScope != "all" {
		t.Error("branch rules of GD and BD come from §08")
	}
	keys := []string{}
	for _, m := range d.Mechanisms {
		keys = append(keys, m.Key)
	}
	if strings.Join(keys, ",") != "owner,employee,admin,catalog.moderate,catalog.categories,catalog.brands" {
		t.Fatalf("mechanisms: %v", keys)
	}

	// The count follows the real grant and its revocation.
	before := d.Roles[d.role(t, "MRX")].Holders
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", u.me().ID), m{"enabled": true}, 204, "", nil)
	ad.do("GET", "/admin/roles", nil, 200, "", &d)
	if d.Roles[d.role(t, "MRX")].Holders != before+1 {
		t.Fatal("MRX holders")
	}
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", u.me().ID), m{"enabled": false}, 204, "", nil)

	// No path to a role of the directory or to admin through the API.
	for _, code := range []string{"SA", "CA", "GD", "admin", "mrx", "MRX"} {
		ad.do("PUT", fmt.Sprintf("/admin/users/%d/permissions/%s", u.me().ID, code), m{"enabled": true}, 404, "not_found", nil)
	}
	// owner is not CA or GD, admin is not SA: nothing appears in their grants.
	if r := u.me().PlatformRoles; len(r) != 0 {
		t.Fatalf("owner got platform roles: %v", r)
	}
	if r := ad.me().PlatformRoles; len(r) != 1 || r[0] != "admin" {
		t.Fatalf("admin roles: %v", r)
	}
	// MRX does not manage products, prices or staff.
	mrx.do("GET", "/admin/products", nil, 403, "forbidden", nil)
	mrx.do("GET", "/admin/users", nil, 403, "forbidden", nil)
}

func TestSystemRolesAreFixed(t *testing.T) {
	requireDB(t)
	for _, q := range []string{
		`INSERT INTO system_roles (code, contour, scope, branch_scope, status, sort_order) VALUES ('MP', 'marketplace', 'organization', 'none', 'planned', 100)`,
		`INSERT INTO system_roles (code, contour, scope, branch_scope, status, sort_order) VALUES ('XX', 'platform', 'platform', 'none', 'planned', 101)`,
		`DELETE FROM system_roles WHERE code = 'SA'`,
		`UPDATE system_roles SET code = 'SA2' WHERE code = 'SA'`,
		`UPDATE system_roles SET scope = 'platform', branch_scope = 'none' WHERE code = 'CA'`,
		`TRUNCATE system_roles`,
	} {
		if _, err := testDB.Exec(q); err == nil {
			t.Errorf("must fail: %s", q)
		}
	}
	var n int
	must(testDB.QueryRow(`SELECT count(*) FROM system_roles`).Scan(&n))
	if n != 19 {
		t.Fatalf("roles: %d", n)
	}
}

func TestOrganizationRolesStayInTheirOrganization(t *testing.T) {
	a, _ := registered(t)
	idA := a.createCompany()
	b, _ := registered(t)
	idB := b.createCompany()
	// An owner of one company is nothing in another one, whatever it is called.
	a.do("GET", fmt.Sprintf("/companies/%d", idB), nil, 404, "company_not_found", nil)
	a.do("GET", fmt.Sprintf("/companies/%d/offers", idB), nil, 404, "company_not_found", nil)
	a.do("GET", fmt.Sprintf("/companies/%d/members", idB), nil, 404, "company_not_found", nil)
	b.do("PATCH", fmt.Sprintf("/companies/%d/members/%d", idA, b.me().ID), m{"role": "owner"}, 404, "company_not_found", nil)
}
