package offers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/products/:id/offers", h.listForProduct)
}

func (h *Handler) listForProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.BadRequest(c, "invalid_id", "id: must be a positive integer")
		return
	}
	list, err := h.svc.ForProduct(c.Request.Context(), id)
	switch {
	case errors.Is(err, ErrProductNotFound):
		httpx.NotFound(c, "product_not_found", "Product not found")
		return
	case err != nil:
		httpx.Internal(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}
