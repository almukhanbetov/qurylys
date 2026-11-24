package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AppError is an expected failure that is safe to show to the client.
type AppError struct {
	Status     int
	Code       string
	Message    string
	RetryAfter int // seconds, for 429
}

func (e *AppError) Error() string { return e.Code }

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// Invalid is a 400 for one input field; the code is "invalid_<field>".
func Invalid(field, message string) *AppError {
	return NewError(http.StatusBadRequest, "invalid_"+field, field+": "+message)
}

// Fail writes err as an AppError, or as a logged generic 500.
func Fail(c *gin.Context, log *slog.Logger, err error) {
	var ae *AppError
	if !errors.As(err, &ae) {
		Internal(c, log, err)
		return
	}
	if ae.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(ae.RetryAfter))
	}
	Error(c, ae.Status, ae.Code, ae.Message)
}

const maxBody = 64 << 10

// BindJSON decodes a small JSON body and rejects unknown fields, so a client
// cannot smuggle in attributes such as roles. It reports whether it succeeded.
func BindJSON(c *gin.Context, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		BadRequest(c, "invalid_body", "Request body must be a valid JSON object")
		return false
	}
	return true
}

// PathID parses a positive integer path parameter.
func PathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 1 {
		BadRequest(c, "invalid_id", name+": must be a positive integer")
		return 0, false
	}
	return id, true
}

// NoStore keeps personal responses out of every cache.
func NoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Next()
}
