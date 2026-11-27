package selleroffers

import (
	"log/slog"
	"net/http"
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

func (h *Handler) Register(rg *gin.RouterGroup, requireUser gin.HandlerFunc) {
	g := rg.Group("/companies/:id", httpx.NoStore, requireUser)
	g.GET("/offer-cards", h.cards)
	g.GET("/offers", h.list)
	g.POST("/offers", h.create)
	g.GET("/offers/:offerId", h.get)
	g.PUT("/offers/:offerId", h.update)
	g.POST("/offers/:offerId/publish", func(c *gin.Context) { h.setState(c, StatePublished) })
	g.POST("/offers/:offerId/hide", func(c *gin.Context) { h.setState(c, StateHidden) })
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

func ids(c *gin.Context) (company, offer int64, ok bool) {
	if company, ok = httpx.PathID(c, "id"); !ok {
		return
	}
	offer, ok = httpx.PathID(c, "offerId")
	return
}

func (h *Handler) cards(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if len(q) > 200 || strings.Contains(q, "\x00") || !utf8.ValidString(q) {
		httpx.BadRequest(c, "invalid_q", "q: invalid")
		return
	}
	v, err := h.svc.repo.cards(c.Request.Context(), auth.Current(c), id, q)
	h.respond(c, http.StatusOK, v, err)
}

func (h *Handler) list(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.list(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) get(c *gin.Context) {
	if company, id, ok := ids(c); ok {
		v, err := h.svc.repo.get(c.Request.Context(), auth.Current(c), company, id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) create(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		ProductID int64 `json:"product_id"`
		Input
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	newID, err := h.svc.Create(c.Request.Context(), auth.Current(c), id, b.ProductID, b.Input)
	h.respond(c, http.StatusCreated, gin.H{"id": newID}, err)
}

func (h *Handler) update(c *gin.Context) {
	company, id, ok := ids(c)
	var b struct {
		Version int `json:"version"`
		Input
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Update(c.Request.Context(), auth.Current(c), company, id, b.Version, b.Input))
}

func (h *Handler) setState(c *gin.Context, state string) {
	company, id, ok := ids(c)
	var b struct {
		Version int `json:"version"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.SetState(c.Request.Context(), auth.Current(c), company, id, b.Version, state))
}
