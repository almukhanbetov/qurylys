package middleware

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

// CSRFHeader must accompany every state-changing request. A cross-site page
// cannot set it without a CORS preflight, which only listed origins pass.
const CSRFHeader = "X-Qurylys-Csrf"

// CSRF adds to SameSite=Lax cookies: a required custom header plus an Origin check.
func CSRF(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		ok := c.GetHeader(CSRFHeader) == "1"
		if origin := c.GetHeader("Origin"); origin != "" {
			u, err := url.Parse(origin)
			ok = ok && err == nil && u.Host != "" && (u.Host == c.Request.Host || slices.Contains(allowed, origin))
		} else if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			ok = false
		}
		if !ok {
			httpx.Error(c, http.StatusForbidden, "csrf_rejected", "Request origin could not be verified")
			return
		}
		c.Next()
	}
}
