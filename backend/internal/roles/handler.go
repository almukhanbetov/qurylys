package roles

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	repo *Repository
	log  *slog.Logger
}

func NewHandler(repo *Repository, log *slog.Logger) *Handler { return &Handler{repo: repo, log: log} }

// Register: administrators only, read-only. There is no route that creates,
// renames, deletes or assigns a role of the directory.
func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireAdmin gin.HandlerFunc) {
	rg.GET("/admin/roles", httpx.NoStore, requireUser, requireAdmin, h.list)
}

func (h *Handler) list(c *gin.Context) {
	d, err := h.repo.directory(c.Request.Context())
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}
