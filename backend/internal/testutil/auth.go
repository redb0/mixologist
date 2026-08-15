package testutil

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/middleware"
)

type AuthSessionFactory interface {
	UpsertGoogleUser(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error)
	CreateSession(
		ctx context.Context,
		userID uint,
		ttl time.Duration,
		now time.Time,
		metadata map[string]any,
	) (string, *domain.Session, error)
}

func TestAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		SessionCookieName:   "session",
		SessionCookieSecret: strings.Repeat("a", 32),
		SessionCookieSecure: false,
		SessionTTL:          24 * time.Hour,
		CSRFSecret:          strings.Repeat("b", 32),
		CSRFCookieName:      "csrf_token",
		CSRFHeaderName:      "X-CSRF-Token",
	}
}

func CreateUserSession(
	ctx context.Context,
	authService AuthSessionFactory,
	identity domain.GoogleIdentity,
	now time.Time,
	ttl time.Duration,
) (*domain.User, string, error) {
	user, err := authService.UpsertGoogleUser(ctx, identity, now)
	if err != nil {
		return nil, "", err
	}
	rawToken, _, err := authService.CreateSession(ctx, user.ID, ttl, now, nil)
	if err != nil {
		return nil, "", err
	}
	return user, rawToken, nil
}

func AttachSession(req *http.Request, cfg config.AuthConfig, rawToken string) {
	if req == nil || strings.TrimSpace(rawToken) == "" {
		return
	}
	req.AddCookie(&http.Cookie{
		Name:  cfg.SessionCookieName,
		Value: rawToken,
	})
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return
	default:
		req.Header.Set(
			cfg.CSRFHeaderName,
			middleware.SignCSRFToken(cfg.CSRFSecret, rawToken, time.Now().UTC()),
		)
	}
}
