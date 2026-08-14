package router

import (
	"strings"
	"testing"
	"time"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/services"
)

func TestNew_RequiresDependencies(t *testing.T) {
	tests := []struct {
		name string
		deps Dependencies
	}{
		{
			name: "missing auth service",
			deps: Dependencies{
				HealthController:     &handlers.HealthController{},
				IngredientController: &handlers.IngredientController{},
				AuthConfig:           testAuthConfig(),
			},
		},
		{
			name: "missing session cookie name",
			deps: Dependencies{
				HealthController:     &handlers.HealthController{},
				IngredientController: &handlers.IngredientController{},
				AuthService:          services.NewAuthService(nil, nil, nil, nil, ""),
				AuthConfig: config.AuthConfig{
					CSRFHeaderName: "X-CSRF-Token",
				},
			},
		},
		{
			name: "missing csrf header",
			deps: Dependencies{
				HealthController:     &handlers.HealthController{},
				IngredientController: &handlers.IngredientController{},
				AuthService:          services.NewAuthService(nil, nil, nil, nil, ""),
				AuthConfig: config.AuthConfig{
					SessionCookieName: "session",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			_ = New(tt.deps)
		})
	}
}

func TestNew_BuildsRouterWhenDependenciesAreValid(t *testing.T) {
	deps := Dependencies{
		HealthController:     &handlers.HealthController{},
		IngredientController: &handlers.IngredientController{},
		AuthService:          services.NewAuthService(nil, nil, nil, nil, ""),
		AuthConfig:           testAuthConfig(),
	}

	if New(deps) == nil {
		t.Fatal("expected router instance")
	}
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		SessionCookieName:   "session",
		SessionCookieSecret: strings.Repeat("a", 32),
		CSRFHeaderName:      "X-CSRF-Token",
		CSRFCookieName:      "csrf_token",
		CSRFSecret:          strings.Repeat("b", 32),
		SessionTTL:          time.Hour,
	}
}
