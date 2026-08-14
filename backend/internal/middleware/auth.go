package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
)

const (
	currentUserKey      = "auth.user"
	currentUserIDKey    = "auth.user_id"
	currentUserEmailKey = "auth.email"
	currentUserRoleKey  = "auth.role"

	msgAuthRequired = "требуется аутентификация"
	msgForbidden    = "недостаточно прав"
)

type SessionUserLookup interface {
	GetUserBySessionToken(ctx context.Context, rawToken string, now time.Time) (*domain.User, error)
}

func RequireAuth(lookup SessionUserLookup, cfg config.AuthConfig, now func() time.Time) gin.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(c *gin.Context) {
		rawToken, err := c.Cookie(cfg.SessionCookieName)
		if err != nil || strings.TrimSpace(rawToken) == "" {
			httperr.AbortError(c, domain.NewErrUnauthorized(msgAuthRequired))
			return
		}

		user, err := lookup.GetUserBySessionToken(c.Request.Context(), rawToken, now().UTC())
		if err != nil {
			httperr.AbortError(c, err)
			return
		}
		if user == nil {
			httperr.AbortError(c, domain.NewErrUnauthorized(msgAuthRequired))
			return
		}

		setCurrentUser(c, user)
		SetCSRFCookie(c, cfg, strings.TrimSpace(rawToken), now().UTC())
		c.Next()
	}
}

func RequireRole(role domain.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			httperr.AbortError(c, domain.NewErrUnauthorized(msgAuthRequired))
			return
		}
		if user.Role != role {
			httperr.AbortError(c, domain.NewErrForbidden(msgForbidden))
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*domain.User, bool) {
	value, exists := c.Get(currentUserKey)
	if !exists {
		return nil, false
	}
	user, ok := value.(*domain.User)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

func CurrentUserID(c *gin.Context) (uint, bool) {
	user, ok := CurrentUser(c)
	if !ok {
		return 0, false
	}
	return user.ID, true
}

func CurrentRole(c *gin.Context) (domain.UserRole, bool) {
	user, ok := CurrentUser(c)
	if !ok {
		return "", false
	}
	return user.Role, true
}

func setCurrentUser(c *gin.Context, user *domain.User) {
	c.Set(currentUserKey, user)
	c.Set(currentUserIDKey, user.ID)
	c.Set(currentUserEmailKey, user.Email)
	c.Set(currentUserRoleKey, user.Role)
}
