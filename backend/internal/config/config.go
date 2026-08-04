package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	GinMode     string
	LogLevel    string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: strings.TrimSpace(os.Getenv("DB_URL")),
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		GinMode:     envOrDefault("GIN_MODE", "debug"),
		LogLevel:    envOrDefault("LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DB_URL is required")
	}
	if cfg.HTTPAddr == "" {
		return Config{}, errors.New("HTTP_ADDR cannot be empty")
	}

	switch cfg.GinMode {
	case "debug", "release", "test":
	default:
		return Config{}, fmt.Errorf("GIN_MODE must be debug, release or test, got %q", cfg.GinMode)
	}

	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", cfg.LogLevel)
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
