package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func corsEngine(allowed []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(allowed))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestCORS(t *testing.T) {
	r := corsEngine([]string{"http://localhost:3100"})

	cases := []struct {
		name       string
		method     string
		origin     string
		wantStatus int
		wantAllow  string
	}{
		{"allowed simple", http.MethodGet, "http://localhost:3100", 200, "http://localhost:3100"},
		{"allowed preflight", http.MethodOptions, "http://localhost:3100", 204, "http://localhost:3100"},
		{"foreign simple", http.MethodGet, "http://evil.example", 200, ""},
		{"foreign preflight", http.MethodOptions, "http://evil.example", 403, ""},
		{"same origin, no header", http.MethodGet, "", 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/x", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllow {
				t.Errorf("allow-origin: got %q, want %q", got, tc.wantAllow)
			}
		})
	}
}

func TestCORSDisabledWhenEmpty(t *testing.T) {
	r := corsEngine(nil)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:3100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS headers must not be sent when the allowlist is empty")
	}
}
