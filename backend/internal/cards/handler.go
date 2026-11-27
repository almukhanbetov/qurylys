package cards

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

// Register: holders of catalog.moderate (R11).
func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireEditor gin.HandlerFunc) {
	g := rg.Group("/admin/products", httpx.NoStore, requireUser, requireEditor)
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.POST("/:id/gtin", h.gtin)
	g.POST("/:id/block", func(c *gin.Context) { h.block(c, true) })
	g.POST("/:id/unblock", func(c *gin.Context) { h.block(c, false) })
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

func bad(c *gin.Context, name string) bool {
	httpx.BadRequest(c, "invalid_"+name, name+": invalid")
	return false
}

func positive(raw string) (int64, bool) {
	n, err := strconv.ParseInt(raw, 10, 64)
	return n, err == nil && n > 0
}

func filter(c *gin.Context) (f Filter, ok bool) {
	f.Q = strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(f.Q) > 100 || strings.Contains(f.Q, "\x00") || !utf8.ValidString(f.Q) {
		return f, bad(c, "q")
	}
	if v := c.Query("category_id"); v != "" {
		if f.CategoryID, ok = positive(v); !ok {
			return f, bad(c, "category_id")
		}
	}
	if f.Brand = c.Query("brand"); f.Brand != "" && f.Brand != "none" {
		if _, ok := positive(f.Brand); !ok {
			return f, bad(c, "brand")
		}
	}
	if f.State = c.Query("state"); f.State != "" && stateSQL[f.State] == "" {
		return f, bad(c, "state")
	}
	p, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || p < 1 || p > 10000 {
		return f, bad(c, "page")
	}
	f.Page = p
	return f, true
}

func (h *Handler) list(c *gin.Context) {
	if f, ok := filter(c); ok {
		v, err := h.svc.repo.list(c.Request.Context(), f)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) get(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.get(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) create(c *gin.Context) {
	var b struct {
		Fields
		GTIN GTIN `json:"gtin"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	id, err := h.svc.Create(c.Request.Context(), auth.Current(c), b.Fields, b.GTIN)
	h.respond(c, http.StatusCreated, gin.H{"id": id}, err)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Fields
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Update(c.Request.Context(), auth.Current(c), id, b.Version, b.Fields, b.Reason))
}

func (h *Handler) gtin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		GTIN
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.ConfirmGTIN(c.Request.Context(), auth.Current(c), id, b.Version, b.GTIN, b.Reason))
}

func (h *Handler) block(c *gin.Context, block bool) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.SetBlocked(c.Request.Context(), auth.Current(c), id, b.Version, block, b.Reason))
}
