package categories

import (
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
	rg.GET("/categories/tree", h.tree)
}

func (h *Handler) tree(c *gin.Context) {
	tree, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		httpx.Internal(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}
