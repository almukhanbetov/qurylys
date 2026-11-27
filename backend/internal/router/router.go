// Package router wires HTTP routes to module handlers.
package router

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qurylys/qurylys/backend/internal/admin"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/brands"
	"github.com/qurylys/qurylys/backend/internal/cards"
	"github.com/qurylys/qurylys/backend/internal/catalog"
	"github.com/qurylys/qurylys/backend/internal/categories"
	"github.com/qurylys/qurylys/backend/internal/companies"
	"github.com/qurylys/qurylys/backend/internal/config"
	"github.com/qurylys/qurylys/backend/internal/delivery"
	"github.com/qurylys/qurylys/backend/internal/health"
	"github.com/qurylys/qurylys/backend/internal/httpx"
	"github.com/qurylys/qurylys/backend/internal/middleware"
	"github.com/qurylys/qurylys/backend/internal/offers"
	"github.com/qurylys/qurylys/backend/internal/products"
	"github.com/qurylys/qurylys/backend/internal/ratelimit"
	"github.com/qurylys/qurylys/backend/internal/roles"
	"github.com/qurylys/qurylys/backend/internal/selleroffers"
	"github.com/qurylys/qurylys/backend/internal/suppliers"
	"github.com/qurylys/qurylys/backend/internal/verification"
)

func New(cfg config.Config, db *sql.DB, log *slog.Logger, sender delivery.Sender) *gin.Engine {
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

	v1 := r.Group("/api/v1", middleware.CSRF(cfg.CORSAllowedOrigins))
	health.NewHandler(db, log).Register(v1)
	categoriesHandler := categories.NewHandler(categories.NewService(categories.NewRepository(db)), log)
	categoriesHandler.Register(v1)
	products.NewHandler(products.NewService(products.NewRepository(db)), log).Register(v1)
	offers.NewHandler(offers.NewService(offers.NewRepository(db)), log).Register(v1)
	suppliers.NewHandler(suppliers.NewRepository(db), log).Register(v1)

	limiter := ratelimit.New(db)
	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(db), sender, limiter, log, cfg.EmailDailyLimit), log, cfg.IsProduction())
	authHandler.Register(v1)
	companies.NewHandler(companies.NewService(companies.NewRepository(db), limiter), log).Register(v1, authHandler.RequireUser)
	verification.NewHandler(verification.NewService(verification.NewRepository(db), limiter, sender, log), log).
		Register(v1, authHandler.RequireUser, authHandler.RequireRole(auth.RoleMRX))

	catalog.NewHandler(catalog.NewService(catalog.NewRepository(db), limiter), log).
		Register(v1, authHandler.RequireUser, authHandler.RequirePermission(auth.PermCatalogModerate))

	selleroffers.NewHandler(selleroffers.NewService(selleroffers.NewRepository(db), limiter), log).
		Register(v1, authHandler.RequireUser)

	// Administration: each section has its own permission; none follows from a role.
	admin.NewHandler(admin.NewRepository(db), log).Register(v1, authHandler.RequireUser, authHandler.RequireRole(auth.RoleAdmin))
	categoriesHandler.RegisterAdmin(v1, authHandler.RequireUser, authHandler.RequirePermission(auth.PermCatalogCategories))
	brandsHandler := brands.NewHandler(brands.NewService(brands.NewRepository(db), limiter), log)
	brandsHandler.RegisterPublic(v1)
	brandsHandler.Register(v1, authHandler.RequireUser,
		authHandler.RequirePermission(auth.PermCatalogBrands),
		authHandler.RequireAnyPermission(auth.PermCatalogBrands, auth.PermCatalogModerate))
	roles.NewHandler(roles.NewRepository(db), log).Register(v1, authHandler.RequireUser, authHandler.RequireRole(auth.RoleAdmin))
	cards.NewHandler(cards.NewService(cards.NewRepository(db), limiter), log).
		Register(v1, authHandler.RequireUser, authHandler.RequirePermission(auth.PermCatalogModerate))

	return r
}
