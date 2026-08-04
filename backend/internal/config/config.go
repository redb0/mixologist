package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	minSessionCookieSecretLen = 32
	defaultSessionCookieName  = "session"
	defaultSessionTTLRaw      = "168h"
	defaultSessionTTL         = 168 * time.Hour
	defaultCSRFCookieName     = "csrf_token"
	defaultCSRFHeaderName     = "X-CSRF-Token"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	GinMode     string
	LogLevel    string
	Auth        AuthConfig
}

type AuthConfig struct {
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleCallbackURL   string
	AdminEmails         []string
	SessionCookieName   string
	SessionCookieSecret string
	SessionCookieDomain string
	SessionCookieSecure bool
	SessionTTL          time.Duration
	CSRFCookieName      string
	CSRFHeaderName      string
}

func (a AuthConfig) IsAdminEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	for _, adminEmail := range a.AdminEmails {
		if strings.ToLower(adminEmail) == normalized {
			return true
		}
	}
	return false
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

	authCfg, err := loadAuthConfig(cfg.GinMode)
	if err != nil {
		return Config{}, err
	}
	cfg.Auth = authCfg

	return cfg, nil
}

func loadAuthConfig(ginMode string) (AuthConfig, error) {
	cfg := AuthConfig{
		GoogleClientID:      strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")),
		GoogleClientSecret:  strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")),
		GoogleCallbackURL:   strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CALLBACK_URL")),
		SessionCookieName:   envOrDefault("SESSION_COOKIE_NAME", defaultSessionCookieName),
		SessionCookieSecret: strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECRET")),
		SessionCookieDomain: strings.TrimSpace(os.Getenv("SESSION_COOKIE_DOMAIN")),
		CSRFCookieName:      envOrDefault("CSRF_COOKIE_NAME", defaultCSRFCookieName),
		CSRFHeaderName:      envOrDefault("CSRF_HEADER_NAME", defaultCSRFHeaderName),
	}

	if cfg.GoogleClientID == "" {
		return AuthConfig{}, errors.New("GOOGLE_OAUTH_CLIENT_ID is required")
	}
	if cfg.GoogleClientSecret == "" {
		return AuthConfig{}, errors.New("GOOGLE_OAUTH_CLIENT_SECRET is required")
	}
	if cfg.GoogleCallbackURL == "" {
		return AuthConfig{}, errors.New("GOOGLE_OAUTH_CALLBACK_URL is required")
	}

	callbackURL, err := url.ParseRequestURI(cfg.GoogleCallbackURL)
	if err != nil || callbackURL.Scheme == "" || callbackURL.Host == "" {
		return AuthConfig{}, fmt.Errorf("GOOGLE_OAUTH_CALLBACK_URL must be an absolute URL, got %q", cfg.GoogleCallbackURL)
	}

	scheme := strings.ToLower(callbackURL.Scheme)
	switch scheme {
	case "http", "https":
	default:
		return AuthConfig{}, fmt.Errorf(
			"GOOGLE_OAUTH_CALLBACK_URL must use http or https scheme, got %q",
			cfg.GoogleCallbackURL,
		)
	}
	if ginMode == "release" && scheme != "https" {
		return AuthConfig{}, fmt.Errorf(
			"GOOGLE_OAUTH_CALLBACK_URL must use https scheme when GIN_MODE is release, got %q",
			cfg.GoogleCallbackURL,
		)
	}

	if cfg.SessionCookieSecret == "" {
		return AuthConfig{}, errors.New("SESSION_COOKIE_SECRET is required")
	}
	if len(cfg.SessionCookieSecret) < minSessionCookieSecretLen {
		return AuthConfig{}, fmt.Errorf(
			"SESSION_COOKIE_SECRET must be at least %d characters",
			minSessionCookieSecretLen,
		)
	}

	sessionTTLRaw := envOrDefault("SESSION_TTL", defaultSessionTTLRaw)
	sessionTTL, err := time.ParseDuration(sessionTTLRaw)
	if err != nil || sessionTTL <= 0 {
		return AuthConfig{}, fmt.Errorf("SESSION_TTL must be a positive duration, got %q", sessionTTLRaw)
	}
	cfg.SessionTTL = sessionTTL

	sessionCookieSecureRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE"))
	switch sessionCookieSecureRaw {
	case "":
		cfg.SessionCookieSecure = ginMode == "release"
	case "1", "true", "TRUE", "True", "yes", "YES":
		cfg.SessionCookieSecure = true
	case "0", "false", "FALSE", "False", "no", "NO":
		if ginMode == "release" {
			return AuthConfig{}, errors.New("SESSION_COOKIE_SECURE must be true when GIN_MODE is release")
		}
		cfg.SessionCookieSecure = false
	default:
		return AuthConfig{}, fmt.Errorf(
			"SESSION_COOKIE_SECURE must be true or false, got %q",
			sessionCookieSecureRaw,
		)
	}

	adminEmails, err := parseAdminEmails(os.Getenv("AUTH_ADMIN_EMAILS"))
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.AdminEmails = adminEmails

	if cfg.SessionCookieName == "" {
		return AuthConfig{}, errors.New("SESSION_COOKIE_NAME cannot be empty")
	}
	if cfg.CSRFCookieName == "" {
		return AuthConfig{}, errors.New("CSRF_COOKIE_NAME cannot be empty")
	}
	if cfg.CSRFHeaderName == "" {
		return AuthConfig{}, errors.New("CSRF_HEADER_NAME cannot be empty")
	}

	return cfg, nil
}

func parseAdminEmails(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	emails := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		email := strings.TrimSpace(part)
		if email == "" {
			return nil, errors.New("AUTH_ADMIN_EMAILS must not contain empty values")
		}

		parsed, err := mail.ParseAddress(email)
		if err != nil {
			return nil, fmt.Errorf("AUTH_ADMIN_EMAILS contains invalid email %q", email)
		}

		normalized := strings.ToLower(parsed.Address)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		emails = append(emails, parsed.Address)
	}

	return emails, nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
