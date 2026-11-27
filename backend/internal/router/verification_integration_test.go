package router

// Company verification: owner flow, reviewer decisions, isolation and races.

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/qurylys/qurylys/backend/migrations"
)

var pdfFile = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF\n")

func (c *client) upload(path, name string, data []byte, status int, code string) {
	c.t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	f, _ := w.CreateFormFile("file", name)
	_, _ = f.Write(data)
	_ = w.Close()
	r := c.request("POST", path, nil)
	r.Body = http.NoBody
	r2 := httptest.NewRequest("POST", "/api/v1"+path, &b)
	r2.RemoteAddr, r2.Header = r.RemoteAddr, r.Header
	r2.Header.Set("Content-Type", w.FormDataContentType())
	rec := c.send(r2)
	if rec.Code != status || (code != "" && !strings.Contains(rec.Body.String(), `"`+code+`"`)) {
		c.t.Fatalf("upload %s: %d %s, want %d %s", name, rec.Code, rec.Body.String(), status, code)
	}
}

type docDTO struct {
	ID       int64
	Filename string
	Kind     string
}

type verDTO struct {
	Status     string
	Comment    string
	CanEdit    bool `json:"can_edit"`
	Documents  []docDTO
	Requisites struct{ Name, BIN string }
	Events     []struct {
		Action, Actor, Comment string
		Changed                []struct{ Field, Old, New string }
	}
}

type reviewDTO struct {
	ID           int64
	Status       string
	SubmissionNo int `json:"submission_no"`
	Documents    []docDTO
	Requisites   struct{ Name, City string }
	Events       []struct{ Action, Actor, Comment string }
}

type listDTO struct {
	Items []struct {
		ID          int64
		Status      string
		CompanyName string `json:"company_name"`
		Conflict    bool
	}
	Total int
}

func newMRX(t *testing.T) *client {
	c, _ := registered(t)
	_, err := testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'mrx')`, c.me().ID)
	must(err)
	return c
}

func ver(id int64) string { return fmt.Sprintf("/companies/%d/verification", id) }

// submitted returns an owner whose company is under review.
func submitted(t *testing.T) (owner *client, email string, company int64) {
	owner, email = registered(t)
	company = owner.createCompany()
	owner.do("POST", ver(company), nil, 204, "", nil)
	owner.upload(ver(company)+"/documents?kind=registration", "талон.pdf", pdfFile, 204, "")
	owner.do("POST", ver(company)+"/submit", nil, 204, "", nil)
	return owner, email, company
}

func requestID(t *testing.T, company int64) (id int64) {
	must(testDB.QueryRow(`SELECT max(id) FROM verification_requests WHERE company_id = $1`, company).Scan(&id))
	return id
}

func (c *client) decide(id int64, n int, decision, comment string, status int, code string) {
	c.t.Helper()
	c.do("POST", fmt.Sprintf("/mrx/requests/%d/decision", id), m{"decision": decision, "comment": comment, "submission_no": n}, status, code, nil)
}

func details(c *client, id int64) m {
	var d m
	c.do("GET", fmt.Sprintf("/companies/%d", id), nil, 200, "", &d)
	for _, k := range []string{"id", "status", "role", "verification"} {
		delete(d, k)
	}
	return d
}

func TestVerificationFullFlow(t *testing.T) {
	owner, email := registered(t)
	id := owner.createCompany()
	var v verDTO
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "none" || v.CanEdit {
		t.Fatalf("%+v", v)
	}
	owner.do("POST", ver(id)+"/submit", nil, 409, "verification_not_editable", nil)
	owner.do("POST", ver(id), nil, 204, "", nil)
	owner.do("POST", ver(id), nil, 409, "verification_exists", nil)
	owner.do("POST", ver(id)+"/submit", nil, 409, "document_required", nil)
	owner.upload(ver(id)+"/documents?kind=other", "x.pdf", pdfFile, 204, "")
	owner.do("POST", ver(id)+"/submit", nil, 409, "document_required", nil)
	owner.upload(ver(id)+"/documents?kind=registration", "../../Талон №1.exe", pdfFile, 204, "")
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "draft" || len(v.Documents) != 2 || v.Documents[1].Filename != "Талон №1.pdf" {
		t.Fatalf("%+v", v)
	}
	mrx := newMRX(t)
	rid := requestID(t, id)
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 404, "not_found", nil) // drafts are private
	owner.do("POST", ver(id)+"/submit", nil, 204, "", nil)
	owner.do("POST", ver(id)+"/submit", nil, 409, "verification_not_editable", nil)
	owner.upload(ver(id)+"/documents?kind=other", "x.pdf", pdfFile, 409, "verification_not_editable")
	owner.do("DELETE", fmt.Sprintf("%s/documents/%d", ver(id), v.Documents[0].ID), nil, 409, "verification_not_editable", nil)

	var l listDTO
	mrx.do("GET", "/mrx/requests?status=in_review", nil, 200, "", &l)
	found := false
	for _, i := range l.Items {
		found = found || i.ID == rid
		if i.Status != "in_review" {
			t.Fatalf("filter: %+v", i)
		}
	}
	if !found {
		t.Fatal("submitted request not listed")
	}
	var r reviewDTO
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 200, "", &r)
	if len(r.Documents) != 2 || r.SubmissionNo != 1 {
		t.Fatalf("%+v", r)
	}
	w := mrx.send(mrx.request("GET", fmt.Sprintf("/mrx/requests/%d/documents/%d", rid, r.Documents[0].ID), nil))
	h := w.Header()
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), pdfFile) || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") ||
		h.Get("Cache-Control") != "no-store" || h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("file: %d %v", w.Code, h)
	}

	// Comment rules.
	for _, d := range []string{"request_changes", "reject"} {
		mrx.decide(rid, 1, d, "  ", 400, "invalid_comment")
	}
	mrx.decide(rid, 1, "approve", strings.Repeat("я", 2001), 400, "invalid_comment")
	mrx.decide(rid, 1, "verify", "", 400, "invalid_decision")
	mrx.decide(rid, 1, "request_changes", "Нужен устав", 204, "")
	mrx.decide(rid, 1, "approve", "", 409, "verification_conflict")

	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "needs_changes" || v.Comment != "Нужен устав" || !v.CanEdit || v.Events[0].Actor != "" {
		t.Fatalf("company view: %+v", v)
	}
	// Edits before resubmission are invisible to the reviewer.
	owner.do("DELETE", fmt.Sprintf("%s/documents/%d", ver(id), v.Documents[0].ID), nil, 204, "", nil)
	owner.upload(ver(id)+"/documents?kind=authority", "устав.pdf", pdfFile, 204, "")
	before := r
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 200, "", &r)
	if len(r.Documents) != 2 || r.Documents[0].ID != before.Documents[0].ID {
		t.Fatalf("reviewer sees an unfinished edit: %+v", r.Documents)
	}
	d := details(owner, id)
	d["name"] = "ТОО Исправлено"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	owner.do("POST", ver(id)+"/submit", nil, 204, "", nil)
	mrx.decide(rid, 1, "approve", "", 409, "verification_conflict") // an old tab must not decide on new data
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 200, "", &r)
	if r.SubmissionNo != 2 || len(r.Documents) != 2 || r.Documents[1].Filename != "устав.pdf" || r.Requisites.Name != "ТОО Исправлено" {
		t.Fatalf("resubmission: %+v", r)
	}
	mrx.decide(rid, 2, "approve", "", 204, "")
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "approved" || v.CanEdit || len(v.Events) != 5 || len(v.Events[1].Changed) != 1 || v.Events[1].Changed[0].Field != "name" {
		t.Fatalf("approved: %+v", v)
	}
	owner.do("POST", ver(id), nil, 409, "already_verified", nil)
	var who int
	must(testDB.QueryRow(`SELECT count(*) FROM verification_events WHERE request_id = $1 AND actor_user_id = $2 AND action IN ('approved','changes_requested')`, rid, mrx.me().ID).Scan(&who))
	if who != 2 {
		t.Fatalf("history: %d", who)
	}

	// Notifications: one per decision, without data.
	var kinds []string
	for _, msg := range messages(t, email) {
		if strings.HasPrefix(msg.Kind, "verification_") {
			kinds = append(kinds, msg.Kind)
		}
	}
	if strings.Join(kinds, ",") != "verification_changes,verification_approved" {
		t.Fatalf("mails: %v", kinds)
	}

	// Non-key details keep the verification, key details drop it.
	d["city"], d["work_phone"], d["actual_address"] = "Астана", "+77019999999", "другой"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "approved" {
		t.Fatal(v.Status)
	}
	d["legal_address"] = "новый адрес"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "reverification" || v.Events[0].Action != "verification_lost" || v.Events[0].Changed[0].Field != "legal_address" {
		t.Fatalf("%+v", v)
	}
	d["legal_address"] = "ул. 1" // changing it back does not restore the verification
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "reverification" {
		t.Fatal(v.Status)
	}
	owner.do("POST", ver(id), nil, 204, "", nil) // a new application is possible
}

func TestVerificationRejectAndReapply(t *testing.T) {
	owner, _, id := submitted(t)
	mrx := newMRX(t)
	rid := requestID(t, id)
	mrx.decide(rid, 1, "reject", "Документ не читается", 204, "")
	var v verDTO
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "rejected" || v.Comment == "" {
		t.Fatalf("%+v", v)
	}
	owner.do("POST", ver(id)+"/submit", nil, 409, "verification_not_editable", nil)
	owner.do("POST", ver(id), nil, 204, "", nil)
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "draft" || len(v.Documents) != 0 {
		t.Fatalf("%+v", v)
	}
	mrx.decide(rid, 1, "approve", "", 409, "verification_conflict")
}

func TestVerificationPermissions(t *testing.T) {
	owner, _, id := submitted(t)
	stranger, _, strangerID := submitted(t)
	emp, mail := registered(t)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", id), m{"contact": mail}, 204, "", nil)
	var inv []struct{ ID int64 }
	emp.do("GET", "/me/invitations", nil, 200, "", &inv)
	emp.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)
	var ov verDTO
	owner.do("GET", ver(id), nil, 200, "", &ov)
	doc := fmt.Sprintf("%s/documents/%d", ver(id), ov.Documents[0].ID)
	rid := requestID(t, id)

	// Employee: status only.
	var v verDTO
	emp.do("GET", ver(id), nil, 200, "", &v)
	if v.Status != "in_review" || len(v.Documents) != 0 || v.CanEdit {
		t.Fatalf("employee: %+v", v)
	}
	emp.do("POST", ver(id)+"/submit", nil, 403, "forbidden", nil)
	emp.do("POST", ver(id), nil, 403, "forbidden", nil)
	emp.upload(ver(id)+"/documents?kind=other", "x.pdf", pdfFile, 403, "forbidden")
	emp.do("GET", doc, nil, 403, "forbidden", nil)
	emp.do("DELETE", doc, nil, 403, "forbidden", nil)

	// Another company, also through its own id.
	stranger.do("GET", ver(id), nil, 404, "company_not_found", nil)
	stranger.do("POST", ver(id)+"/submit", nil, 404, "company_not_found", nil)
	stranger.upload(ver(id)+"/documents?kind=other", "x.pdf", pdfFile, 404, "company_not_found")
	stranger.do("GET", doc, nil, 404, "company_not_found", nil)
	stranger.do("GET", fmt.Sprintf("%s/documents/%d", ver(strangerID), ov.Documents[0].ID), nil, 404, "not_found", nil)

	// Reviewer endpoints: role only.
	admin, _ := registered(t)
	_, err := testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'admin')`, admin.me().ID)
	must(err)
	for _, c := range []*client{owner, emp, stranger, admin} {
		c.do("GET", "/mrx/requests", nil, 403, "forbidden", nil)
		c.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 403, "forbidden", nil)
		c.do("GET", fmt.Sprintf("/mrx/requests/%d/documents/%d", rid, ov.Documents[0].ID), nil, 403, "forbidden", nil)
		c.decide(rid, 1, "approve", "", 403, "forbidden")
	}
	anon := newClient(t)
	for _, p := range []string{"GET " + ver(id), "GET " + doc, "POST " + ver(id) + "/submit", "GET /mrx/requests", fmt.Sprintf("POST /mrx/requests/%d/decision", rid)} {
		method, path, _ := strings.Cut(p, " ")
		anon.do(method, path, m{}, 401, "unauthorized", nil)
	}

	// The reviewer has no rights inside the company.
	mrx := newMRX(t)
	mrx.do("GET", fmt.Sprintf("/companies/%d", id), nil, 404, "company_not_found", nil)
	mrx.do("PUT", fmt.Sprintf("/companies/%d", id), company(), 404, "company_not_found", nil)
	mrx.do("POST", ver(id)+"/submit", nil, 404, "company_not_found", nil)
	mrx.do("GET", doc, nil, 404, "company_not_found", nil)
	// A document of one request cannot be read through another.
	other := requestID(t, strangerID)
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d/documents/%d", other, ov.Documents[0].ID), nil, 404, "not_found", nil)

	// Nobody can set the status through company data.
	owner.do("PUT", "/me/active-company", m{"company_id": id}, 204, "", nil)
	for _, k := range []string{"verification", "verified_request_id", "status"} {
		d := details(owner, id)
		d[k] = "approved"
		owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 400, "invalid_body", nil)
		d2 := company()
		d2[k] = "approved"
		owner.do("POST", "/companies", d2, 400, "invalid_body", nil)
	}
	// Key details are locked during the review, others are not.
	d := details(owner, id)
	d["name"] = "Другое имя"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 409, "company_in_review", nil)
	d = details(owner, id)
	d["city"] = "Шымкент"
	owner.do("PUT", fmt.Sprintf("/companies/%d", id), d, 204, "", nil)
	// The reviewer still sees what was submitted.
	var r reviewDTO
	mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 200, "", &r)
	if r.Requisites.City != "Алматы" {
		t.Fatalf("snapshot: %+v", r.Requisites)
	}
	// Revoked role takes effect at once; a suspended company cannot be approved.
	_, err = testDB.Exec(`UPDATE companies SET status = 'suspended' WHERE id = $1`, id)
	must(err)
	mrx.decide(rid, 1, "approve", "", 403, "company_suspended")
	_, err = testDB.Exec(`DELETE FROM platform_roles WHERE user_id = $1`, mrx.me().ID)
	must(err)
	mrx.decide(rid, 1, "reject", "x", 403, "forbidden")
}

func TestReviewerConflictOfInterest(t *testing.T) {
	owner, _, id := submitted(t)
	rid := requestID(t, id)
	mrx, mail := registered(t)
	_, err := testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'mrx')`, mrx.me().ID)
	must(err)
	owner.do("POST", fmt.Sprintf("/companies/%d/invitations", id), m{"contact": mail}, 204, "", nil)
	var inv []struct{ ID int64 }
	mrx.do("GET", "/me/invitations", nil, 200, "", &inv)
	mrx.do("POST", fmt.Sprintf("/me/invitations/%d/accept", inv[0].ID), nil, 204, "", nil)
	blocked := func() {
		t.Helper()
		mrx.do("GET", fmt.Sprintf("/mrx/requests/%d", rid), nil, 403, "conflict_of_interest", nil)
		mrx.do("GET", fmt.Sprintf("/mrx/requests/%d/documents/1", rid), nil, 403, "conflict_of_interest", nil)
		mrx.decide(rid, 1, "approve", "", 403, "conflict_of_interest")
		mrx.decide(rid, 1, "reject", "x", 403, "conflict_of_interest")
	}
	blocked()
	var l listDTO
	mrx.do("GET", "/mrx/requests?status=in_review", nil, 200, "", &l)
	for _, i := range l.Items {
		if i.Conflict != (i.ID == rid) {
			t.Fatalf("conflict flag: %+v", i)
		}
	}
	// Leaving the company does not help.
	mrx.do("DELETE", fmt.Sprintf("/companies/%d/members/%d", id, mrx.me().ID), nil, 204, "", nil)
	blocked()
	// An owner who later becomes a reviewer cannot decide either.
	_, err = testDB.Exec(`INSERT INTO platform_roles (user_id, role) VALUES ($1, 'mrx')`, owner.me().ID)
	must(err)
	owner.decide(rid, 1, "approve", "", 403, "conflict_of_interest")
}

// Contradictory simultaneous decisions: exactly one wins.
func TestDecisionRace(t *testing.T) {
	for i := 0; i < 8; i++ {
		_, _, id := submitted(t)
		rid := requestID(t, id)
		a, b := newMRX(t), newMRX(t)
		reqs := []*http.Request{
			a.request("POST", fmt.Sprintf("/mrx/requests/%d/decision", rid), m{"decision": "approve", "submission_no": 1}),
			b.request("POST", fmt.Sprintf("/mrx/requests/%d/decision", rid), m{"decision": "reject", "comment": "нет", "submission_no": 1}),
			b.request("POST", fmt.Sprintf("/mrx/requests/%d/decision", rid), m{"decision": "request_changes", "comment": "нет", "submission_no": 1}),
		}
		codes := make([]int, len(reqs))
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
		var won, n int
		for _, c := range codes {
			if c == 204 {
				won++
			} else if c != 409 {
				t.Fatalf("codes: %v", codes)
			}
		}
		var status string
		var verified bool
		must(testDB.QueryRow(`SELECT r.status, c.verified_request_id IS NOT NULL, (SELECT count(*) FROM verification_events e WHERE e.request_id = r.id AND e.from_status = 'in_review')
			FROM verification_requests r JOIN companies c ON c.id = r.company_id WHERE r.id = $1`, rid).Scan(&status, &verified, &n))
		if won != 1 || n != 1 || verified != (status == "approved") {
			t.Fatalf("won %d, events %d, %s verified=%v", won, n, status, verified)
		}
	}
}

// A decision racing with a change of key details never verifies unseen data.
func TestApproveVersusEditRace(t *testing.T) {
	for i := 0; i < 8; i++ {
		owner, _, id := submitted(t)
		rid := requestID(t, id)
		mrx := newMRX(t)
		d := details(owner, id)
		d["name"] = "Подмена"
		reqs := []*http.Request{
			mrx.request("POST", fmt.Sprintf("/mrx/requests/%d/decision", rid), m{"decision": "approve", "submission_no": 1}),
			owner.request("PUT", fmt.Sprintf("/companies/%d", id), d),
		}
		var wg sync.WaitGroup
		for _, r := range reqs {
			wg.Add(1)
			go func() { defer wg.Done(); testRouter.ServeHTTP(httptest.NewRecorder(), r) }()
		}
		wg.Wait()
		var bad bool
		must(testDB.QueryRow(`SELECT verified_request_id IS NOT NULL AND name = 'Подмена' FROM companies WHERE id = $1`, id).Scan(&bad))
		if bad {
			t.Fatal("changed details are verified")
		}
	}
}

func TestDocumentUploadRules(t *testing.T) {
	owner, _ := registered(t)
	id := owner.createCompany()
	path := ver(id) + "/documents?kind=registration"
	owner.upload(path, "x.pdf", pdfFile, 409, "verification_not_editable")
	owner.do("POST", ver(id), nil, 204, "", nil)
	for name, data := range map[string][]byte{"evil.pdf": []byte("<html><script>"), "x.png": []byte("MZ\x90"), "x.jpg": []byte("PK\x03\x04")} {
		owner.upload(path, name, data, 415, "file_type")
	}
	owner.upload(path, "empty.pdf", nil, 400, "invalid_body")
	owner.upload(ver(id)+"/documents?kind=passport", "x.pdf", pdfFile, 400, "invalid_kind")
	big := append(append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 5<<20)...), "%%EOF"...)
	owner.upload(path, "big.pdf", big, 413, "file_too_large")
	owner.upload(path, "huge.pdf", append(big, make([]byte, 2<<20)...), 413, "file_too_large")
	exact := append(append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 5<<20-9-5)...), "%%EOF"...)
	owner.upload(path, "exact.pdf", exact, 204, "")
	var p bytes.Buffer
	_ = png.Encode(&p, image.NewGray(image.Rect(0, 0, 2, 2)))
	owner.upload(path, "photo.pdf", p.Bytes(), 204, "") // real type wins over the name
	var v verDTO
	owner.do("GET", ver(id), nil, 200, "", &v)
	if v.Documents[1].Filename != "photo.png" {
		t.Fatalf("%+v", v.Documents)
	}
	for i := 0; i < 8; i++ {
		owner.upload(path, "x.pdf", pdfFile, 204, "")
	}
	owner.upload(path, "x.pdf", pdfFile, 409, "too_many_documents")
	// An upload without the CSRF header is refused.
	r := owner.request("POST", path, nil)
	r.Header.Del("X-Qurylys-Csrf")
	if w := owner.send(r); w.Code != 403 {
		t.Fatalf("csrf: %d", w.Code)
	}
	// Never-submitted files are really deleted.
	owner.do("DELETE", fmt.Sprintf("%s/documents/%d", ver(id), v.Documents[0].ID), nil, 204, "", nil)
	var left int
	must(testDB.QueryRow(`SELECT count(*) FROM verification_documents WHERE id = $1`, v.Documents[0].ID).Scan(&left))
	if left != 0 {
		t.Fatal("draft file kept")
	}
}

func TestVerificationDatabaseGuards(t *testing.T) {
	_, _, id := submitted(t)
	rid := requestID(t, id)
	_, _, otherID := submitted(t)
	for name, q := range map[string]string{
		"verify without approval":  fmt.Sprintf(`UPDATE companies SET verified_request_id = %d WHERE id = %d`, rid, id),
		"verify by foreign":        fmt.Sprintf(`UPDATE companies SET verified_request_id = %d WHERE id = %d`, rid, otherID),
		"rename under review":      fmt.Sprintf(`UPDATE companies SET name = 'x' WHERE id = %d`, id),
		"back to draft":            fmt.Sprintf(`UPDATE verification_requests SET status = 'draft', submission_no = 0 WHERE id = %d`, rid),
		"reject without comment":   fmt.Sprintf(`UPDATE verification_requests SET status = 'rejected', comment = ' ' WHERE id = %d`, rid),
		"insert approved":          fmt.Sprintf(`INSERT INTO verification_requests (company_id, status, submission_no, snapshot, submitted_at) VALUES (%d, 'approved', 1, '{}', now())`, id),
		"second open request":      fmt.Sprintf(`INSERT INTO verification_requests (company_id) VALUES (%d)`, id),
		"edit history":             `UPDATE verification_events SET comment = 'x'`,
		"delete history":           `DELETE FROM verification_events`,
		"insert verified company":  fmt.Sprintf(`INSERT INTO companies (slug, name, is_buyer, verified_request_id) VALUES ('zz-x', 'x', TRUE, %d)`, rid),
		"delete submitted company": fmt.Sprintf(`DELETE FROM companies WHERE id = %d`, id),
	} {
		if _, err := testDB.Exec(q); err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
	// Any code path that changes key data drops the verification.
	mrx := newMRX(t)
	mrx.decide(rid, 1, "approve", "", 204, "")
	_, err := testDB.Exec(`UPDATE companies SET bin = '999999999999' WHERE id = $1`, id)
	must(err)
	var verified bool
	must(testDB.QueryRow(`SELECT verified_request_id IS NOT NULL FROM companies WHERE id = $1`, id).Scan(&verified))
	if verified {
		t.Fatal("verification survived a BIN change")
	}
}

// Upgrading from stage 1 keeps users, memberships and catalogue data.
func TestVerificationMigrationKeepsData(t *testing.T) {
	requireDB(t)
	owner, _ := registered(t)
	owner.createCompany()
	sum := func() (s string) {
		must(testDB.QueryRow(`SELECT (SELECT md5(string_agg(concat_ws('|', id, slug, name, bin, city, legal_address, status), ',' ORDER BY id)) FROM companies)
			|| (SELECT md5(string_agg(u::text, ',' ORDER BY id)) FROM users u) || (SELECT md5(string_agg(m::text, ',' ORDER BY company_id, user_id)) FROM company_members m)
			|| (SELECT md5(string_agg(o::text, ',' ORDER BY id)) FROM offers o) || (SELECT md5(string_agg((to_jsonb(p) - '{version,brand_id,blocked_at,block_reason,blocked_by}'::text[])::text, ',' ORDER BY id)) FROM products p)`).Scan(&s))
		return s
	}
	before := sum()
	goose.SetBaseFS(migrations.FS)
	must(goose.DownToContext(context.Background(), testDB, ".", 7))
	must(goose.UpContext(context.Background(), testDB, "."))
	if sum() != before {
		t.Fatal("data changed")
	}
	var v verDTO
	owner.do("GET", ver(*owner.me().ActiveCompanyID), nil, 200, "", &v)
	if v.Status != "none" {
		t.Fatal(v.Status)
	}
}
