// Package router wires HTTP routes to module handlers.
package router

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/categories"
	"github.com/qurylys/qurylys/backend/internal/config"
	"github.com/qurylys/qurylys/backend/internal/health"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/middleware"
	"github.com/qurylys/qurylys/backend/internal/offers"
	"github.com/qurylys/qurylys/backend/internal/products"
)

func New(cfg config.Config, db *sql.DB, log *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Nginx is the only proxy in front of the API (private Docker network).
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	r.Use(middleware.Recovery(log), middleware.RequestLogger(log), middleware.CORS(cfg.CORSAllowedOrigins))

	r.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, "route_not_found", "Route not found")
	})

	v1 := r.Group("/api/v1")
	health.NewHandler(db, log).Register(v1)
	categories.NewHandler(categories.NewService(categories.NewRepository(db)), log).Register(v1)
	products.NewHandler(products.NewService(products.NewRepository(db)), log).Register(v1)
	offers.NewHandler(offers.NewService(offers.NewRepository(db)), log).Register(v1)

	return r
}
