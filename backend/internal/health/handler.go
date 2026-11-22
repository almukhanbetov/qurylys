// Package health exposes liveness (/health) and readiness (/ready) probes.
package health

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db      *sql.DB
	log     *slog.Logger
	started time.Time
}

func NewHandler(db *sql.DB, log *slog.Logger) *Handler {
	return &Handler{db: db, log: log, started: time.Now()}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/health", h.health)
	rg.GET("/ready", h.ready)
}

// health answers as long as the process serves HTTP; it does not touch the DB,
// so a database outage does not make orchestrators restart the API.
func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"uptime_seconds": int(time.Since(h.started).Seconds()),
	})
}

// ready reports whether the API can serve requests that need PostgreSQL.
func (h *Handler) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		h.log.WarnContext(ctx, "readiness check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"checks": gin.H{"database": "down"},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"checks": gin.H{"database": "up"},
	})
}
