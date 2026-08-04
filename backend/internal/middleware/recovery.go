package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

const internalErrorMessage = "Внутренняя ошибка сервера"

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := requestid.Get(c)
				slog.Error("panic recovered", "err", err, "request_id", requestID)
				c.Header(RequestIDHeader, requestID)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":       "INTERNAL_ERROR",
						"message":    internalErrorMessage,
						"request_id": requestID,
					},
				})
			}
		}()
		c.Next()
	}
}
