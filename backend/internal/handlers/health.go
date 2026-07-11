package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type HealthController struct {
	db DatabasePinger
}

func NewHealthController(db DatabasePinger) *HealthController {
	return &HealthController{db: db}
}

func (c *HealthController) GetHealth(ctx *gin.Context) {
	pingCtx, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
	defer cancel()

	if err := c.db.PingContext(pingCtx); err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
