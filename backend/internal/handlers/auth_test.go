package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
	"github.com/redb0/mixologist/internal/middleware"
)

type mockGoogleOAuthClient struct {
	beginAuth    func(state string, nonce string) (authURL string, sessionData string, err error)
	completeAuth func(ctx context.Context, sessionData string, code string) (idToken string, err error)
}

func (m *mockGoogleOAuthClient) BeginAuth(state string, nonce string) (string, string, error) {
	if m.beginAuth == nil {
		panic("unexpected call to BeginAuth")
	}
	return m.beginAuth(state, nonce)
}

func (m *mockGoogleOAuthClient) CompleteAuth(ctx context.Context, sessionData string, code string) (string, error) {
	if m.completeAuth == nil {
		panic("unexpected call to CompleteAuth")
	}
	return m.completeAuth(ctx, sessionData, code)
}

type mockAuthService struct {
	googleIdentityFromIDToken   func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error)
	upsertGoogleUser            func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error)
	createSession               func(ctx context.Context, userID uint, ttl time.Duration, now time.Time, metadata map[string]any) (string, *domain.Session, error)
	getActiveSessionByRawToken  func(ctx context.Context, rawToken string, now time.Time) (*domain.Session, error)
	getUserBySessionToken       func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error)
	revokeSessionByRawToken     func(ctx context.Context, rawToken string, now time.Time) error
	cleanupExpiredOrRevokedFunc func(ctx context.Context, now time.Time) (int64, error)
}

func (m *mockAuthService) GoogleIdentityFromIDToken(
	ctx context.Context,
	rawIDToken string,
	expectedNonce string,
) (domain.GoogleIdentity, error) {
	if m.googleIdentityFromIDToken == nil {
		panic("unexpected call to GoogleIdentityFromIDToken")
	}
	return m.googleIdentityFromIDToken(ctx, rawIDToken, expectedNonce)
}

func (m *mockAuthService) UpsertGoogleUser(
	ctx context.Context,
	identity domain.GoogleIdentity,
	loginAt time.Time,
) (*domain.User, error) {
	if m.upsertGoogleUser == nil {
		panic("unexpected call to UpsertGoogleUser")
	}
	return m.upsertGoogleUser(ctx, identity, loginAt)
}

func (m *mockAuthService) CreateSession(
	ctx context.Context,
	userID uint,
	ttl time.Duration,
	now time.Time,
	metadata map[string]any,
) (string, *domain.Session, error) {
	if m.createSession == nil {
		panic("unexpected call to CreateSession")
	}
	return m.createSession(ctx, userID, ttl, now, metadata)
}

func (m *mockAuthService) GetActiveSessionByRawToken(ctx context.Context, rawToken string, now time.Time) (*domain.Session, error) {
	if m.getActiveSessionByRawToken == nil {
		panic("unexpected call to GetActiveSessionByRawToken")
	}
	return m.getActiveSessionByRawToken(ctx, rawToken, now)
}

func (m *mockAuthService) GetUserBySessionToken(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
	if m.getUserBySessionToken == nil {
		panic("unexpected call to GetUserBySessionToken")
	}
	return m.getUserBySessionToken(ctx, rawToken, now)
}

func (m *mockAuthService) RevokeSessionByRawToken(ctx context.Context, rawToken string, now time.Time) error {
	if m.revokeSessionByRawToken == nil {
		panic("unexpected call to RevokeSessionByRawToken")
	}
	return m.revokeSessionByRawToken(ctx, rawToken, now)
}

func (m *mockAuthService) CleanupExpiredOrRevokedSessions(ctx context.Context, now time.Time) (int64, error) {
	if m.cleanupExpiredOrRevokedFunc == nil {
		panic("unexpected call to CleanupExpiredOrRevokedSessions")
	}
	return m.cleanupExpiredOrRevokedFunc(ctx, now)
}

func TestNewAuthController_ValidateDependencies(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = NewAuthController(
		&mockAuthService{},
		&mockGoogleOAuthClient{},
		config.AuthConfig{
			SessionCookieName:   "session",
			CSRFHeaderName:      "X-CSRF-Token",
			CSRFSecret:          "csrf_secret",
			SessionCookieSecret: "",
		},
	)
}

func TestAuthController_StartGoogleLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedState string
	var capturedNonce string
	controller := NewAuthController(
		&mockAuthService{},
		&mockGoogleOAuthClient{
			beginAuth: func(state string, nonce string) (string, string, error) {
				capturedState = state
				capturedNonce = nonce
				return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state, "session-data", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/login", controller.StartGoogleLogin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/login?return_to=/ingredients", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if capturedState == "" || capturedNonce == "" {
		t.Fatal("oauth state and nonce must be generated")
	}
	if location := w.Header().Get("Location"); !strings.Contains(location, capturedState) {
		t.Fatalf("redirect location must contain state, got %q", location)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), oauthStateCookieName+"=") {
		t.Fatalf("oauth state cookie not set: %q", w.Header().Get("Set-Cookie"))
	}
}

func TestAuthController_StartGoogleLogin_BeginAuthError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{},
		&mockGoogleOAuthClient{
			beginAuth: func(state string, nonce string) (string, string, error) {
				return "", "", errors.New("provider unavailable")
			},
		},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/login", controller.StartGoogleLogin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_HandleGoogleCallback_StateValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(&mockAuthService{}, &mockGoogleOAuthClient{}, testAuthConfig())
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=invalid", nil)
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, CodeOAuthStateInvalid)
}

func TestAuthController_HandleGoogleCallback_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledCreateSession := false
	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				if rawIDToken != "signed-id-token" || expectedNonce != "nonce-1" {
					t.Fatalf("unexpected token/nonce: %q / %q", rawIDToken, expectedNonce)
				}
				return domain.GoogleIdentity{
					Subject:     "subject-1",
					Email:       "user@example.com",
					DisplayName: "Test User",
					AvatarURL:   "https://example.com/avatar.jpg",
				}, nil
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				return &domain.User{ID: 1, Email: identity.Email, Role: domain.UserRoleUser}, nil
			},
			createSession: func(
				ctx context.Context,
				userID uint,
				ttl time.Duration,
				now time.Time,
				metadata map[string]any,
			) (string, *domain.Session, error) {
				calledCreateSession = true
				return "session-token", &domain.Session{ID: 1, UserID: userID}, nil
			},
		},
		&mockGoogleOAuthClient{
			beginAuth: func(state string, nonce string) (string, string, error) { return "", "", nil },
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if !calledCreateSession {
		t.Fatal("CreateSession was not called")
	}
	if w.Header().Get("Location") != "/ingredients" {
		t.Fatalf("unexpected redirect location: %q", w.Header().Get("Location"))
	}

	sessionCookie := cookieHeader(w, testAuthConfig().SessionCookieName)
	if sessionCookie == "" {
		t.Fatal("session cookie missing")
	}
	if !strings.Contains(sessionCookie, "HttpOnly") {
		t.Fatalf("session cookie must be HttpOnly: %q", sessionCookie)
	}

	csrfCookie := cookieHeader(w, testAuthConfig().CSRFCookieName)
	if csrfCookie == "" {
		t.Fatal("csrf cookie missing")
	}
	if strings.Contains(csrfCookie, "HttpOnly") {
		t.Fatalf("csrf cookie must be readable by JS: %q", csrfCookie)
	}
	expectedCSRF := middleware.SignCSRFToken(testAuthConfig().CSRFSecret, "session-token", time.Unix(1_700_000_000, 0).UTC())
	if !strings.Contains(csrfCookie, expectedCSRF) {
		t.Fatalf("csrf cookie must contain signed token: %q", csrfCookie)
	}
	wantCSRFMaxAge := strconv.Itoa(int(testAuthConfig().SessionTTL.Seconds()))
	if !strings.Contains(csrfCookie, "Max-Age="+wantCSRFMaxAge) {
		t.Fatalf("csrf cookie Max-Age must match session TTL: %q", csrfCookie)
	}

	oauthCookie := cookieHeader(w, oauthStateCookieName)
	if oauthCookie == "" || !strings.Contains(oauthCookie, "Max-Age=0") {
		t.Fatalf("oauth state cookie must be cleared: %q", oauthCookie)
	}
}

func TestAuthController_HandleGoogleCallback_EmailConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				return domain.GoogleIdentity{
					Subject:     "subject-1",
					Email:       "user@example.com",
					DisplayName: "Test User",
				}, nil
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				return nil, domain.NewErrAlreadyExists("email уже используется")
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, CodeOAuthCallbackFailed)
}

func TestAuthController_GetCurrentUser_UnauthorizedWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_GetCurrentUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{
		getUserBySessionToken: func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
			return &domain.User{
				ID:          1,
				Email:       "user@example.com",
				DisplayName: "User",
				Role:        domain.UserRoleAdmin,
			}, nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var body currentUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != 1 || body.Email != "user@example.com" || body.DisplayName != "User" || body.Role != domain.UserRoleAdmin {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthController_GetCurrentUser_InvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{
		getUserBySessionToken: func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
			return nil, domain.NewErrUnauthorized("сессия недействительна")
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "stale-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_Logout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	calledRevoke := false
	authService := &mockAuthService{
		getUserBySessionToken: authenticatedUserLookup(),
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			calledRevoke = true
			return nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	req.Header.Set(cfg.CSRFHeaderName, signedCSRF(cfg, "session-token"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if !calledRevoke {
		t.Fatal("RevokeSessionByRawToken was not called")
	}
	setCookie := strings.Join(w.Header().Values("Set-Cookie"), ";")
	if !strings.Contains(setCookie, cfg.SessionCookieName+"=") ||
		!strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("session cookie must be cleared: %q", setCookie)
	}
}

func TestAuthController_Logout_UnauthorizedWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			t.Fatal("RevokeSessionByRawToken must not be called")
			return nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_Logout_MissingCSRFHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	calledRevoke := false
	authService := &mockAuthService{
		getUserBySessionToken: authenticatedUserLookup(),
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			calledRevoke = true
			return nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: cfg.CSRFCookieName, Value: signedCSRF(cfg, "session-token")})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if calledRevoke {
		t.Fatal("RevokeSessionByRawToken must not be called when CSRF header is missing")
	}
}

func TestAuthController_Logout_ForgedMatchingCookieAndHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	calledRevoke := false
	authService := &mockAuthService{
		getUserBySessionToken: authenticatedUserLookup(),
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			calledRevoke = true
			return nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: cfg.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(cfg.CSRFHeaderName, "csrf-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if calledRevoke {
		t.Fatal("RevokeSessionByRawToken must not be called for client-forged CSRF")
	}
}

func TestAuthController_Logout_InvalidCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	calledRevoke := false
	authService := &mockAuthService{
		getUserBySessionToken: authenticatedUserLookup(),
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			calledRevoke = true
			return nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: cfg.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(cfg.CSRFHeaderName, "other-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if calledRevoke {
		t.Fatal("RevokeSessionByRawToken must not be called when CSRF is invalid")
	}
	var response httperr.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != httperr.CodeCSRFTokenInvalid {
		t.Fatalf("unexpected error code: %s", response.Error.Code)
	}
}

func TestAuthController_Logout_RevokeErrorClearsCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{
		getUserBySessionToken: authenticatedUserLookup(),
		revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
			return domain.NewErrUnauthorized("сессия недействительна")
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	req.Header.Set(cfg.CSRFHeaderName, signedCSRF(cfg, "session-token"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}

	setCookie := strings.Join(w.Header().Values("Set-Cookie"), ";")
	if !strings.Contains(setCookie, cfg.SessionCookieName+"=") {
		t.Fatalf("session cookie should be cleared: %q", setCookie)
	}
	if !strings.Contains(setCookie, cfg.CSRFCookieName+"=") {
		t.Fatalf("csrf cookie should be cleared: %q", setCookie)
	}
}

func authenticatedUserLookup() func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
	return func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
		return &domain.User{
			ID:          1,
			Email:       "user@example.com",
			DisplayName: "User",
			Role:        domain.UserRoleUser,
		}, nil
	}
}

func testNow() time.Time {
	return time.Unix(1_700_000_000, 0).UTC()
}

func signedCSRF(cfg config.AuthConfig, sessionToken string) string {
	return middleware.SignCSRFToken(cfg.CSRFSecret, sessionToken, testNow())
}

func newProtectedAuthEngine(
	controller *AuthController,
	lookup middleware.SessionUserLookup,
	cfg config.AuthConfig,
) *gin.Engine {
	r := gin.New()
	r.Use(requestid.New())
	requireAuth := middleware.RequireAuth(lookup, cfg, testNow)
	r.GET("/api/v1/auth/me", requireAuth, controller.GetCurrentUser)
	r.POST("/api/v1/auth/logout", requireAuth, middleware.RequireCSRF(cfg, testNow), controller.Logout)
	return r
}

func TestAuthController_StartGoogleLogin_InvalidReturnTo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(&mockAuthService{}, &mockGoogleOAuthClient{}, testAuthConfig())
	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/login", controller.StartGoogleLogin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/login?return_to=https://evil.example", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var response httperr.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != httperr.CodeValidationError {
		t.Fatalf("unexpected error code: %s", response.Error.Code)
	}
}

func TestAuthController_HandleGoogleCallback_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Unix(1700000000, 0).UTC()
	validCookie, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    now.Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}
	expiredCookie, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    now.Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build expired oauth state cookie: %v", err)
	}
	tamperedCookie := validCookie[:strings.LastIndex(validCookie, ".")+1] + strings.Repeat("0", 64)

	tests := []struct {
		name     string
		query    string
		cookie   string
		oauthErr error
		wantCode string
	}{
		{
			name:     "google error",
			query:    "?error=access_denied",
			wantCode: CodeOAuthCallbackFailed,
		},
		{
			name:     "missing code",
			query:    "?state=state-1",
			wantCode: CodeOAuthCallbackFailed,
		},
		{
			name:     "missing cookie",
			query:    "?code=ok&state=state-1",
			wantCode: CodeOAuthStateInvalid,
		},
		{
			name:     "expired state",
			query:    "?code=ok&state=state-1",
			cookie:   expiredCookie,
			wantCode: CodeOAuthStateInvalid,
		},
		{
			name:     "state mismatch",
			query:    "?code=ok&state=other-state",
			cookie:   validCookie,
			wantCode: CodeOAuthStateInvalid,
		},
		{
			name:     "tampered cookie",
			query:    "?code=ok&state=state-1",
			cookie:   tamperedCookie,
			wantCode: CodeOAuthStateInvalid,
		},
		{
			name:     "complete auth failed",
			query:    "?code=ok&state=state-1",
			cookie:   validCookie,
			oauthErr: errors.New("exchange failed"),
			wantCode: CodeOAuthCallbackFailed,
		},
		{
			name:     "complete auth timeout",
			query:    "?code=ok&state=state-1",
			cookie:   validCookie,
			oauthErr: context.DeadlineExceeded,
			wantCode: httperr.CodeServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewAuthController(
				&mockAuthService{},
				&mockGoogleOAuthClient{
					completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
						if tt.oauthErr != nil {
							return "", tt.oauthErr
						}
						t.Fatal("CompleteAuth must not be called")
						return "", nil
					},
				},
				testAuthConfig(),
			)
			controller.now = func() time.Time { return now }

			r := gin.New()
			r.Use(requestid.New())
			r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback"+tt.query, nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: tt.cookie})
			}
			r.ServeHTTP(w, req)

			assertAuthErrorRedirect(t, w, tt.wantCode)
		})
	}
}

func TestAuthController_HandleGoogleCallback_InvalidIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				t.Fatal("UpsertGoogleUser must not be called")
				return nil, nil
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, httperr.CodeUnauthorized)
}

func TestAuthController_HandleGoogleCallback_IdentityValidatorUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				return domain.GoogleIdentity{}, domain.NewErrServiceUnavailable("не удалось проверить identity token")
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				t.Fatal("UpsertGoogleUser must not be called")
				return nil, nil
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, httperr.CodeServiceUnavailable)
}

func TestAuthController_HandleGoogleCallback_UpsertInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				return domain.GoogleIdentity{
					Subject:     "subject-1",
					Email:       "user@example.com",
					DisplayName: "Test User",
				}, nil
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				return nil, errors.New("db unavailable")
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, httperr.CodeInternalError)
}

func TestAuthController_HandleGoogleCallback_CreateSessionError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			googleIdentityFromIDToken: func(ctx context.Context, rawIDToken string, expectedNonce string) (domain.GoogleIdentity, error) {
				return domain.GoogleIdentity{
					Subject:     "subject-1",
					Email:       "user@example.com",
					DisplayName: "Test User",
				}, nil
			},
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				return &domain.User{ID: 1, Email: identity.Email, Role: domain.UserRoleUser}, nil
			},
			createSession: func(
				ctx context.Context,
				userID uint,
				ttl time.Duration,
				now time.Time,
				metadata map[string]any,
			) (string, *domain.Session, error) {
				return "", nil, errors.New("db unavailable")
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (string, error) {
				return "signed-id-token", nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    controller.now().Add(oauthStateTTL).Unix(),
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}

	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/google/callback", controller.HandleGoogleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: cookieValue})
	r.ServeHTTP(w, req)

	assertAuthErrorRedirect(t, w, httperr.CodeInternalError)
}

func TestGoogleOAuthClient_CompleteAuth_RespectsContext(t *testing.T) {
	client := NewGoogleOAuthClient(testAuthConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.CompleteAuth(ctx, `{"AuthURL":"","AccessToken":"","RefreshToken":"","ExpiresAt":"0001-01-01T00:00:00Z","IDToken":""}`, "code")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGoogleOAuthClient_CompleteAuth_InvalidSessionData(t *testing.T) {
	client := NewGoogleOAuthClient(testAuthConfig())

	_, err := client.CompleteAuth(context.Background(), "not-json", "code")
	if err == nil {
		t.Fatal("expected error for invalid session data")
	}
}

func TestContextHTTPClient_UsesRequestContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be called when context is canceled")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := contextHTTPClient(ctx)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if _, err = client.Do(req); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAuthController_GetCurrentUser_MissingContextUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(&mockAuthService{}, &mockGoogleOAuthClient{}, testAuthConfig())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)

	controller.GetCurrentUser(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_GetCurrentUser_NoUserFromMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testAuthConfig()
	authService := &mockAuthService{
		getUserBySessionToken: func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
			return nil, nil
		},
	}
	controller := NewAuthController(authService, &mockGoogleOAuthClient{}, cfg)
	r := newProtectedAuthEngine(controller, authService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName, Value: "session-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestValidateReturnTo(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to root", raw: "", want: "/"},
		{name: "whitespace defaults to root", raw: "   ", want: "/"},
		{name: "root", raw: "/", want: "/"},
		{name: "relative path", raw: "/ingredients", want: "/ingredients"},
		{name: "path with query", raw: "/ingredients?sort=name", want: "/ingredients?sort=name"},
		{name: "absolute url", raw: "https://evil.example", wantErr: true},
		{name: "protocol-relative url", raw: "//evil.example", wantErr: true},
		{name: "backslash", raw: `/\evil.example`, wantErr: true},
		{name: "backslash in path", raw: `/foo\bar`, wantErr: true},
		{name: "too long", raw: "/" + strings.Repeat("a", returnToMaxLength), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateReturnTo(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("return_to mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestOAuthStateCookie_RoundTripAndRejectsTampering(t *testing.T) {
	secret := testAuthConfig().SessionCookieSecret
	payload := oauthStatePayload{
		State:        "state-1",
		Nonce:        "nonce-1",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    1700000000,
	}

	value, err := buildOAuthStateCookie(payload, secret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}
	got, err := parseOAuthStateCookie(value, secret)
	if err != nil {
		t.Fatalf("parse oauth state cookie: %v", err)
	}
	if got != payload {
		t.Fatalf("payload mismatch: got %+v want %+v", got, payload)
	}

	_, err = parseOAuthStateCookie(value, "wrong-secret-wrong-secret-wrong")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}

	parts := strings.Split(value, ".")
	_, err = parseOAuthStateCookie(parts[0]+"."+strings.Repeat("0", 64), secret)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}

	_, err = parseOAuthStateCookie("not-a-cookie", secret)
	if err == nil {
		t.Fatal("expected error for malformed cookie")
	}

	_, err = parseOAuthStateCookie("%%%."+strings.Repeat("0", 64), secret)
	if err == nil {
		t.Fatal("expected error for invalid payload base64")
	}
}

func TestParseOAuthStateCookie_IncompletePayload(t *testing.T) {
	value, err := buildOAuthStateCookie(oauthStatePayload{
		State:        "state-1",
		Nonce:        "",
		ReturnTo:     "/ingredients",
		OAuthSession: "oauth-session",
		ExpiresAt:    1700000000,
	}, testAuthConfig().SessionCookieSecret)
	if err != nil {
		t.Fatalf("build oauth state cookie: %v", err)
	}
	_, err = parseOAuthStateCookie(value, testAuthConfig().SessionCookieSecret)
	if err == nil {
		t.Fatal("expected error for incomplete payload")
	}
}

func TestGoogleOAuthClient_BeginAuth_RequiresStateAndNonce(t *testing.T) {
	client := NewGoogleOAuthClient(testAuthConfig())

	_, _, err := client.BeginAuth("", "nonce")
	if !errors.Is(err, domain.ErrInvalidAuthData) {
		t.Fatalf("expected invalid auth data for empty state, got %v", err)
	}

	_, _, err = client.BeginAuth("state", "  ")
	if !errors.Is(err, domain.ErrInvalidAuthData) {
		t.Fatalf("expected invalid auth data for empty nonce, got %v", err)
	}
}

func TestGoogleOAuthClient_BeginAuth_Success(t *testing.T) {
	client := NewGoogleOAuthClient(testAuthConfig())

	authURL, sessionData, err := client.BeginAuth("state-1", "nonce-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authURL == "" || sessionData == "" {
		t.Fatal("auth URL and session data must be returned")
	}
	if !strings.Contains(authURL, "nonce=nonce-1") {
		t.Fatalf("auth URL must include nonce: %q", authURL)
	}
}

func TestAppendQueryParam(t *testing.T) {
	got, err := appendQueryParam("https://accounts.google.com/o/oauth2/v2/auth?state=abc", "nonce", "nonce-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "nonce=nonce-1") || !strings.Contains(got, "state=abc") {
		t.Fatalf("query param missing: %q", got)
	}

	_, err = appendQueryParam("http://[", "nonce", "nonce-1")
	if err == nil {
		t.Fatal("expected error for invalid url")
	}
}

func cookieHeader(w *httptest.ResponseRecorder, name string) string {
	prefix := name + "="
	for _, header := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(header, prefix) {
			return header
		}
	}
	return ""
}

func assertAuthErrorRedirect(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	if w.Code != http.StatusFound {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if location.Path != authErrorPath {
		t.Fatalf("unexpected redirect path: %q", location.Path)
	}
	if location.Query().Get("code") != wantCode {
		t.Fatalf("unexpected error code: %q", location.Query().Get("code"))
	}
	if strings.TrimSpace(location.Query().Get("message")) == "" {
		t.Fatal("redirect must include error message")
	}
	oauthCookie := cookieHeader(w, oauthStateCookieName)
	if oauthCookie == "" || !strings.Contains(oauthCookie, "Max-Age=0") {
		t.Fatalf("oauth state cookie must be cleared: %q", oauthCookie)
	}
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		GoogleClientID:      "google-client-id",
		GoogleClientSecret:  "google-client-secret",
		GoogleCallbackURL:   "http://localhost:8080/api/v1/auth/google/callback",
		SessionCookieName:   "session",
		SessionCookieSecret: strings.Repeat("a", 32),
		SessionCookieDomain: "",
		SessionCookieSecure: false,
		SessionTTL:          24 * time.Hour,
		CSRFSecret:          strings.Repeat("b", 32),
		CSRFCookieName:      "csrf_token",
		CSRFHeaderName:      "X-CSRF-Token",
	}
}
