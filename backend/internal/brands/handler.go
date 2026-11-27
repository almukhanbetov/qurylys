package brands

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Register: holders of catalog.brands manage the directory. Card editors
// (catalog.moderate) may also read it to choose a brand for a card.
func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireManager, requireReader gin.HandlerFunc) {
	rg.GET("/admin/brands", httpx.NoStore, requireUser, requireReader, h.list)
	g := rg.Group("/admin/brands", httpx.NoStore, requireUser, requireManager)
	g.GET("/unlinked", h.legacy)
	g.POST("", h.create)
	g.PUT("/:id", h.rename)
	g.DELETE("/:id", h.remove)
	g.POST("/:id/link", h.link)
}

func (h *Handler) respond(c *gin.Context, status int, data any, err error) {
	switch {
	case err != nil:
		httpx.Fail(c, h.log, err)
	case data == nil:
		c.Status(http.StatusNoContent)
	default:
		c.JSON(status, gin.H{"data": data})
	}
}

func query(c *gin.Context) (string, bool) {
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) > 100 || strings.Contains(q, "\x00") || !utf8.ValidString(q) {
		httpx.BadRequest(c, "invalid_q", "q: invalid")
		return "", false
	}
	return q, true
}

func (h *Handler) list(c *gin.Context) {
	q, ok := query(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 10000 {
		httpx.BadRequest(c, "invalid_page", "page: must be 1..10000")
		return
	}
	v, err := h.svc.repo.list(c.Request.Context(), q, page)
	h.respond(c, http.StatusOK, v, err)
}

func (h *Handler) legacy(c *gin.Context) {
	if q, ok := query(c); ok {
		v, err := h.svc.repo.legacy(c.Request.Context(), q)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) create(c *gin.Context) {
	var b struct {
		Name string `json:"name"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	id, err := h.svc.Create(c.Request.Context(), auth.Current(c), b.Name)
	h.respond(c, http.StatusCreated, gin.H{"id": id}, err)
}

func (h *Handler) rename(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Version int    `json:"version"`
		Name    string `json:"name"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Rename(c.Request.Context(), auth.Current(c), id, b.Version, b.Name))
}

func (h *Handler) remove(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Version int `json:"version"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Delete(c.Request.Context(), auth.Current(c), id, b.Version))
}

func (h *Handler) link(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Text string `json:"text"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	n, err := h.svc.Link(c.Request.Context(), auth.Current(c), id, b.Text)
	h.respond(c, http.StatusOK, gin.H{"linked": n}, err)
}
