package router

import (
	"log/slog"
	"strings"

	"github.com/gin-contrib/requestid"
	ginslog "github.com/gin-contrib/slog"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/middleware"
	"github.com/redb0/mixologist/internal/services"
)

type Dependencies struct {
	HealthController     *handlers.HealthController
	IngredientController *handlers.IngredientController
	AuthController       *handlers.AuthController
	AuthService          services.AuthService
	AuthConfig           config.AuthConfig
}

func New(deps Dependencies) *gin.Engine {
	validateDependencies(deps)

	r := gin.New()
	r.Use(
		requestid.New(
			requestid.WithCustomHeaderStrKey(middleware.RequestIDHeader),
		),
		ginslog.SetLogger(
			ginslog.WithLogger(
				func(c *gin.Context, l *slog.Logger) *slog.Logger {
					return l.With("request_id", requestid.Get(c))
				},
			),
		),
		middleware.Recovery(),
	)

	r.GET("/health", deps.HealthController.GetHealth)

	v1 := r.Group("/api/v1")
	requireAuth := middleware.RequireAuth(deps.AuthService, deps.AuthConfig, nil)
	requireAdmin := middleware.RequireRole(domain.UserRoleAdmin)
	requireCSRF := middleware.RequireCSRF(deps.AuthConfig, nil)

	if deps.AuthController != nil {
		v1.GET("/auth/google/login", deps.AuthController.StartGoogleLogin)
		v1.GET("/auth/google/callback", deps.AuthController.HandleGoogleCallback)
		v1.GET("/auth/me", requireAuth, deps.AuthController.GetCurrentUser)
		v1.POST("/auth/logout", requireAuth, requireCSRF, deps.AuthController.Logout)
	}

	v1.GET("/ingredients", deps.IngredientController.ListIngredients)
	v1.GET("/ingredients/:id", deps.IngredientController.GetIngredient)
	v1.GET("/ingredients/:id/icon", deps.IngredientController.GetIngredientIcon)

	v1.POST("/ingredients", requireAuth, requireAdmin, requireCSRF, deps.IngredientController.CreateIngredient)
	v1.PATCH("/ingredients/:id", requireAuth, requireAdmin, requireCSRF, deps.IngredientController.UpdateIngredient)
	v1.DELETE("/ingredients/:id", requireAuth, requireAdmin, requireCSRF, deps.IngredientController.DeleteIngredient)
	v1.PUT("/ingredients/:id/icon", requireAuth, requireAdmin, requireCSRF, deps.IngredientController.SetIngredientIcon)

	return r
}

func validateDependencies(deps Dependencies) {
	mustNotBeNil("HealthController", deps.HealthController)
	mustNotBeNil("IngredientController", deps.IngredientController)
	mustNotBeNil("AuthService", deps.AuthService)

	if strings.TrimSpace(deps.AuthConfig.SessionCookieName) == "" {
		panic("router dependency AuthConfig.SessionCookieName is required")
	}
	if strings.TrimSpace(deps.AuthConfig.CSRFHeaderName) == "" {
		panic("router dependency AuthConfig.CSRFHeaderName is required")
	}
}

func mustNotBeNil(name string, dep any) {
	if dep == nil {
		panic("router dependency " + name + " is required")
	}
}
