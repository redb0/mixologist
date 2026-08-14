package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GIN_MODE", "")
	t.Setenv("LOG_LEVEL", "")
}

func setValidAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "google-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "google-client-secret")
	t.Setenv("GOOGLE_OAUTH_CALLBACK_URL", "http://localhost:8080/api/v1/auth/google/callback")
	t.Setenv("AUTH_ADMIN_EMAILS", "admin@example.com, ops@example.com")
	t.Setenv("SESSION_COOKIE_SECRET", strings.Repeat("a", 32))
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("SESSION_COOKIE_DOMAIN", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("SESSION_TTL", "")
	t.Setenv("CSRF_SECRET", strings.Repeat("b", 32))
	t.Setenv("CSRF_COOKIE_NAME", "")
	t.Setenv("CSRF_HEADER_NAME", "")
}

func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GOOGLE_OAUTH_CLIENT_ID",
		"GOOGLE_OAUTH_CLIENT_SECRET",
		"GOOGLE_OAUTH_CALLBACK_URL",
		"AUTH_ADMIN_EMAILS",
		"SESSION_COOKIE_SECRET",
		"SESSION_COOKIE_NAME",
		"SESSION_COOKIE_DOMAIN",
		"SESSION_COOKIE_SECURE",
		"SESSION_TTL",
		"CSRF_SECRET",
		"CSRF_COOKIE_NAME",
		"CSRF_HEADER_NAME",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_success(t *testing.T) {
	setBaseEnv(t)
	setValidAuthEnv(t)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("GIN_MODE", "release")
	t.Setenv("LOG_LEVEL", "WARN")
	t.Setenv("GOOGLE_OAUTH_CALLBACK_URL", "https://localhost:8080/api/v1/auth/google/callback")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseURL)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "release", cfg.GinMode)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, "google-client-id", cfg.Auth.GoogleClientID)
	assert.Equal(t, "google-client-secret", cfg.Auth.GoogleClientSecret)
	assert.Equal(t, "https://localhost:8080/api/v1/auth/google/callback", cfg.Auth.GoogleCallbackURL)
	assert.Equal(t, []string{"admin@example.com", "ops@example.com"}, cfg.Auth.AdminEmails)
	assert.Equal(t, defaultSessionCookieName, cfg.Auth.SessionCookieName)
	assert.Equal(t, strings.Repeat("a", 32), cfg.Auth.SessionCookieSecret)
	assert.Equal(t, strings.Repeat("b", 32), cfg.Auth.CSRFSecret)
	assert.True(t, cfg.Auth.SessionCookieSecure)
	assert.Equal(t, defaultSessionTTL, cfg.Auth.SessionTTL)
	assert.Equal(t, defaultCSRFCookieName, cfg.Auth.CSRFCookieName)
	assert.Equal(t, defaultCSRFHeaderName, cfg.Auth.CSRFHeaderName)
}

func TestLoad_defaults(t *testing.T) {
	setBaseEnv(t)
	setValidAuthEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "debug", cfg.GinMode)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, defaultSessionCookieName, cfg.Auth.SessionCookieName)
	assert.Equal(t, defaultSessionTTL, cfg.Auth.SessionTTL)
	assert.False(t, cfg.Auth.SessionCookieSecure)
}

func TestLoad_trimSpace(t *testing.T) {
	setBaseEnv(t)
	setValidAuthEnv(t)
	t.Setenv("DB_URL", "  postgres://localhost/db  ")
	t.Setenv("HTTP_ADDR", "  :3000  ")
	t.Setenv("GIN_MODE", "  test  ")
	t.Setenv("LOG_LEVEL", "  ERROR  ")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "  google-client-id  ")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "  google-client-secret  ")
	t.Setenv("GOOGLE_OAUTH_CALLBACK_URL", "  http://localhost:8080/api/v1/auth/google/callback  ")
	t.Setenv("AUTH_ADMIN_EMAILS", "  admin@example.com ,  ops@example.com  ")
	t.Setenv("SESSION_COOKIE_SECRET", "  "+strings.Repeat("a", 32)+"  ")
	t.Setenv("CSRF_SECRET", "  "+strings.Repeat("b", 32)+"  ")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseURL)
	assert.Equal(t, ":3000", cfg.HTTPAddr)
	assert.Equal(t, "test", cfg.GinMode)
	assert.Equal(t, "error", cfg.LogLevel)
	assert.Equal(t, "google-client-id", cfg.Auth.GoogleClientID)
	assert.Equal(t, []string{"admin@example.com", "ops@example.com"}, cfg.Auth.AdminEmails)
	assert.Equal(t, strings.Repeat("a", 32), cfg.Auth.SessionCookieSecret)
	assert.Equal(t, strings.Repeat("b", 32), cfg.Auth.CSRFSecret)
}

func TestLoad_errors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing DB_URL",
			env:     map[string]string{"DB_URL": ""},
			wantErr: "DB_URL is required",
		},
		{
			name:    "whitespace DB_URL",
			env:     map[string]string{"DB_URL": "   "},
			wantErr: "DB_URL is required",
		},
		{
			name:    "invalid GIN_MODE",
			env:     map[string]string{"DB_URL": "postgres://localhost/db", "GIN_MODE": "production"},
			wantErr: "GIN_MODE must be debug, release or test, got \"production\"",
		},
		{
			name:    "invalid LOG_LEVEL",
			env:     map[string]string{"DB_URL": "postgres://localhost/db", "LOG_LEVEL": "trace"},
			wantErr: "LOG_LEVEL must be debug, info, warn or error, got \"trace\"",
		},
		{
			name: "missing GOOGLE_OAUTH_CLIENT_ID",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CLIENT_ID is required",
		},
		{
			name: "missing GOOGLE_OAUTH_CLIENT_SECRET",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CLIENT_SECRET is required",
		},
		{
			name: "missing GOOGLE_OAUTH_CALLBACK_URL",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CALLBACK_URL is required",
		},
		{
			name: "invalid GOOGLE_OAUTH_CALLBACK_URL",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "/relative/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CALLBACK_URL must be an absolute URL, got \"/relative/callback\"",
		},
		{
			name: "invalid GOOGLE_OAUTH_CALLBACK_URL scheme",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "ftp://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CALLBACK_URL must use http or https scheme, got \"ftp://localhost/callback\"",
		},
		{
			name: "http GOOGLE_OAUTH_CALLBACK_URL in release mode",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GIN_MODE":                   "release",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
			},
			wantErr: "GOOGLE_OAUTH_CALLBACK_URL must use https scheme when GIN_MODE is release, got \"http://localhost/callback\"",
		},
		{
			name: "SESSION_COOKIE_SECURE false in release mode",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GIN_MODE":                   "release",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "https://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                strings.Repeat("b", 32),
				"SESSION_COOKIE_SECURE":      "false",
			},
			wantErr: "SESSION_COOKIE_SECURE must be true when GIN_MODE is release",
		},
		{
			name: "missing SESSION_COOKIE_SECRET",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      "",
			},
			wantErr: "SESSION_COOKIE_SECRET is required",
		},
		{
			name: "short SESSION_COOKIE_SECRET",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      "too-short",
			},
			wantErr: "SESSION_COOKIE_SECRET must be at least 32 characters",
		},
		{
			name: "missing CSRF_SECRET",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                "",
			},
			wantErr: "CSRF_SECRET is required",
		},
		{
			name: "short CSRF_SECRET",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                "too-short",
			},
			wantErr: "CSRF_SECRET must be at least 32 characters",
		},
		{
			name: "invalid SESSION_TTL",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                strings.Repeat("b", 32),
				"SESSION_TTL":                "not-a-duration",
			},
			wantErr: "SESSION_TTL must be a positive duration, got \"not-a-duration\"",
		},
		{
			name: "invalid SESSION_COOKIE_SECURE",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                strings.Repeat("b", 32),
				"SESSION_COOKIE_SECURE":      "maybe",
			},
			wantErr: "SESSION_COOKIE_SECURE must be true or false, got \"maybe\"",
		},
		{
			name: "empty AUTH_ADMIN_EMAILS entry",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                strings.Repeat("b", 32),
				"AUTH_ADMIN_EMAILS":          "admin@example.com,,ops@example.com",
			},
			wantErr: "AUTH_ADMIN_EMAILS must not contain empty values",
		},
		{
			name: "invalid AUTH_ADMIN_EMAILS entry",
			env: map[string]string{
				"DB_URL":                     "postgres://localhost/db",
				"GOOGLE_OAUTH_CLIENT_ID":     "client-id",
				"GOOGLE_OAUTH_CLIENT_SECRET": "secret",
				"GOOGLE_OAUTH_CALLBACK_URL":  "http://localhost/callback",
				"SESSION_COOKIE_SECRET":      strings.Repeat("a", 32),
				"CSRF_SECRET":                strings.Repeat("b", 32),
				"AUTH_ADMIN_EMAILS":          "not-an-email",
			},
			wantErr: "AUTH_ADMIN_EMAILS contains invalid email \"not-an-email\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			clearAuthEnv(t)

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			require.Error(t, err)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestLoad_validGinModes(t *testing.T) {
	for _, mode := range []string{"debug", "release", "test"} {
		t.Run(mode, func(t *testing.T) {
			setBaseEnv(t)
			setValidAuthEnv(t)
			t.Setenv("GIN_MODE", mode)
			if mode == "release" {
				t.Setenv("GOOGLE_OAUTH_CALLBACK_URL", "https://localhost:8080/api/v1/auth/google/callback")
			}

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, mode, cfg.GinMode)
			if mode == "release" {
				assert.True(t, cfg.Auth.SessionCookieSecure)
			} else {
				assert.False(t, cfg.Auth.SessionCookieSecure)
			}
		})
	}
}

func TestLoad_validLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			setBaseEnv(t)
			setValidAuthEnv(t)
			t.Setenv("LOG_LEVEL", level)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, level, cfg.LogLevel)
		})
	}
}

func TestLoad_authDefaultsAndOverrides(t *testing.T) {
	setBaseEnv(t)
	setValidAuthEnv(t)
	t.Setenv("SESSION_COOKIE_NAME", "mixologist_session")
	t.Setenv("SESSION_COOKIE_DOMAIN", "localhost")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_TTL", "24h")
	t.Setenv("CSRF_COOKIE_NAME", "mixologist_csrf")
	t.Setenv("CSRF_HEADER_NAME", "X-Mixologist-CSRF")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "mixologist_session", cfg.Auth.SessionCookieName)
	assert.Equal(t, "localhost", cfg.Auth.SessionCookieDomain)
	assert.True(t, cfg.Auth.SessionCookieSecure)
	assert.Equal(t, 24*time.Hour, cfg.Auth.SessionTTL)
	assert.Equal(t, "mixologist_csrf", cfg.Auth.CSRFCookieName)
	assert.Equal(t, "X-Mixologist-CSRF", cfg.Auth.CSRFHeaderName)
}

func TestParseAdminEmails_deduplicatesAndAllowsEmptyList(t *testing.T) {
	emails, err := parseAdminEmails("")
	require.NoError(t, err)
	assert.Nil(t, emails)

	emails, err = parseAdminEmails("Admin@Example.com, admin@example.com, ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"Admin@Example.com", "ops@example.com"}, emails)
}

func TestAuthConfig_IsAdminEmail(t *testing.T) {
	cfg := AuthConfig{
		AdminEmails: []string{"Admin@Example.com"},
	}

	assert.True(t, cfg.IsAdminEmail("admin@example.com"))
	assert.False(t, cfg.IsAdminEmail("user@example.com"))
}

func TestEnvOrDefault(t *testing.T) {
	const key = "CONFIG_TEST_ENV_OR_DEFAULT"

	t.Setenv(key, "custom")
	assert.Equal(t, "custom", envOrDefault(key, "fallback"))

	t.Setenv(key, "")
	assert.Equal(t, "fallback", envOrDefault(key, "fallback"))

	t.Setenv(key, "  ")
	assert.Equal(t, "fallback", envOrDefault(key, "fallback"))

	t.Setenv(key, "  trimmed  ")
	assert.Equal(t, "trimmed", envOrDefault(key, "fallback"))
}
