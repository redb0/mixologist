package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"

	"github.com/redb0/mixologist/internal/httperr"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := requestid.Get(c)
				slog.Error("panic recovered", "err", err, "request_id", requestID)
				c.Header(RequestIDHeader, requestID)
				httperr.Abort(
					c,
					http.StatusInternalServerError,
					httperr.CodeInternalError,
					httperr.InternalServerErrorMessage,
				)
			}
		}()
		c.Next()
	}
}
