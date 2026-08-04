package router

import (
	"log/slog"

	"github.com/gin-contrib/requestid"
	ginslog "github.com/gin-contrib/slog"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/middleware"
)

type Dependencies struct {
	HealthController     *handlers.HealthController
	IngredientController *handlers.IngredientController
}

func New(deps Dependencies) *gin.Engine {
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
	v1.GET("/ingredients", deps.IngredientController.ListIngredients)
	v1.GET("/ingredients/:id", deps.IngredientController.GetIngredient)
	v1.GET("/ingredients/:id/icon", deps.IngredientController.GetIngredientIcon)
	v1.POST("/ingredients", deps.IngredientController.CreateIngredient)
	v1.PATCH("/ingredients/:id", deps.IngredientController.UpdateIngredient)
	v1.DELETE("/ingredients/:id", deps.IngredientController.DeleteIngredient)
	v1.PUT("/ingredients/:id/icon", deps.IngredientController.SetIngredientIcon)

	return r
}
