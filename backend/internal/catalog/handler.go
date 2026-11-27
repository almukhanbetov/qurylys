package catalog

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireModerator gin.HandlerFunc) {
	g := rg.Group("/companies/:id/catalog-requests", httpx.NoStore, requireUser)
	g.GET("", h.companyList)
	g.POST("", h.create)
	g.GET("/:requestId", h.companyGet)
	g.PUT("/:requestId", h.update)
	g.POST("/:requestId/submit", h.submit)

	m := rg.Group("/catalog/moderation/requests", httpx.NoStore, requireUser, requireModerator)
	m.GET("", h.list)
	m.GET("/:id", h.review)
	m.POST("/:id/gtin", h.confirmGTIN)
	m.POST("/:id/decision", h.decide)
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

func ids(c *gin.Context) (company, request int64, ok bool) {
	if company, ok = httpx.PathID(c, "id"); !ok {
		return
	}
	request, ok = httpx.PathID(c, "requestId")
	return
}

func (h *Handler) companyList(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.companyList(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) companyGet(c *gin.Context) {
	if company, id, ok := ids(c); ok {
		v, err := h.svc.repo.companyGet(c.Request.Context(), auth.Current(c), company, id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) create(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var f Fields
	if !ok || !httpx.BindJSON(c, &f) {
		return
	}
	newID, err := h.svc.Create(c.Request.Context(), auth.Current(c), id, f)
	h.respond(c, http.StatusCreated, gin.H{"id": newID}, err)
}

func (h *Handler) update(c *gin.Context) {
	company, id, ok := ids(c)
	var f Fields
	if !ok || !httpx.BindJSON(c, &f) {
		return
	}
	h.respond(c, 0, nil, h.svc.Update(c.Request.Context(), auth.Current(c), company, id, f))
}

func (h *Handler) submit(c *gin.Context) {
	if company, id, ok := ids(c); ok {
		h.respond(c, 0, nil, h.svc.repo.submit(c.Request.Context(), auth.Current(c), company, id))
	}
}

func (h *Handler) list(c *gin.Context) {
	status := c.Query("status")
	if status != "" && !slices.Contains([]string{StatusInReview, StatusNeedsChanges, StatusApproved, StatusRejected}, status) {
		httpx.BadRequest(c, "invalid_status", "status: unknown")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 10000 {
		httpx.BadRequest(c, "invalid_page", "page: must be 1-10000")
		return
	}
	v, err := h.svc.repo.list(c.Request.Context(), auth.Current(c), status, page)
	h.respond(c, http.StatusOK, v, err)
}

func (h *Handler) review(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.review(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) confirmGTIN(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		SubmissionNo int    `json:"submission_no"`
		SourceKind   string `json:"source_kind"`
		Source       string `json:"source"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.ConfirmGTIN(c.Request.Context(), auth.Current(c), id, b.SubmissionNo, b.SourceKind, b.Source))
}

func (h *Handler) decide(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		SubmissionNo int    `json:"submission_no"`
		Decision     string `json:"decision"`
		Comment      string `json:"comment"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Decide(c.Request.Context(), auth.Current(c), id, b.SubmissionNo, b.Decision, b.Comment))
}
