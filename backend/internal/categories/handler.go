package categories

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/auth"
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

// RegisterAdmin: holders of catalog.categories manage the whole tree.
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup, requireUser, requireManager gin.HandlerFunc) {
	g := rg.Group("/admin/categories", httpx.NoStore, requireUser, requireManager)
	g.GET("", h.list)
	g.POST("", h.create)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.remove)
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

func (h *Handler) list(c *gin.Context) {
	v, err := h.svc.repo.ListAll(c.Request.Context())
	h.respond(c, http.StatusOK, v, err)
}

func (h *Handler) create(c *gin.Context) {
	var b Input
	if !httpx.BindJSON(c, &b) {
		return
	}
	id, err := h.svc.Create(c.Request.Context(), auth.Current(c), b)
	h.respond(c, http.StatusCreated, gin.H{"id": id}, err)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Version int `json:"version"`
		Input
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Update(c.Request.Context(), auth.Current(c), id, b.Version, b.Input))
}

func (h *Handler) remove(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Version int `json:"version"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.repo.Delete(c.Request.Context(), auth.Current(c), id, b.Version))
}

func (h *Handler) tree(c *gin.Context) {
	tree, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		httpx.Internal(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}
