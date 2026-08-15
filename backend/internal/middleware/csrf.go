package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/httperr"
)

const (
	CodeCSRFTokenInvalid = httperr.CodeCSRFTokenInvalid
	csrfInvalidMessage   = "Некорректный CSRF token"
	csrfMACPrefix        = "csrf-v1:"
	csrfHourSeconds      = int64(3600)
	csrfHourSkew         = int64(1)
)

func SignCSRFToken(secret, sessionToken string, now time.Time) string {
	hour := csrfHourBucket(now)
	return strconv.FormatInt(hour, 10) + "." + csrfMAC(secret, sessionToken, hour)
}

func ValidCSRFToken(secret, sessionToken, token string, now time.Time, maxAge time.Duration) bool {
	hourStr, mac, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || hourStr == "" || mac == "" {
		return false
	}

	hour, err := strconv.ParseInt(hourStr, 10, 64)
	if err != nil || hour < 0 {
		return false
	}

	current := csrfHourBucket(now)
	delta := current - hour
	if delta < -csrfHourSkew || delta > csrfMaxHourDelta(maxAge) {
		return false
	}

	expected := csrfMAC(secret, sessionToken, hour)
	return hmac.Equal([]byte(mac), []byte(expected))
}

func SetCSRFCookie(c *gin.Context, cfg config.AuthConfig, sessionToken string, now time.Time) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		cfg.CSRFCookieName,
		SignCSRFToken(cfg.CSRFSecret, sessionToken, now),
		csrfCookieMaxAge(cfg.SessionTTL),
		"/",
		cfg.SessionCookieDomain,
		cfg.SessionCookieSecure,
		false,
	)
}

func RequireCSRF(cfg config.AuthConfig, now func() time.Time) gin.HandlerFunc {
	if now == nil {
		now = time.Now
	}

	return func(c *gin.Context) {
		if isSafeHTTPMethod(c.Request.Method) {
			c.Next()
			return
		}

		sessionToken, err := c.Cookie(cfg.SessionCookieName)
		sessionToken = strings.TrimSpace(sessionToken)
		if err != nil || sessionToken == "" {
			c.Next()
			return
		}

		headerToken := strings.TrimSpace(c.GetHeader(cfg.CSRFHeaderName))
		if !ValidCSRFToken(cfg.CSRFSecret, sessionToken, headerToken, now().UTC(), cfg.SessionTTL) {
			httperr.Abort(c, http.StatusForbidden, CodeCSRFTokenInvalid, csrfInvalidMessage)
			return
		}

		c.Next()
	}
}

func csrfHourBucket(now time.Time) int64 {
	return now.UTC().Unix() / csrfHourSeconds
}

func csrfMaxHourDelta(maxAge time.Duration) int64 {
	if maxAge <= 0 {
		return csrfHourSkew
	}
	delta := int64(maxAge/time.Second) / csrfHourSeconds
	if delta < csrfHourSkew {
		return csrfHourSkew
	}
	return delta
}

func csrfCookieMaxAge(ttl time.Duration) int {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return int((csrfHourSkew + 1) * csrfHourSeconds)
	}
	return seconds
}

func csrfMAC(secret, sessionToken string, hour int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(csrfMACPrefix))
	_, _ = mac.Write([]byte(sessionToken))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write([]byte(strconv.FormatInt(hour, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
