package suppliers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	repo *Repository
	log  *slog.Logger
}

func NewHandler(repo *Repository, log *slog.Logger) *Handler { return &Handler{repo: repo, log: log} }

// Register: public. no-store, so a supplier that lost its verification or
// direction is not served from any cache.
func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/suppliers", httpx.NoStore)
	g.GET("", h.list)
	g.GET("/:id", h.get)
}

func (h *Handler) respond(c *gin.Context, data any, err error) {
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func text(c *gin.Context, name string) (string, bool) {
	v := strings.TrimSpace(c.Query(name))
	if utf8.RuneCountInString(v) > 100 || strings.Contains(v, "\x00") || !utf8.ValidString(v) {
		httpx.BadRequest(c, "invalid_"+name, name+": invalid")
		return "", false
	}
	return v, true
}

func (h *Handler) list(c *gin.Context) {
	q, ok := text(c, "q")
	if !ok {
		return
	}
	city, ok := text(c, "city")
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 10000 {
		httpx.BadRequest(c, "invalid_page", "page: must be 1..10000")
		return
	}
	v, err := h.repo.list(c.Request.Context(), q, city, page)
	h.respond(c, v, err)
}

func (h *Handler) get(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.repo.get(c.Request.Context(), id)
		h.respond(c, v, err)
	}
}
