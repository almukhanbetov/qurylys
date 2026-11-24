package router

// Accounts: registration, sessions, recovery, membership and company isolation.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/config"
	"github.com/qurylys/qurylys/backend/internal/delivery"
	"github.com/qurylys/qurylys/backend/migrations"
)

const pass = "correct horse battery"

// client is a browser: own IP, cookie jar, CSRF header.
type client struct {
	t      *testing.T
	ip     string
	cookie string
}

var ipSeq, contactSeq struct {
	sync.Mutex
	n int
}

func newClient(t *testing.T) *client {
	requireDB(t)
	ipSeq.Lock()
	defer ipSeq.Unlock()
	ipSeq.n++
	return &client{t: t, ip: fmt.Sprintf("203.0.%d.%d", ipSeq.n/250, ipSeq.n%250+1)}
}

func newEmail() string {
	contactSeq.Lock()
	defer contactSeq.Unlock()
	contactSeq.n++
	return fmt.Sprintf("user%d@example.kz", contactSeq.n)
}

func (c *client) request(method, path string, body any) *http.Request {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	}
	r := httptest.NewRequest(method, "/api/v1"+path, rd)
	r.RemoteAddr = c.ip + ":1234"
	r.Header.Set("X-Qurylys-Csrf", "1")
	r.Header.Set("Origin", "http://"+r.Host)
	if c.cookie != "" {
		r.AddCookie(&http.Cookie{Name: "qurylys_session", Value: c.cookie})
	}
	return r
}

func (c *client) send(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, r)
	for _, k := range w.Result().Cookies() {
		if k.Name == "qurylys_session" {
			if !k.HttpOnly || k.SameSite != http.SameSiteLaxMode || k.Path != "/" {
				c.t.Errorf("unsafe cookie: %+v", k)
			}
			c.cookie = k.Value
		}
	}
	return w
}

// do sends a request, checks status and error code ("" = none) and decodes data.
func (c *client) do(method, path string, body any, status int, code string, out any) {
	c.t.Helper()
	w := c.send(c.request(method, path, body))
	var env struct {
		Data  json.RawMessage
		Error struct{ Code string }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if w.Code != status || env.Error.Code != code {
		c.t.Fatalf("%s %s: %d %q, want %d %q; body %s", method, path, w.Code, env.Error.Code, status, code, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			c.t.Fatal(err)
		}
	}
}

func messages(t *testing.T, to string) (out []delivery.Message) {
	t.Helper()
	f, err := os.Open(filepath.Join(outboxDir, delivery.OutboxFile))
	if err != nil {
		return nil
	}
	defer f.Close()
	for sc := bufio.NewScanner(f); sc.Scan(); {
		var m delivery.Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if m.To == to {
			out = append(out, m)
		}
	}
	return out
}

func lastCode(t *testing.T, to string) string {
	t.Helper()
	m := messages(t, to)
	if len(m) == 0 || len(m[len(m)-1].Code) != 6 {
		t.Fatalf("no code for %s: %+v", to, m)
	}
	return m[len(m)-1].Code
}

type ch struct{ Challenge string }

type m = map[string]any

func (c *client) register(contact string) *client {
	c.t.Helper()
	var x ch
	c.do("POST", "/auth/register", m{"full_name": "Айгүл Тест", "contact": contact, "password": pass}, 202, "", &x)
	c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": lastCode(c.t, contact)}, 204, "", nil)
	return c
}

func registered(t *testing.T) (*client, string) {
	e := newEmail()
	return newClient(t).register(e), e
}

type meDTO struct {
	ID              int64
	FullName        string `json:"full_name"`
	Email, Phone    *string
	PlatformRoles   []string `json:"platform_roles"`
	ActiveCompanyID *int64   `json:"active_company_id"`
	Companies       []struct {
		ID   int64
		Role string
	}
	PendingInvitations int `json:"pending_invitations"`
}

func (c *client) me() (v meDTO) {
	c.t.Helper()
	c.do("GET", "/me", nil, 200, "", &v)
	return v
}

var binSeq = 100000000000

func company() m {
	binSeq++
	return m{"name": "ТОО Тест", "bin": fmt.Sprint(binSeq), "city": "Алматы", "legal_address": "ул. 1", "actual_address": "ул. 2",
		"work_phone": "8 701 000 00 01", "representative_name": "Директор", "is_buyer": true, "is_supplier": true}
}

func (c *client) createCompany() int64 {
	c.t.Helper()
	var x struct{ ID int64 }
	c.do("POST", "/companies", company(), 201, "", &x)
	return x.ID
}

func TestRegistrationLoginLogout(t *testing.T) {
	for _, contact := range []string{newEmail(), "+77015550001"} {
		c := newClient(t)
		var x ch
		w := c.send(c.request("POST", "/auth/register", m{"full_name": "Айгүл", "contact": contact, "password": pass}))
		_ = json.Unmarshal(w.Body.Bytes(), &struct{ Data *ch }{&x})
		code := lastCode(t, contact)
		if w.Code != 202 || strings.Contains(w.Body.String(), code) || c.cookie != "" {
			t.Fatalf("code leaked or session given early: %s", w.Body.String())
		}
		// Not a user and cannot sign in until the contact is confirmed.
		c.do("POST", "/auth/login", m{"contact": contact, "password": pass}, 401, "invalid_credentials", nil)
		c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": "abc"}, 400, "invalid_code", nil)
		c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": code}, 204, "", nil)
		c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": code}, 400, "invalid_code", nil) // single use
		if me := c.me(); me.FullName != "Айгүл" || len(me.PlatformRoles) != 0 || len(me.Companies) != 0 {
			t.Fatalf("me: %+v", me)
		}
		old := c.cookie
		c.do("POST", "/auth/logout", nil, 204, "", nil)
		if c.cookie != "" {
			t.Fatal("cookie not cleared")
		}
		c.cookie = old // a stolen copy must be dead on the server too
		c.do("GET", "/me", nil, 401, "unauthorized", nil)
		c.do("POST", "/auth/login", m{"contact": contact, "password": "wrong password"}, 401, "invalid_credentials", nil)
		c.do("POST", "/auth/login", m{"contact": contact, "password": pass}, 204, "", nil)
		if c.cookie == old {
			t.Fatal("session token must change on sign in")
		}
		c.me()
	}
	var plain int
	if err := testDB.QueryRow(`SELECT count(*) FROM users WHERE password_hash NOT LIKE '$argon2id$%'`).Scan(&plain); err != nil || plain != 0 {
		t.Fatalf("passwords must be argon2id: %d %v", plain, err)
	}
}

func TestRegistrationDoesNotRevealOrSelfAssign(t *testing.T) {
	_, taken := registered(t)
	c := newClient(t)
	var x ch
	c.do("POST", "/auth/register", m{"full_name": "Чужой", "contact": taken, "password": "another password 1"}, 202, "", &x)
	if msgs := messages(t, taken); msgs[len(msgs)-1].Kind != "already_registered" || msgs[len(msgs)-1].Code != "" {
		t.Fatalf("owner must be notified without a code: %+v", msgs)
	}
	c.do("POST", "/auth/login", m{"contact": taken, "password": "another password 1"}, 401, "invalid_credentials", nil)
	for _, k := range []string{"role", "platform_roles", "is_admin"} {
		c.do("POST", "/auth/register", m{"full_name": "Х", "contact": newEmail(), "password": pass, k: "admin"}, 400, "invalid_body", nil)
	}
	c.do("POST", "/auth/register", m{"full_name": "Х", "contact": newEmail(), "password": "short"}, 400, "invalid_password", nil)
	c.do("POST", "/auth/register", m{"full_name": " ", "contact": newEmail(), "password": pass}, 400, "invalid_full_name", nil)
	c.do("POST", "/auth/register", m{"full_name": "Х", "contact": "nope", "password": pass}, 400, "invalid_contact", nil)
}

func TestCodeBruteForce(t *testing.T) {
	c, e := newClient(t), newEmail()
	var x ch
	c.do("POST", "/auth/register", m{"full_name": "Х", "contact": e, "password": pass}, 202, "", &x)
	code := lastCode(t, e)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ { // parallel attempts are counted too
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, c.request("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": "x"}))
		}()
	}
	wg.Wait()
	c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": code}, 400, "invalid_code", nil)

	// Expired code.
	c.do("POST", "/auth/register", m{"full_name": "Х", "contact": e, "password": pass}, 202, "", &x)
	if _, err := testDB.Exec(`UPDATE auth_challenges SET expires_at = now() - interval '1 second' WHERE contact = $1`, e); err != nil {
		t.Fatal(err)
	}
	c.do("POST", "/auth/register/confirm", m{"challenge": x.Challenge, "code": lastCode(t, e)}, 400, "invalid_code", nil)
	// Sending is limited per contact.
	for i := 0; i < 3; i++ {
		c.do("POST", "/auth/register", m{"full_name": "Х", "contact": e, "password": pass}, 202, "", &x)
	}
	c.do("POST", "/auth/register", m{"full_name": "Х", "contact": e, "password": pass}, 429, "too_many_requests", nil)
}

func TestLoginBruteForce(t *testing.T) {
	_, e := registered(t)
	c := newClient(t)
	for i := 0; i < 8; i++ {
		c.do("POST", "/auth/login", m{"contact": e, "password": "wrong password"}, 401, "invalid_credentials", nil)
	}
	w := c.send(c.request("POST", "/auth/login", m{"contact": e, "password": pass}))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("want 429 with Retry-After, got %d", w.Code)
	}
	// X-Forwarded-For supplied by a direct client is not trusted.
	r := c.request("POST", "/auth/login", m{"contact": e, "password": pass})
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	if w := c.send(r); w.Code != 429 {
		t.Fatalf("spoofed address bypassed the limit: %d", w.Code)
	}
	newClient(t).do("POST", "/auth/login", m{"contact": e, "password": pass}, 204, "", nil) // owner elsewhere is not locked out
}

func TestPasswordRecovery(t *testing.T) {
	c, e := registered(t)
	anon := newClient(t)
	var known, unknown ch
	anon.do("POST", "/auth/password/forgot", m{"contact": e}, 202, "", &known)
	ghost := newEmail()
	anon.do("POST", "/auth/password/forgot", m{"contact": ghost}, 202, "", &unknown)
	if len(known.Challenge) != len(unknown.Challenge) || len(messages(t, ghost)) != 0 {
		t.Fatal("unknown contact must look the same and get nothing")
	}
	code := lastCode(t, e)
	anon.do("POST", "/auth/password/reset", m{"challenge": unknown.Challenge, "code": code, "password": "brand new pass"}, 400, "invalid_code", nil)
	anon.do("POST", "/auth/password/reset", m{"challenge": known.Challenge, "code": code, "password": "short"}, 400, "invalid_password", nil)
	// A reset challenge cannot be used for another purpose.
	anon.do("POST", "/auth/register/confirm", m{"challenge": known.Challenge, "code": code}, 400, "invalid_code", nil)
	anon.do("POST", "/auth/password/reset", m{"challenge": known.Challenge, "code": code, "password": "brand new pass"}, 204, "", nil)
	anon.do("POST", "/auth/password/reset", m{"challenge": known.Challenge, "code": code, "password": "brand new pass 2"}, 400, "invalid_code", nil)
	c.do("GET", "/me", nil, 401, "unauthorized", nil) // all sessions revoked
	c.do("POST", "/auth/login", m{"contact": e, "password": pass}, 401, "invalid_credentials", nil)
	c.do("POST", "/auth/login", m{"contact": e, "password": "brand new pass"}, 204, "", nil)
}

func TestSessionsProfileAndContacts(t *testing.T) {
	a, e := registered(t)
	b := newClient(t)
	b.do("POST", "/auth/login", m{"contact": e, "password": pass}, 204, "", nil)
	other, _ := registered(t)
	var list []struct {
		ID      int64
		Current bool
	}
	a.do("GET", "/me/sessions", nil, 200, "", &list)
	if len(list) != 2 {
		t.Fatalf("sessions: %+v", list)
	}
	for _, s := range list {
		other.do("DELETE", fmt.Sprintf("/me/sessions/%d", s.ID), nil, 404, "not_found", nil) // foreign session
		if !s.Current {
			a.do("DELETE", fmt.Sprintf("/me/sessions/%d", s.ID), nil, 204, "", nil)
		}
	}
	b.do("GET", "/me", nil, 401, "unauthorized", nil)
	b.do("POST", "/auth/login", m{"contact": e, "password": pass}, 204, "", nil)
	a.do("POST", "/me/password", m{"current_password": "nope nope nope", "password": "next password 1"}, 400, "wrong_password", nil)
	a.do("POST", "/me/password", m{"current_password": pass, "password": "next password 1"}, 204, "", nil)
	a.me()
	b.do("GET", "/me", nil, 401, "unauthorized", nil)

	a.do("PATCH", "/me", m{"full_name": "Жаңа Есім"}, 204, "", nil)
	a.do("PATCH", "/me", m{"full_name": "x", "email": "evil@example.kz"}, 400, "invalid_body", nil)
	phone := "+77015550099"
	var x ch
	a.do("POST", "/me/contacts", m{"contact": phone, "current_password": pass}, 400, "wrong_password", nil)
	a.do("POST", "/me/contacts", m{"contact": phone, "current_password": "next password 1"}, 202, "", &x)
	if a.me().Phone != nil {
		t.Fatal("unconfirmed contact must not be stored")
	}
	other.do("POST", "/me/contacts/confirm", m{"challenge": x.Challenge, "code": lastCode(t, phone)}, 400, "invalid_code", nil)
	a.do("POST", "/me/contacts/confirm", m{"challenge": x.Challenge, "code": lastCode(t, phone)}, 204, "", nil)
	if me := a.me(); me.Phone == nil || *me.Phone != phone || me.FullName != "Жаңа Есім" {
		t.Fatalf("me: %+v", me)
	}
	other.do("POST", "/me/contacts", m{"contact": phone, "current_password": pass}, 202, "", &x)
	other.do("POST", "/me/contacts/confirm", m{"challenge": x.Challenge, "code": lastCode(t, phone)}, 409, "contact_taken", nil)

	// Idle and absolute expiry, blocked users.
	for _, q := range []string{
		`UPDATE sessions SET last_seen_at = now() - interval '8 days' WHERE user_id = $1`,
		`UPDATE sessions SET expires_at = now() WHERE user_id = $1`,
		`UPDATE users SET status = 'blocked' WHERE id = $1`,
	} {
		u, ue := registered(t)
		if _, err := testDB.Exec(q, u.me().ID); err != nil {
			t.Fatal(err)
		}
		u.do("GET", "/me", nil, 401, "unauthorized", nil)
		if strings.Contains(q, "blocked") {
			u.do("POST", "/auth/login", m{"contact": ue, "password": pass}, 401, "invalid_credentials", nil)
		}
	}
}

func TestCSRF(t *testing.T) {
	c, _ := registered(t)
	for name, mod := range map[string]func(*http.Request){
		"no header":      func(r *http.Request) { r.Header.Del("X-Qurylys-Csrf") },
		"foreign origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"null origin":    func(r *http.Request) { r.Header.Set("Origin", "null") },
		"cross-site":     func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Sec-Fetch-Site", "cross-site") },
	} {
		for _, path := range []string{"/auth/logout", "/auth/login", "/companies", "/me/sessions/revoke-others"} {
			r := c.request("POST", path, m{})
			mod(r)
			if w := c.send(r); w.Code != 403 || !strings.Contains(w.Body.String(), "csrf_rejected") {
				t.Errorf("%s %s: %d", name, path, w.Code)
			}
		}
	}
	c.me() // still signed in
}

func TestAnonymousIsRejected(t *testing.T) {
	c := newClient(t)
	for _, p := range []string{"GET /me", "PATCH /me", "GET /me/sessions", "GET /me/invitations", "POST /companies", "GET /companies/1",
		"PUT /companies/1", "GET /companies/1/members", "POST /companies/1/invitations", "GET /admin/users", "PUT /admin/users/1/mrx", "PUT /me/active-company"} {
		method, path, _ := strings.Cut(p, " ")
		c.do(method, path, m{}, 401, "unauthorized", nil)
	}
	c.cookie = "forged"
	c.do("GET", "/me", nil, 401, "unauthorized", nil)
}

func TestCompanyIsolation(t *testing.T) {
	a, _ := registered(t)
	b, _ := registered(t)
	idA, idB := a.createCompany(), b.createCompany()
	bUser := b.me().ID
	var secret string
	a.do("POST", fmt.Sprintf("/companies/%d/invitations", idA), m{"contact": newEmail()}, 204, "", nil)
	var inv int64
	must(testDB.QueryRow(`SELECT id FROM company_invitations WHERE company_id = $1`, idA).Scan(&inv))

	pa := fmt.Sprintf("/companies/%d", idA)
	before := a.send(a.request("GET", pa, nil)).Body.String()
	for _, p := range []string{"GET " + pa, "PUT " + pa, "GET " + pa + "/members", "GET " + pa + "/invitations",
		"DELETE " + pa + "/members/" + fmt.Sprint(a.me().ID), fmt.Sprintf("DELETE %s/invitations/%d", pa, inv)} {
		method, path, _ := strings.Cut(p, " ")
		var body any
		if method == "PUT" {
			body = company()
		}
		b.do(method, path, body, 404, "company_not_found", nil)
	}
	b.do("PATCH", fmt.Sprintf("%s/members/%d", pa, bUser), m{"role": "owner"}, 404, "company_not_found", nil)
	b.do("POST", pa+"/invitations", m{"contact": "x@example.kz"}, 404, "company_not_found", nil)
	b.do("PUT", "/me/active-company", m{"company_id": idA}, 404, "not_found", nil)
	// An invitation of A cannot be reached through B, nor answered by a stranger.
	b.do("DELETE", fmt.Sprintf("/companies/%d/invitations/%d", idB, inv), nil, 404, "not_found", nil)
	b.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv), nil, 404, "not_found", nil)
	// Foreign members cannot be touched through the own company.
	b.do("PATCH", fmt.Sprintf("/companies/%d/members/%d", idB, a.me().ID), m{"role": "owner"}, 404, "not_found", nil)
	// Demo companies have no members and are out of reach.
	must(testDB.QueryRow(`SELECT id::text FROM companies WHERE is_demo LIMIT 1`).Scan(&secret))
	b.do("GET", "/companies/"+secret, nil, 404, "company_not_found", nil)
	if after := a.send(a.request("GET", pa, nil)).Body.String(); after != before {
		t.Fatalf("company A changed: %s -> %s", before, after)
	}
}

func TestCompanyDetails(t *testing.T) {
	c, _ := registered(t)
	for field, bad := range map[string]any{"bin": "123", "name": "", "city": " ", "legal_address": "", "actual_address": "", "work_phone": "1", "representative_name": ""} {
		d := company()
		d[field] = bad
		c.do("POST", "/companies", d, 400, "invalid_"+field, nil)
	}
	d := company()
	d["is_buyer"], d["is_supplier"] = false, false
	c.do("POST", "/companies", d, 400, "invalid_company_type", nil)
	d = company()
	d["status"] = "active"
	c.do("POST", "/companies", d, 400, "invalid_body", nil)

	id := c.createCompany()
	path := fmt.Sprintf("/companies/%d", id)
	var got m
	c.do("GET", path, nil, 200, "", &got)
	if got["work_phone"] != "+77010000001" || got["role"] != "owner" {
		t.Fatalf("company: %v", got)
	}
	dup := company()
	dup["bin"] = got["bin"]
	c.do("POST", "/companies", dup, 409, "bin_taken", nil)
	upd := company()
	upd["name"], upd["bin"], upd["is_supplier"] = "Жаңа атау", got["bin"], false
	c.do("PUT", path, upd, 204, "", nil)
	c.do("GET", path, nil, 200, "", &got)
	if got["name"] != "Жаңа атау" || got["is_supplier"] != false {
		t.Fatalf("not updated: %v", got)
	}
	// A supplier with offers stays a supplier.
	upd["is_supplier"] = true
	c.do("PUT", path, upd, 204, "", nil)
	if _, err := testDB.Exec(`INSERT INTO offers (product_id, supplier_company_id, price_tiyn, is_active) SELECT min(id), $1, 100, FALSE FROM products`, id); err != nil {
		t.Fatal(err)
	}
	defer testDB.Exec(`DELETE FROM offers WHERE supplier_company_id = $1`, id)
	upd["is_supplier"] = false
	c.do("PUT", path, upd, 409, "supplier_has_offers", nil)
	if _, err := testDB.Exec(`UPDATE companies SET status = 'suspended' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	upd["is_supplier"] = true
	c.do("PUT", path, upd, 403, "company_suspended", nil)
}

func TestMembershipAndActiveCompany(t *testing.T) {
	owner, _ := registered(t)
	emp, empMail := registered(t)
	first := owner.createCompany()
	second := owner.createCompany()
	if me := owner.me(); *me.ActiveCompanyID != second || len(me.Companies) != 2 {
		t.Fatalf("me: %+v", me)
	}
	path := fmt.Sprintf("/companies/%d", first)
	owner.do("PUT", path, company(), 409, "company_context_mismatch", nil) // rights apply to the selected company only
	owner.do("GET", path, nil, 409, "company_context_mismatch", nil)
	owner.do("PUT", "/me/active-company", m{"company_id": first}, 204, "", nil)
	owner.do("GET", path, nil, 200, "", nil)

	owner.do("POST", path+"/invitations", m{"contact": strings.ToUpper(empMail)}, 204, "", nil)
	owner.do("POST", path+"/invitations", m{"contact": empMail}, 409, "already_invited", nil)
	if emp.me().PendingInvitations != 1 {
		t.Fatal("invitation not visible")
	}
	var mine []struct{ ID int64 }
	emp.do("GET", "/me/invitations", nil, 200, "", &mine)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", mine[0].ID), nil, 204, "", nil)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", mine[0].ID), nil, 404, "not_found", nil)
	me := emp.me()
	if *me.ActiveCompanyID != first || me.Companies[0].Role != "employee" {
		t.Fatalf("employee: %+v", me)
	}
	// Employee: read only, no contacts of others, no self-promotion.
	emp.do("GET", path, nil, 200, "", nil)
	if b := emp.send(emp.request("GET", path+"/members", nil)).Body.String(); strings.Contains(b, "@") {
		t.Fatalf("employee sees contacts: %s", b)
	}
	emp.do("PUT", path, company(), 403, "forbidden", nil)
	emp.do("PATCH", fmt.Sprintf("%s/members/%d", path, me.ID), m{"role": "owner"}, 403, "forbidden", nil)
	emp.do("POST", path+"/invitations", m{"contact": "y@example.kz"}, 403, "forbidden", nil)
	emp.do("GET", path+"/invitations", nil, 403, "forbidden", nil)
	ownerID := owner.me().ID
	emp.do("DELETE", fmt.Sprintf("%s/members/%d", path, ownerID), nil, 403, "forbidden", nil)

	owner.do("DELETE", fmt.Sprintf("%s/members/%d", path, ownerID), nil, 409, "last_owner", nil)
	owner.do("PATCH", fmt.Sprintf("%s/members/%d", path, ownerID), m{"role": "employee"}, 409, "last_owner", nil)
	owner.do("PATCH", fmt.Sprintf("%s/members/%d", path, me.ID), m{"role": "mrx"}, 400, "invalid_role", nil)
	// Removal takes effect at once, in the open session too.
	owner.do("DELETE", fmt.Sprintf("%s/members/%d", path, me.ID), nil, 204, "", nil)
	if v := emp.me(); v.ActiveCompanyID != nil || len(v.Companies) != 0 {
		t.Fatalf("removed employee: %+v", v)
	}
	emp.do("GET", path, nil, 404, "company_not_found", nil)
}

// Two owners removing each other at the same time must leave one owner.
func TestLastOwnerRace(t *testing.T) {
	for i := 0; i < 5; i++ {
		a, _ := registered(t)
		b, mail := registered(t)
		id := a.createCompany()
		path := fmt.Sprintf("/companies/%d", id)
		a.do("POST", path+"/invitations", m{"contact": mail}, 204, "", nil)
		var mine []struct{ ID int64 }
		b.do("GET", "/me/invitations", nil, 200, "", &mine)
		b.do("POST", fmt.Sprintf("/me/invitations/%d/accept", mine[0].ID), nil, 204, "", nil)
		a.do("PATCH", fmt.Sprintf("%s/members/%d", path, b.me().ID), m{"role": "owner"}, 204, "", nil)
		reqs := []*http.Request{
			a.request("DELETE", fmt.Sprintf("%s/members/%d", path, b.me().ID), nil),
			b.request("DELETE", fmt.Sprintf("%s/members/%d", path, a.me().ID), nil),
		}
		var wg sync.WaitGroup
		for _, r := range reqs {
			wg.Add(1)
			go func() { defer wg.Done(); testRouter.ServeHTTP(httptest.NewRecorder(), r) }()
		}
		wg.Wait()
		var owners int
		must(testDB.QueryRow(`SELECT count(*) FROM company_members WHERE company_id = $1 AND role = 'owner'`, id).Scan(&owners))
		if owners != 1 {
			t.Fatalf("owners left: %d", owners)
		}
	}
}

func TestPlatformRoles(t *testing.T) {
	ctx := context.Background()
	u, _ := registered(t)
	uid := u.me().ID
	u.do("GET", "/admin/users", nil, 403, "forbidden", nil)
	u.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", uid), m{"enabled": true}, 403, "forbidden", nil)

	adminMail := newEmail()
	if _, err := auth.NewRepository(testDB).CreateAdmin(ctx, "Админ", "email", adminMail, auth.HashPassword(pass)); err != nil {
		t.Fatal(err)
	}
	ad := newClient(t)
	ad.do("POST", "/auth/login", m{"contact": adminMail, "password": pass}, 204, "", nil)
	ad.do("GET", "/admin/users?q=example", nil, 200, "", nil)
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", uid), m{"enabled": true}, 204, "", nil)
	if r := u.me().PlatformRoles; len(r) != 1 || r[0] != "mrx" {
		t.Fatalf("roles: %v", r)
	}
	// Platform roles grant nothing inside companies.
	o, _ := registered(t)
	id := o.createCompany()
	for _, c := range []*client{u, ad} {
		c.do("GET", fmt.Sprintf("/companies/%d", id), nil, 404, "company_not_found", nil)
		c.do("PUT", fmt.Sprintf("/companies/%d", id), company(), 404, "company_not_found", nil)
	}
	u.do("GET", "/admin/users", nil, 403, "forbidden", nil) // MRX is not an admin
	ad.do("PUT", fmt.Sprintf("/admin/users/%d/mrx", uid), m{"enabled": false}, 204, "", nil)
	if len(u.me().PlatformRoles) != 0 {
		t.Fatal("mrx not revoked")
	}
	var n int
	must(testDB.QueryRow(`SELECT count(*) FROM audit_events WHERE target_user_id = $1 AND action LIKE 'platform_role.%'`, uid).Scan(&n))
	if n != 2 {
		t.Fatalf("audit events: %d", n)
	}
	if _, err := testDB.Exec(`DELETE FROM audit_events`); err == nil {
		t.Fatal("audit must be append-only")
	}
}

func TestProductionModeWithoutProvider(t *testing.T) {
	requireDB(t)
	r := New(config.Config{AppEnv: config.EnvProduction}, testDB, slog.New(slog.NewTextHandler(io.Discard, nil)),
		delivery.New(delivery.ModeDisabled, "", delivery.SMTPConfig{}, nil))
	c := newClient(t)
	_, mail := registered(t)
	for path, body := range map[string]m{
		"/auth/register":        {"full_name": "Х", "contact": newEmail(), "password": pass},
		"/auth/password/forgot": {"contact": mail},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, c.request("POST", path, body))
		if w.Code != 503 || !strings.Contains(w.Body.String(), "delivery_unavailable") {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, c.request("POST", "/auth/login", m{"contact": mail, "password": pass}))
	k := w.Result().Cookies()
	if w.Code != 204 || len(k) != 1 || !k[0].Secure || !k[0].HttpOnly || k[0].Name != "__Host-qurylys_session" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("production cookie: %d %+v", w.Code, k)
	}
}

// Upgrading an existing database keeps its companies, products and offers.
func TestMigrationKeepsData(t *testing.T) {
	requireDB(t)
	ctx := context.Background()
	count := func() (s string) {
		must(testDB.QueryRow(`SELECT (SELECT md5(string_agg(concat(id, slug, name, city, status), '|' ORDER BY id)) FROM companies WHERE is_demo)
			|| (SELECT md5(string_agg(o::text, '|' ORDER BY id)) FROM offers o) || (SELECT md5(string_agg(p::text, '|' ORDER BY id)) FROM products p)`).Scan(&s))
		return s
	}
	before := count()
	goose.SetBaseFS(migrations.FS)
	must(goose.DownToContext(ctx, testDB, ".", 5))
	must(goose.UpContext(ctx, testDB, "."))
	if count() != before {
		t.Fatal("catalogue data changed")
	}
}

// emailOnly behaves like the SMTP sender: email works, phone does not.
type emailOnly struct {
	mu   sync.Mutex
	sent []delivery.Message
}

func (*emailOnly) Available(ch string) bool { return ch == "email" }
func (*emailOnly) Close(context.Context)    {}
func (e *emailOnly) Send(_ context.Context, m delivery.Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sent = append(e.sent, m)
	return nil
}

func TestEmailOnlyLocaleAndDailyLimit(t *testing.T) {
	requireDB(t)
	_, err := testDB.Exec(`DELETE FROM rate_limits`)
	must(err)
	defer testDB.Exec(`DELETE FROM rate_limits`)
	s := &emailOnly{}
	r := New(config.Config{AppEnv: config.EnvTest, EmailDailyLimit: 3}, testDB, slog.New(slog.NewTextHandler(io.Discard, nil)), s)
	send := func(contact, locale string) (int, string) {
		req := newClient(t).request("POST", "/auth/register", m{"full_name": "Х", "contact": contact, "password": pass})
		if locale != "" {
			req.AddCookie(&http.Cookie{Name: "qurylys_locale", Value: locale})
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	if code, body := send("+77015557777", "ru"); code != 400 || !strings.Contains(body, "channel_unavailable") {
		t.Fatalf("phone: %d %s", code, body)
	}
	for _, l := range []string{"ru", "", "evil"} {
		if code, body := send(newEmail(), l); code != 202 {
			t.Fatalf("%d %s", code, body)
		}
	}
	if s.sent[0].Locale != "ru" || s.sent[1].Locale != "kk" || s.sent[2].Locale != "kk" {
		t.Fatalf("locales: %+v", s.sent)
	}
	if code, _ := send(newEmail(), "ru"); code != 429 || len(s.sent) != 3 {
		t.Fatalf("daily cap not applied: %d, sent %d", code, len(s.sent))
	}
}
