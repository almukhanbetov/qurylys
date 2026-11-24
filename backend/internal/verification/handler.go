package verification

import (
	"errors"
	"io"
	"log/slog"
	"mime"
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

func (h *Handler) Register(rg *gin.RouterGroup, requireUser, requireMRX gin.HandlerFunc) {
	g := rg.Group("/companies/:id/verification", httpx.NoStore, requireUser)
	g.GET("", h.view)
	g.POST("", h.create)
	g.POST("/submit", h.submit)
	g.POST("/documents", h.upload)
	g.GET("/documents/:docId", h.companyFile)
	g.DELETE("/documents/:docId", h.removeDocument)

	m := rg.Group("/mrx/requests", httpx.NoStore, requireUser, requireMRX)
	m.GET("", h.list)
	m.GET("/:id", h.review)
	m.GET("/:id/documents/:docId", h.reviewFile)
	m.POST("/:id/decision", h.decide)
}

func (h *Handler) respond(c *gin.Context, data any, err error) {
	switch {
	case err != nil:
		httpx.Fail(c, h.log, err)
	case data == nil:
		c.Status(http.StatusNoContent)
	default:
		c.JSON(http.StatusOK, gin.H{"data": data})
	}
}

func (h *Handler) view(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.companyView(c.Request.Context(), auth.Current(c), id)
		h.respond(c, v, err)
	}
}

func (h *Handler) create(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		h.respond(c, nil, h.svc.repo.createDraft(c.Request.Context(), auth.Current(c), id))
	}
}

func (h *Handler) submit(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		h.respond(c, nil, h.svc.Submit(c.Request.Context(), auth.Current(c), id))
	}
}

// upload expects multipart/form-data with one part named "file".
func (h *Handler) upload(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	p, kind := auth.Current(c), c.Query("kind")
	if err := h.svc.PrepareUpload(c.Request.Context(), p, id, kind); err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	name, data, err := readFile(c)
	if err == nil {
		err = h.svc.Upload(c.Request.Context(), p, id, kind, name, data)
	}
	h.respond(c, nil, err)
}

func readFile(c *gin.Context) (string, []byte, error) {
	bad := httpx.NewError(http.StatusBadRequest, "invalid_body", "Expected one file")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileSize+64<<10)
	mr, err := c.Request.MultipartReader()
	if err != nil {
		return "", nil, bad
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "file" {
		return "", nil, bad
	}
	data, err := io.ReadAll(io.LimitReader(part, MaxFileSize+1))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge) || len(data) > MaxFileSize:
		return "", nil, errFileSize
	case err != nil || len(data) == 0:
		return "", nil, bad
	}
	return part.FileName(), data, nil
}

func (h *Handler) sendFile(c *gin.Context, f *file, err error) {
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	// Always a download: uploaded content never renders on this origin.
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(http.StatusOK, f.ContentType, f.Data)
}

func (h *Handler) companyFile(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if doc, ok := httpx.PathID(c, "docId"); ok {
		f, err := h.svc.repo.companyFile(c.Request.Context(), auth.Current(c), id, doc)
		h.sendFile(c, f, err)
	}
}

func (h *Handler) removeDocument(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if doc, ok := httpx.PathID(c, "docId"); ok {
		h.respond(c, nil, h.svc.repo.removeDocument(c.Request.Context(), auth.Current(c), id, doc))
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
	h.respond(c, v, err)
}

func (h *Handler) review(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.reviewView(c.Request.Context(), auth.Current(c), id)
		h.respond(c, v, err)
	}
}

func (h *Handler) reviewFile(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if doc, ok := httpx.PathID(c, "docId"); ok {
		f, err := h.svc.repo.reviewFile(c.Request.Context(), auth.Current(c), id, doc)
		h.sendFile(c, f, err)
	}
}

func (h *Handler) decide(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Decision     string `json:"decision"`
		Comment      string `json:"comment"`
		SubmissionNo int    `json:"submission_no"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, nil, h.svc.Decide(c.Request.Context(), auth.Current(c), id, b.SubmissionNo, b.Decision, b.Comment))
}
