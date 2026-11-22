// Package httpx holds the JSON error envelope shared by all HTTP handlers.
//
// Every error response has the shape:
//
//	{"error": {"code": "machine_readable_code", "message": "human readable text"}}
package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": ErrorBody{Code: code, Message: message}})
}

func BadRequest(c *gin.Context, code, message string) {
	Error(c, http.StatusBadRequest, code, message)
}

func NotFound(c *gin.Context, code, message string) {
	Error(c, http.StatusNotFound, code, message)
}

// Internal logs the real error and returns a generic message, so SQL details
// never leak to clients.
func Internal(c *gin.Context, log *slog.Logger, err error) {
	log.ErrorContext(c.Request.Context(), "request failed",
		"method", c.Request.Method, "path", c.FullPath(), "error", err)
	Error(c, http.StatusInternalServerError, "internal_error", "Internal server error")
}
