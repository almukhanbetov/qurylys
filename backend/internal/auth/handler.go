package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/validate"
)

type Handler struct {
	svc        *Service
	log        *slog.Logger
	production bool
}

func NewHandler(svc *Service, log *slog.Logger, production bool) *Handler {
	return &Handler{svc: svc, log: log, production: production}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	a := rg.Group("/auth", httpx.NoStore)
	a.GET("/options", h.options)
	a.POST("/register", h.register)
	a.POST("/register/confirm", h.registerConfirm)
	a.POST("/login", h.login)
	a.POST("/logout", h.logout)
	a.POST("/password/forgot", h.forgot)
	a.POST("/password/reset", h.reset)

	m := rg.Group("/me", httpx.NoStore, h.RequireUser)
	m.GET("", h.me)
	m.PATCH("", h.updateMe)
	m.POST("/password", h.changePassword)
	m.POST("/contacts", h.startContact)
	m.POST("/contacts/confirm", h.confirmContact)
	m.GET("/sessions", h.listSessions)
	m.DELETE("/sessions/:id", h.deleteSession)
	m.POST("/sessions/revoke-others", h.revokeOthers)
	m.PUT("/active-company", h.setActive)

	ad := rg.Group("/admin", httpx.NoStore, h.RequireUser, h.RequireRole(RoleAdmin))
	ad.GET("/users", h.adminUsers)
	ad.PUT("/users/:id/mrx", h.adminSetMRX)
}

// respond writes the error, or data with the given status (204 when data is nil).
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

func (h *Handler) options(c *gin.Context) {
	h.respond(c, http.StatusOK, gin.H{
		"email": h.svc.sender.Available(validate.Email),
		"phone": h.svc.sender.Available(validate.Phone),
	}, nil)
}

// locale is the interface language chosen by the visitor (frontend cookie).
func locale(c *gin.Context) string {
	if v, _ := c.Cookie("qurylys_locale"); v == "ru" {
		return v
	}
	return "kk"
}

func (h *Handler) challenge(c *gin.Context, token string, err error) {
	h.respond(c, http.StatusAccepted, gin.H{"challenge": token}, err)
}

func (h *Handler) session(c *gin.Context, token string, err error) {
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	// Signing in replaces the session of this browser.
	if old := h.token(c); old != "" {
		if err := h.svc.repo.deleteSessionByToken(c.Request.Context(), hashToken(old)); err != nil {
			httpx.Fail(c, h.log, err)
			return
		}
	}
	h.setCookie(c, token, int(sessionLifetime.Seconds()))
	c.Status(http.StatusNoContent)
}

func (h *Handler) register(c *gin.Context) {
	var b struct {
		FullName string `json:"full_name"`
		Contact  string `json:"contact"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	token, err := h.svc.StartRegistration(c.Request.Context(), b.FullName, b.Contact, b.Password, c.ClientIP(), locale(c))
	h.challenge(c, token, err)
}

type codeBody struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

func (h *Handler) registerConfirm(c *gin.Context) {
	var b codeBody
	if !httpx.BindJSON(c, &b) {
		return
	}
	token, err := h.svc.ConfirmRegistration(c.Request.Context(), b.Challenge, b.Code, c.ClientIP(), c.Request.UserAgent())
	h.session(c, token, err)
}

func (h *Handler) login(c *gin.Context) {
	var b struct {
		Contact  string `json:"contact"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	token, err := h.svc.Login(c.Request.Context(), b.Contact, b.Password, c.ClientIP(), c.Request.UserAgent())
	h.session(c, token, err)
}

func (h *Handler) logout(c *gin.Context) {
	if t := h.token(c); t != "" {
		if err := h.svc.repo.deleteSessionByToken(c.Request.Context(), hashToken(t)); err != nil {
			httpx.Fail(c, h.log, err)
			return
		}
	}
	h.setCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

func (h *Handler) forgot(c *gin.Context) {
	var b struct {
		Contact string `json:"contact"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	token, err := h.svc.StartReset(c.Request.Context(), b.Contact, c.ClientIP(), locale(c))
	h.challenge(c, token, err)
}

func (h *Handler) reset(c *gin.Context) {
	var b struct {
		codeBody
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.Reset(c.Request.Context(), b.Challenge, b.Code, b.Password, c.ClientIP()))
}

func (h *Handler) me(c *gin.Context) {
	m, err := h.svc.repo.me(c.Request.Context(), Current(c))
	h.respond(c, http.StatusOK, m, err)
}

func (h *Handler) updateMe(c *gin.Context) {
	var b struct {
		FullName string `json:"full_name"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	name, err := validate.Text("full_name", b.FullName, 120)
	if err == nil {
		err = h.svc.repo.setFullName(c.Request.Context(), Current(c).UserID, name)
	}
	h.respond(c, 0, nil, err)
}

func (h *Handler) changePassword(c *gin.Context) {
	var b struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.ChangePassword(c.Request.Context(), Current(c), b.Current, b.Password))
}

func (h *Handler) startContact(c *gin.Context) {
	var b struct {
		Contact string `json:"contact"`
		Current string `json:"current_password"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	token, err := h.svc.StartContact(c.Request.Context(), Current(c), b.Contact, b.Current, c.ClientIP(), locale(c))
	h.challenge(c, token, err)
}

func (h *Handler) confirmContact(c *gin.Context) {
	var b codeBody
	if !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.ConfirmContact(c.Request.Context(), Current(c), b.Challenge, b.Code, c.ClientIP()))
}

func (h *Handler) listSessions(c *gin.Context) {
	list, err := h.svc.repo.sessions(c.Request.Context(), Current(c))
	h.respond(c, http.StatusOK, list, err)
}

func (h *Handler) deleteSession(c *gin.Context) {
	if id, ok := httpx.PathID(c, "id"); ok {
		h.respond(c, 0, nil, h.svc.repo.deleteSession(c.Request.Context(), Current(c).UserID, id))
	}
}

func (h *Handler) revokeOthers(c *gin.Context) {
	h.respond(c, 0, nil, h.svc.repo.deleteOtherSessions(c.Request.Context(), Current(c)))
}

func (h *Handler) setActive(c *gin.Context) {
	var b struct {
		CompanyID int64 `json:"company_id"`
	}
	if !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.repo.setActiveCompany(c.Request.Context(), Current(c), b.CompanyID))
}

func (h *Handler) adminUsers(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) > 200 || strings.Contains(q, "\x00") || strings.ToValidUTF8(q, "") != q {
		httpx.BadRequest(c, "invalid_q", "q: invalid")
		return
	}
	list, err := h.svc.repo.listUsers(c.Request.Context(), q)
	h.respond(c, http.StatusOK, list, err)
}

func (h *Handler) adminSetMRX(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	var b struct {
		Enabled bool `json:"enabled"`
	}
	if !ok || !httpx.BindJSON(c, &b) {
		return
	}
	h.respond(c, 0, nil, h.svc.repo.setRole(c.Request.Context(), Current(c).UserID, id, RoleMRX, b.Enabled))
}
