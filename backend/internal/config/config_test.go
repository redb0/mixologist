package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_success(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("GIN_MODE", "release")
	t.Setenv("LOG_LEVEL", "WARN")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseURL)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "release", cfg.GinMode)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoad_defaults(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GIN_MODE", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "debug", cfg.GinMode)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_trimSpace(t *testing.T) {
	t.Setenv("DB_URL", "  postgres://localhost/db  ")
	t.Setenv("HTTP_ADDR", "  :3000  ")
	t.Setenv("GIN_MODE", "  test  ")
	t.Setenv("LOG_LEVEL", "  ERROR  ")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseURL)
	assert.Equal(t, ":3000", cfg.HTTPAddr)
	assert.Equal(t, "test", cfg.GinMode)
	assert.Equal(t, "error", cfg.LogLevel)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_URL", "")
			t.Setenv("HTTP_ADDR", "")
			t.Setenv("GIN_MODE", "")
			t.Setenv("LOG_LEVEL", "")

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
			t.Setenv("DB_URL", "postgres://localhost/db")
			t.Setenv("GIN_MODE", mode)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, mode, cfg.GinMode)
		})
	}
}

func TestLoad_validLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			t.Setenv("DB_URL", "postgres://localhost/db")
			t.Setenv("LOG_LEVEL", level)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, level, cfg.LogLevel)
		})
	}
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
