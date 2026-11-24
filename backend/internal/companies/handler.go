package companies

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

func (h *Handler) Register(rg *gin.RouterGroup, requireUser gin.HandlerFunc) {
	g := rg.Group("/companies", httpx.NoStore, requireUser)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.GET("/:id/members", h.members)
	g.PATCH("/:id/members/:userId", h.setRole)
	g.DELETE("/:id/members/:userId", h.removeMember)
	g.GET("/:id/invitations", h.invitations)
	g.POST("/:id/invitations", h.invite)
	g.DELETE("/:id/invitations/:invitationId", h.cancelInvitation)

	m := rg.Group("/me/invitations", httpx.NoStore, requireUser)
	m.GET("", h.myInvitations)
	m.POST("/:id/accept", func(c *gin.Context) { h.answer(c, true) })
	m.POST("/:id/decline", func(c *gin.Context) { h.answer(c, false) })
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

func (h *Handler) create(c *gin.Context) {
	var d Details
	if !httpx.BindJSON(c, &d) {
		return
	}
	id, err := h.svc.Create(c.Request.Context(), auth.Current(c), d)
	h.respond(c, http.StatusCreated, gin.H{"id": id}, err)
}

func (h *Handler) get(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.get(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) update(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var d Details
	if !ok || !httpx.BindJSON(c, &d) {
		return
	}
	h.respond(c, 0, nil, h.svc.Update(c.Request.Context(), auth.Current(c), id, d))
}

func (h *Handler) members(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.members(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) setRole(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	userID, ok := httpx.PathID(c, "userId")
	var b struct {
		Role string `json:"role"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.SetRole(c.Request.Context(), auth.Current(c), id, userID, b.Role))
}

func (h *Handler) removeMember(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if userID, ok := httpx.PathID(c, "userId"); ok {
		h.respond(c, 0, nil, h.svc.repo.removeMember(c.Request.Context(), auth.Current(c), id, userID))
	}
}

func (h *Handler) invitations(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		v, err := h.svc.repo.invitations(c.Request.Context(), auth.Current(c), id)
		h.respond(c, http.StatusOK, v, err)
	}
}

func (h *Handler) invite(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Contact string `json:"contact"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Invite(c.Request.Context(), auth.Current(c), id, b.Contact))
}

func (h *Handler) cancelInvitation(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if inv, ok := httpx.PathID(c, "invitationId"); ok {
		h.respond(c, 0, nil, h.svc.repo.cancelInvitation(c.Request.Context(), auth.Current(c), id, inv))
	}
}

func (h *Handler) myInvitations(c *gin.Context) {
	v, err := h.svc.repo.myInvitations(c.Request.Context(), auth.Current(c).UserID)
	h.respond(c, http.StatusOK, v, err)
}

func (h *Handler) answer(c *gin.Context, accept bool) {
	if id, ok := httpx.PathID(c, "id"); ok {
		h.respond(c, 0, nil, h.svc.repo.answerInvitation(c.Request.Context(), auth.Current(c), id, accept))
	}
}
