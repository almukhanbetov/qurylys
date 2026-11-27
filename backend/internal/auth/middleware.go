package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

const principalKey = "auth.principal"

// CookieName uses the __Host- prefix in production: Secure, Path=/, no Domain.
func CookieName(production bool) string {
	if production {
		return "__Host-qurylys_session"
	}
	return "qurylys_session"
}

func (h *Handler) setCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieName(h.production), Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.production, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) token(c *gin.Context) string {
	v, _ := c.Cookie(CookieName(h.production))
	return v
}

// RequireUser rejects requests without a live session.
func (h *Handler) RequireUser(c *gin.Context) {
	token := h.token(c)
	if token == "" {
		httpx.Fail(c, h.log, errUnauthorized)
		return
	}
	p, err := h.svc.repo.principal(c.Request.Context(), hashToken(token))
	if err != nil {
		httpx.Fail(c, h.log, err)
		return
	}
	if p == nil {
		h.setCookie(c, "", -1)
		httpx.Fail(c, h.log, errUnauthorized)
		return
	}
	c.Set(principalKey, *p)
	c.Next()
}

// RequireRole allows only holders of a platform role. Use after RequireUser.
func (h *Handler) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Current(c).Has(role) {
			httpx.Fail(c, h.log, errForbidden)
			return
		}
		c.Next()
	}
}

// RequirePermission allows only holders of a permission. Use after RequireUser.
func (h *Handler) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Current(c).Can(permission) {
			httpx.Fail(c, h.log, errForbidden)
			return
		}
		c.Next()
	}
}

// Current returns the principal set by RequireUser (zero value otherwise).
func Current(c *gin.Context) Principal {
	p, _ := c.Get(principalKey)
	v, _ := p.(Principal)
	return v
}
