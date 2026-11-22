package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS allows cross-origin requests only from the explicit allowlist. It is used
// for local development, where the Next.js dev server runs on another port.
// Behind the shared Nginx the frontend and API share one origin and the list is empty.
// The API is read-only and uses no cookies at this stage, so credentials are not allowed.
func CORS(allowed []string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" || len(set) == 0 {
			c.Next()
			return
		}
		c.Writer.Header().Add("Vary", "Origin")

		if _, ok := set[strings.TrimRight(origin, "/")]; !ok {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			// No CORS headers: the browser blocks the response.
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Accept-Language")
			h.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
