package products

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/products", h.list)
	rg.GET("/products/:id", h.get)
}

func (h *Handler) list(c *gin.Context) {
	f, err := ParseListParams(RawListParams{
		Q:        c.Query("q"),
		Category: c.Query("category"),
		Sort:     c.Query("sort"),
		Page:     c.Query("page"),
		PageSize: c.Query("page_size"),
	})
	if h.handleError(c, err) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), f)
	if h.handleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": page})
}

func (h *Handler) get(c *gin.Context) {
	id, err := ParseID(c.Param("id"))
	if h.handleError(c, err) {
		return
	}
	d, err := h.svc.Get(c.Request.Context(), id)
	if h.handleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

// handleError writes an error response and reports whether one was written.
func (h *Handler) handleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.BadRequest(c, "invalid_"+ve.Field, ve.Error())
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "product_not_found", "Product not found")
	case errors.Is(err, ErrCategoryNotFound):
		httpx.NotFound(c, "category_not_found", "Category not found")
	default:
		httpx.Internal(c, h.log, err)
	}
	return true
}
