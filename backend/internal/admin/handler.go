package admin

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	repo *Repository
	log  *slog.Logger
}

func NewHandler(repo *Repository, log *slog.Logger) *Handler { return &Handler{repo: repo, log: log} }

// Register: administrators only. Grants are changed by package auth.
func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireAdmin gin.HandlerFunc) {
	g := rg.Group("/admin", httpx.NoStore, requireUser, requireAdmin)
	g.GET("/users", h.users)
	g.GET("/users/:id", h.user)
	g.GET("/companies", h.companies)
	g.GET("/companies/:id/members", h.companyMembers)
}

func (h *Handler) respond(c *gin.Context, data any, err error) {
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func query(c *gin.Context) (string, bool) {
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) > 100 || strings.Contains(q, "\x00") || !utf8.ValidString(q) {
		httpx.BadRequest(c, "invalid_q", "q: invalid")
		return "", false
	}
	return q, true
}

func page(c *gin.Context) (int, bool) {
	raw := c.DefaultQuery("page", "1")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 10000 {
		httpx.BadRequest(c, "invalid_page", "page: must be 1..10000")
		return 0, false
	}
	return n, true
}

func oneOf(c *gin.Context, name string, allowed ...string) (string, bool) {
	v := c.Query(name)
	if v == "" || slices.Contains(allowed, v) {
		return v, true
	}
	httpx.BadRequest(c, "invalid_"+name, name+": unknown value")
	return "", false
}

func (h *Handler) users(c *gin.Context) {
	var f UserFilter
	var ok bool
	if f.Q, ok = query(c); !ok {
		return
	}
	if f.Page, ok = page(c); !ok {
		return
	}
	access := append([]string{"staff", "none", auth.RoleAdmin, auth.RoleMRX}, auth.GrantablePermissions...)
	if f.Access, ok = oneOf(c, "access", access...); !ok {
		return
	}
	if f.Membership, ok = oneOf(c, "membership", "with", "without"); !ok {
		return
	}
	if f.Status, ok = oneOf(c, "status", "active", "blocked"); !ok {
		return
	}
	v, err := h.repo.users(c.Request.Context(), f)
	h.respond(c, v, err)
}

func (h *Handler) user(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.repo.user(c.Request.Context(), id)
		h.respond(c, v, err)
	}
}

func (h *Handler) companies(c *gin.Context) {
	q, ok := query(c)
	if !ok {
		return
	}
	p, ok := page(c)
	if !ok {
		return
	}
	v, err := h.repo.companies(c.Request.Context(), q, p)
	h.respond(c, v, err)
}

func (h *Handler) companyMembers(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.repo.companyMembers(c.Request.Context(), id)
		h.respond(c, v, err)
	}
}
