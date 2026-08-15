package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
)

var csrfTestNow = time.Unix(1_700_000_000, 0).UTC()

type stubSessionLookup struct {
	user  *domain.User
	err   error
	calls int
}

func (s *stubSessionLookup) GetUserBySessionToken(
	_ context.Context,
	_ string,
	_ time.Time,
) (*domain.User, error) {
	s.calls++
	return s.user, s.err
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	adminCSRF := SignCSRFToken(cfg.CSRFSecret, "admin-token", csrfTestNow)
	userCSRF := SignCSRFToken(cfg.CSRFSecret, "user-token", csrfTestNow)
	admin := &domain.User{
		ID:          1,
		Email:       "admin@example.com",
		DisplayName: "Admin",
		Role:        domain.UserRoleAdmin,
	}
	user := &domain.User{
		ID:          2,
		Email:       "user@example.com",
		DisplayName: "User",
		Role:        domain.UserRoleUser,
	}

	tests := []struct {
		name           string
		method         string
		path           string
		lookup         *stubSessionLookup
		cookies        []*http.Cookie
		csrfHeader     string
		wantStatus     int
		wantCode       string
		wantHandlerHit bool
		assertContext  bool
	}{
		{
			name:       "protected route without cookie returns 401",
			method:     http.MethodGet,
			path:       "/me",
			lookup:     &stubSessionLookup{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
		},
		{
			name:   "expired session returns 401",
			method: http.MethodGet,
			path:   "/me",
			lookup: &stubSessionLookup{err: domain.NewErrUnauthorized("сессия недействительна")},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "expired-token"},
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
		},
		{
			name:   "revoked session returns 401",
			method: http.MethodGet,
			path:   "/me",
			lookup: &stubSessionLookup{err: domain.NewErrUnauthorized("сессия недействительна")},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "revoked-token"},
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
		},
		{
			name:   "authenticated user without admin role returns 403",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: user},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "user-token"},
			},
			csrfHeader: userCSRF,
			wantStatus: http.StatusForbidden,
			wantCode:   httperr.CodeForbidden,
		},
		{
			name:   "admin passes RequireRole",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			csrfHeader:     adminCSRF,
			wantStatus:     http.StatusOK,
			wantHandlerHit: true,
			assertContext:  true,
		},
		{
			name:   "mutating request without csrf token is rejected",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			wantStatus: http.StatusForbidden,
			wantCode:   CodeCSRFTokenInvalid,
		},
		{
			name:   "mutating request with invalid csrf token is rejected",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
				{Name: cfg.CSRFCookieName, Value: "csrf-token"},
			},
			csrfHeader: "other-token",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeCSRFTokenInvalid,
		},
		{
			name:   "mutating request with valid csrf token passes",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			csrfHeader:     adminCSRF,
			wantStatus:     http.StatusOK,
			wantHandlerHit: true,
		},
		{
			name:   "csrf token from two hours ago is accepted within session ttl",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			csrfHeader: SignCSRFToken(
				cfg.CSRFSecret,
				"admin-token",
				csrfTestNow.Add(-2*time.Hour),
			),
			wantStatus:     http.StatusOK,
			wantHandlerHit: true,
		},
		{
			name:   "client forged matching csrf cookie and header is rejected",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
				{Name: cfg.CSRFCookieName, Value: "csrf-token"},
			},
			csrfHeader: "csrf-token",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeCSRFTokenInvalid,
		},
		{
			name:   "csrf token signed for another session is rejected",
			method: http.MethodPost,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			csrfHeader: userCSRF,
			wantStatus: http.StatusForbidden,
			wantCode:   CodeCSRFTokenInvalid,
		},
		{
			name:   "safe method skips csrf check",
			method: http.MethodGet,
			path:   "/admin",
			lookup: &stubSessionLookup{user: admin},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "admin-token"},
			},
			wantStatus:     http.StatusOK,
			wantHandlerHit: true,
		},
		{
			name:   "authenticated user is written to context without extra lookup",
			method: http.MethodGet,
			path:   "/me",
			lookup: &stubSessionLookup{user: user},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "user-token"},
			},
			wantStatus:     http.StatusOK,
			wantHandlerHit: true,
			assertContext:  true,
		},
		{
			name:   "session lookup failure returns 503",
			method: http.MethodGet,
			path:   "/me",
			lookup: &stubSessionLookup{err: domain.NewErrServiceUnavailable("сервис недоступен")},
			cookies: []*http.Cookie{
				{Name: cfg.SessionCookieName, Value: "session-token"},
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   httperr.CodeServiceUnavailable,
		},
		{
			name:       "require role without auth returns 401",
			method:     http.MethodGet,
			path:       "/role-only",
			lookup:     &stubSessionLookup{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerHit := false
			var capturedUser *domain.User
			var capturedID uint
			var capturedEmail string
			var capturedRole domain.UserRole

			r := gin.New()
			r.Use(requestid.New())
			now := func() time.Time { return csrfTestNow }
			requireAuth := RequireAuth(tt.lookup, cfg, now)
			requireAdmin := RequireRole(domain.UserRoleAdmin)
			requireCSRF := RequireCSRF(cfg, now)

			writeUser := func(c *gin.Context) {
				handlerHit = true
				capturedUser, _ = CurrentUser(c)
				capturedID, _ = CurrentUserID(c)
				capturedRole, _ = CurrentRole(c)
				if capturedUser != nil {
					capturedEmail = capturedUser.Email
				}
				id, _ := c.Get(currentUserIDKey)
				email, _ := c.Get(currentUserEmailKey)
				role, _ := c.Get(currentUserRoleKey)
				assert.Equal(t, capturedID, id)
				assert.Equal(t, capturedEmail, email)
				assert.Equal(t, capturedRole, role)
				c.Status(http.StatusOK)
			}

			r.GET("/me", requireAuth, writeUser)
			r.GET("/admin", requireAuth, requireAdmin, requireCSRF, writeUser)
			r.POST("/admin", requireAuth, requireAdmin, requireCSRF, writeUser)
			r.GET("/role-only", requireAdmin, writeUser)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for _, cookie := range tt.cookies {
				req.AddCookie(cookie)
			}
			if tt.csrfHeader != "" {
				req.Header.Set(cfg.CSRFHeaderName, tt.csrfHeader)
			}

			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantHandlerHit, handlerHit)

			if tt.wantCode != "" {
				var body errorResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body.Error.Code)
				assert.NotEmpty(t, body.Error.Message)
				assert.NotEmpty(t, body.Error.RequestID)
			}

			if !hasSessionCookie(tt.cookies, cfg.SessionCookieName) {
				assert.Equal(t, 0, tt.lookup.calls)
			}

			if tt.assertContext {
				require.NotNil(t, capturedUser)
				assert.Equal(t, tt.lookup.user.ID, capturedID)
				assert.Equal(t, tt.lookup.user.Email, capturedEmail)
				assert.Equal(t, tt.lookup.user.Role, capturedRole)
				assert.Equal(t, 1, tt.lookup.calls)
			}
		})
	}
}

func TestRequireCSRF_SkipsWhenSessionCookieMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	handlerHit := false
	r := gin.New()
	r.Use(requestid.New(), RequireCSRF(cfg, nil))
	r.POST("/public", func(c *gin.Context) {
		handlerHit = true
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/public", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, handlerHit)
}

func TestSignCSRFToken(t *testing.T) {
	secret := strings.Repeat("a", 32)

	token := SignCSRFToken(secret, "session-token", csrfTestNow)
	assert.NotEmpty(t, token)
	assert.Contains(t, token, ".")
	assert.Equal(t, token, SignCSRFToken(secret, "session-token", csrfTestNow))
	assert.NotEqual(t, token, SignCSRFToken(secret, "other-session", csrfTestNow))
	assert.NotEqual(t, token, SignCSRFToken(strings.Repeat("b", 32), "session-token", csrfTestNow))
	assert.NotEqual(t, token, SignCSRFToken(secret, "session-token", csrfTestNow.Add(2*time.Hour)))
}

func TestValidCSRFToken(t *testing.T) {
	secret := strings.Repeat("a", 32)
	session := "session-token"
	current := SignCSRFToken(secret, session, csrfTestNow)
	sessionTTL := 24 * time.Hour

	tests := []struct {
		name   string
		token  string
		now    time.Time
		maxAge time.Duration
		want   bool
	}{
		{name: "current hour", token: current, now: csrfTestNow, maxAge: sessionTTL, want: true},
		{name: "previous hour", token: SignCSRFToken(secret, session, csrfTestNow.Add(-time.Hour)), now: csrfTestNow, maxAge: sessionTTL, want: true},
		{name: "next hour", token: SignCSRFToken(secret, session, csrfTestNow.Add(time.Hour)), now: csrfTestNow, maxAge: sessionTTL, want: true},
		{name: "two hours old within session ttl", token: SignCSRFToken(secret, session, csrfTestNow.Add(-2*time.Hour)), now: csrfTestNow, maxAge: sessionTTL, want: true},
		{name: "older than session ttl", token: SignCSRFToken(secret, session, csrfTestNow.Add(-25*time.Hour)), now: csrfTestNow, maxAge: sessionTTL, want: false},
		{name: "other session", token: SignCSRFToken(secret, "other-session", csrfTestNow), now: csrfTestNow, maxAge: sessionTTL, want: false},
		{name: "empty", token: "", now: csrfTestNow, maxAge: sessionTTL, want: false},
		{name: "malformed", token: "not-a-token", now: csrfTestNow, maxAge: sessionTTL, want: false},
		{name: "tampered mac", token: strings.Split(current, ".")[0] + ".deadbeef", now: csrfTestNow, maxAge: sessionTTL, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidCSRFToken(secret, session, tt.token, tt.now, tt.maxAge))
		})
	}
}

func TestRequireAuth_RefreshesCSRFCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	lookup := &stubSessionLookup{
		user: &domain.User{ID: 1, Email: "user@example.com", Role: domain.UserRoleUser},
	}
	r := gin.New()
	r.Use(requestid.New(), RequireAuth(lookup, cfg, func() time.Time { return csrfTestNow }))
	r.GET("/me", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "user-token"})
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	expected := SignCSRFToken(cfg.CSRFSecret, "user-token", csrfTestNow)
	found := false
	for _, header := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(header, cfg.CSRFCookieName+"=") && strings.Contains(header, expected) {
			found = true
			assert.NotContains(t, header, "HttpOnly")
			assert.Contains(t, header, "Max-Age="+strconv.Itoa(int(cfg.SessionTTL.Seconds())))
			break
		}
	}
	assert.True(t, found, "csrf cookie must be refreshed: %q", w.Header().Values("Set-Cookie"))
}

func TestCurrentUser_AbsentOrInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name  string
		setup func(c *gin.Context)
	}{
		{name: "missing"},
		{
			name: "wrong type",
			setup: func(c *gin.Context) {
				c.Set(currentUserKey, "not-a-user")
			},
		},
		{
			name: "nil user",
			setup: func(c *gin.Context) {
				var user *domain.User
				c.Set(currentUserKey, user)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if tt.setup != nil {
				tt.setup(c)
			}

			user, ok := CurrentUser(c)
			assert.False(t, ok)
			assert.Nil(t, user)

			id, ok := CurrentUserID(c)
			assert.False(t, ok)
			assert.Equal(t, uint(0), id)

			role, ok := CurrentRole(c)
			assert.False(t, ok)
			assert.Equal(t, domain.UserRole(""), role)
		})
	}
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "unauthorized",
			err:        domain.NewErrUnauthorized("требуется аутентификация"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
			wantMsg:    "требуется аутентификация",
		},
		{
			name:       "forbidden",
			err:        domain.NewErrForbidden("недостаточно прав"),
			wantStatus: http.StatusForbidden,
			wantCode:   httperr.CodeForbidden,
			wantMsg:    "недостаточно прав",
		},
		{
			name:       "service unavailable",
			err:        domain.NewErrServiceUnavailable("сервис недоступен"),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   httperr.CodeServiceUnavailable,
			wantMsg:    "сервис недоступен",
		},
		{
			name:       "unknown",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   httperr.CodeInternalError,
			wantMsg:    httperr.InternalServerErrorMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, message := httperr.Map(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, message)
		})
	}
}

func hasSessionCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}

func TestRequireAuth_NilUserFromLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	lookup := &stubSessionLookup{user: nil}
	r := gin.New()
	r.Use(requestid.New(), RequireAuth(lookup, cfg, func() time.Time { return csrfTestNow }))
	r.GET("/me", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "user-token"})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidCSRFToken_ZeroMaxAgeUsesMinimumSkew(t *testing.T) {
	secret := strings.Repeat("a", 32)
	session := "session-token"
	token := SignCSRFToken(secret, session, csrfTestNow)
	assert.True(t, ValidCSRFToken(secret, session, token, csrfTestNow, 0))
}

func TestSetCSRFCookie_ZeroTTLUsesFallbackMaxAge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	cfg.SessionTTL = 0
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/me", nil)

	SetCSRFCookie(c, cfg, "session-token", csrfTestNow)

	found := false
	for _, header := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(header, cfg.CSRFCookieName+"=") {
			found = true
			assert.Contains(t, header, "Max-Age=")
			break
		}
	}
	assert.True(t, found)
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		SessionCookieName:   "session",
		SessionCookieSecret: strings.Repeat("a", 32),
		SessionTTL:          24 * time.Hour,
		CSRFSecret:          strings.Repeat("b", 32),
		CSRFCookieName:      "csrf_token",
		CSRFHeaderName:      "X-CSRF-Token",
	}
}
