// Package middleware contains cross-cutting HTTP middleware.
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger writes one structured line per request.
func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		}
		log.Log(c.Request.Context(), level, "http request",
			"method", c.Request.Method,
			"route", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Recovery turns panics into a JSON 500 and logs them.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, rec any) {
		log.ErrorContext(c.Request.Context(), "panic recovered", "panic", rec, "route", c.FullPath())
		c.AbortWithStatusJSON(500, gin.H{"error": gin.H{"code": "internal_error", "message": "Internal server error"}})
	})
}
