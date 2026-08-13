package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
)

type mockGoogleOAuthClient struct {
	beginAuth    func(state string, nonce string) (authURL string, sessionData string, err error)
	completeAuth func(ctx context.Context, sessionData string, code string) (goth.User, error)
}

func (m *mockGoogleOAuthClient) BeginAuth(state string, nonce string) (string, string, error) {
	if m.beginAuth == nil {
		panic("unexpected call to BeginAuth")
	}
	return m.beginAuth(state, nonce)
}

func (m *mockGoogleOAuthClient) CompleteAuth(ctx context.Context, sessionData string, code string) (goth.User, error) {
	if m.completeAuth == nil {
		panic("unexpected call to CompleteAuth")
	}
	return m.completeAuth(ctx, sessionData, code)
}

type mockGoogleIDTokenValidator struct {
	validate func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error)
}

func (m *mockGoogleIDTokenValidator) Validate(
	ctx context.Context,
	idToken string,
	audience string,
) (googleIDTokenClaims, error) {
	if m.validate == nil {
		panic("unexpected call to Validate")
	}
	return m.validate(ctx, idToken, audience)
}

type mockAuthService struct {
	upsertGoogleUser            func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error)
	createSession               func(ctx context.Context, userID uint, ttl time.Duration, now time.Time, metadata map[string]any) (string, *domain.Session, error)
	getActiveSessionByRawToken  func(ctx context.Context, rawToken string, now time.Time) (*domain.Session, error)
	getUserBySessionToken       func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error)
	revokeSessionByRawToken     func(ctx context.Context, rawToken string, now time.Time) error
	cleanupExpiredOrRevokedFunc func(ctx context.Context, now time.Time) (int64, error)
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

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != CodeOAuthStateInvalid {
		t.Fatalf("unexpected error code: %s", response.Error.Code)
	}
}

func TestAuthController_HandleGoogleCallback_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledCreateSession := false
	controller := NewAuthController(
		&mockAuthService{
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
			completeAuth: func(ctx context.Context, sessionData string, code string) (goth.User, error) {
				return goth.User{
					IDToken:   "signed-id-token",
					UserID:    "subject-1",
					Email:     "user@example.com",
					Name:      "Test User",
					AvatarURL: "https://example.com/avatar.jpg",
				}, nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	controller.idTokenValidator = &mockGoogleIDTokenValidator{
		validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
			if idToken != "signed-id-token" {
				t.Fatalf("unexpected id token: %q", idToken)
			}
			if audience != "google-client-id" {
				t.Fatalf("unexpected audience: %q", audience)
			}
			return googleIDTokenClaims{
				Subject:       "subject-1",
				Nonce:         "nonce-1",
				EmailVerified: true,
			}, nil
		},
	}

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

	oauthCookie := cookieHeader(w, oauthStateCookieName)
	if oauthCookie == "" || !strings.Contains(oauthCookie, "Max-Age=0") {
		t.Fatalf("oauth state cookie must be cleared: %q", oauthCookie)
	}
}

func TestAuthController_HandleGoogleCallback_EmailConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				return nil, domain.NewErrAlreadyExists("email уже используется")
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (goth.User, error) {
				return goth.User{
					IDToken: "signed-id-token",
					UserID:  "subject-1",
					Email:   "user@example.com",
					Name:    "Test User",
				}, nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	controller.idTokenValidator = &mockGoogleIDTokenValidator{
		validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
			return googleIDTokenClaims{
				Subject:       "subject-1",
				Nonce:         "nonce-1",
				EmailVerified: true,
			}, nil
		},
	}

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

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var response ErrorResponse
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != CodeOAuthCallbackFailed {
		t.Fatalf("unexpected error code: %s", response.Error.Code)
	}
}

func TestAuthController_GetCurrentUser_UnauthorizedWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(&mockAuthService{}, &mockGoogleOAuthClient{}, testAuthConfig())
	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/me", controller.GetCurrentUser)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_GetCurrentUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			getUserBySessionToken: func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
				return &domain.User{
					ID:          1,
					Email:       "user@example.com",
					DisplayName: "User",
					Role:        domain.UserRoleAdmin,
				}, nil
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/me", controller.GetCurrentUser)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "session-token"})
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

	controller := NewAuthController(
		&mockAuthService{
			getUserBySessionToken: func(ctx context.Context, rawToken string, now time.Time) (*domain.User, error) {
				return nil, domain.NewErrUnauthorized("сессия недействительна")
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.GET("/api/v1/auth/me", controller.GetCurrentUser)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "stale-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_Logout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledRevoke := false
	controller := NewAuthController(
		&mockAuthService{
			revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
				calledRevoke = true
				return nil
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.POST("/api/v1/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: testAuthConfig().CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(testAuthConfig().CSRFHeaderName, "csrf-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if !calledRevoke {
		t.Fatal("RevokeSessionByRawToken was not called")
	}
	setCookie := strings.Join(w.Header().Values("Set-Cookie"), ";")
	if !strings.Contains(setCookie, testAuthConfig().SessionCookieName+"=") ||
		!strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("session cookie must be cleared: %q", setCookie)
	}
}

func TestAuthController_Logout_UnauthorizedWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
				t.Fatal("RevokeSessionByRawToken must not be called")
				return nil
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.POST("/api/v1/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_Logout_MissingCSRFCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledRevoke := false
	controller := NewAuthController(
		&mockAuthService{
			revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
				calledRevoke = true
				return nil
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.POST("/api/v1/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "session-token"})
	req.Header.Set(testAuthConfig().CSRFHeaderName, "csrf-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if calledRevoke {
		t.Fatal("RevokeSessionByRawToken must not be called when CSRF cookie is missing")
	}
}

func TestAuthController_Logout_InvalidCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledRevoke := false
	controller := NewAuthController(
		&mockAuthService{
			revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
				calledRevoke = true
				return nil
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.POST("/api/v1/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: testAuthConfig().CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(testAuthConfig().CSRFHeaderName, "other-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if calledRevoke {
		t.Fatal("RevokeSessionByRawToken must not be called when CSRF is invalid")
	}
	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != CodeCSRFTokenInvalid {
		t.Fatalf("unexpected error code: %s", response.Error.Code)
	}
}

func TestAuthController_Logout_RevokeErrorClearsCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			revokeSessionByRawToken: func(ctx context.Context, rawToken string, now time.Time) error {
				return domain.NewErrUnauthorized("сессия недействительна")
			},
		},
		&mockGoogleOAuthClient{},
		testAuthConfig(),
	)
	r := gin.New()
	r.Use(requestid.New())
	r.POST("/api/v1/auth/logout", controller.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: testAuthConfig().SessionCookieName, Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: testAuthConfig().CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(testAuthConfig().CSRFHeaderName, "csrf-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}

	setCookie := strings.Join(w.Header().Values("Set-Cookie"), ";")
	if !strings.Contains(setCookie, testAuthConfig().SessionCookieName+"=") {
		t.Fatalf("session cookie should be cleared: %q", setCookie)
	}
	if !strings.Contains(setCookie, testAuthConfig().CSRFCookieName+"=") {
		t.Fatalf("csrf cookie should be cleared: %q", setCookie)
	}
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
	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != CodeValidationError {
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
		name       string
		query      string
		cookie     string
		oauthErr   error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "google error",
			query:      "?error=access_denied",
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthCallbackFailed,
		},
		{
			name:       "missing code",
			query:      "?state=state-1",
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthCallbackFailed,
		},
		{
			name:       "missing cookie",
			query:      "?code=ok&state=state-1",
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthStateInvalid,
		},
		{
			name:       "expired state",
			query:      "?code=ok&state=state-1",
			cookie:     expiredCookie,
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthStateInvalid,
		},
		{
			name:       "state mismatch",
			query:      "?code=ok&state=other-state",
			cookie:     validCookie,
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthStateInvalid,
		},
		{
			name:       "tampered cookie",
			query:      "?code=ok&state=state-1",
			cookie:     tamperedCookie,
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthStateInvalid,
		},
		{
			name:       "complete auth failed",
			query:      "?code=ok&state=state-1",
			cookie:     validCookie,
			oauthErr:   errors.New("exchange failed"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeOAuthCallbackFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewAuthController(
				&mockAuthService{},
				&mockGoogleOAuthClient{
					completeAuth: func(ctx context.Context, sessionData string, code string) (goth.User, error) {
						if tt.oauthErr != nil {
							return goth.User{}, tt.oauthErr
						}
						t.Fatal("CompleteAuth must not be called")
						return goth.User{}, nil
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

			if w.Code != tt.wantStatus {
				t.Fatalf("unexpected status: %d", w.Code)
			}
			var response ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if response.Error.Code != tt.wantCode {
				t.Fatalf("unexpected error code: %s", response.Error.Code)
			}
		})
	}
}

func TestAuthController_HandleGoogleCallback_InvalidIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
			upsertGoogleUser: func(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error) {
				t.Fatal("UpsertGoogleUser must not be called")
				return nil, nil
			},
		},
		&mockGoogleOAuthClient{
			completeAuth: func(ctx context.Context, sessionData string, code string) (goth.User, error) {
				return goth.User{
					IDToken: "signed-id-token",
					UserID:  "subject-1",
					Email:   "user@example.com",
					Name:    "Test User",
				}, nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	controller.idTokenValidator = &mockGoogleIDTokenValidator{
		validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
			return googleIDTokenClaims{
				Subject:       "subject-1",
				Nonce:         "other-nonce",
				EmailVerified: true,
			}, nil
		},
	}

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

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestAuthController_HandleGoogleCallback_CreateSessionError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewAuthController(
		&mockAuthService{
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
			completeAuth: func(ctx context.Context, sessionData string, code string) (goth.User, error) {
				return goth.User{
					IDToken: "signed-id-token",
					UserID:  "subject-1",
					Email:   "user@example.com",
					Name:    "Test User",
				}, nil
			},
		},
		testAuthConfig(),
	)
	controller.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	controller.idTokenValidator = &mockGoogleIDTokenValidator{
		validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
			return googleIDTokenClaims{
				Subject:       "subject-1",
				Nonce:         "nonce-1",
				EmailVerified: true,
			}, nil
		},
	}

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

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestValidateGoogleUserData(t *testing.T) {
	successUser := goth.User{
		IDToken:   "signed-id-token",
		UserID:    "subject-1",
		Email:     "User@Example.com",
		Name:      " User ",
		AvatarURL: "https://example.com/avatar.jpg",
	}
	validClaims := googleIDTokenClaims{
		Subject:       "subject-1",
		Nonce:         "nonce-1",
		EmailVerified: true,
	}
	testCases := []struct {
		name             string
		validator        GoogleIDTokenValidator
		user             goth.User
		expectedNonce    string
		expectedAudience string
		expectError      bool
		expectedSubject  string
		expectedEmail    string
		expectedName     string
	}{
		{
			name: "success",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					if idToken != "signed-id-token" {
						t.Fatalf("unexpected id token: %q", idToken)
					}
					if audience != "google-client-id" {
						t.Fatalf("unexpected audience: %q", audience)
					}
					return validClaims, nil
				},
			},
			user:             successUser,
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      false,
			expectedSubject:  "subject-1",
			expectedEmail:    "user@example.com",
			expectedName:     "User",
		},
		{
			name: "invalid token signature",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					return googleIDTokenClaims{}, errors.New("invalid signature")
				},
			},
			user:             successUser,
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
		{
			name: "nonce mismatch",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					return validClaims, nil
				},
			},
			user:             successUser,
			expectedNonce:    "other-nonce",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
		{
			name: "empty nonce",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					claims := validClaims
					claims.Nonce = ""
					return claims, nil
				},
			},
			user:             successUser,
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
		{
			name: "email not verified",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					claims := validClaims
					claims.EmailVerified = false
					return claims, nil
				},
			},
			user:             successUser,
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
		{
			name: "empty email",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					return validClaims, nil
				},
			},
			user: goth.User{
				IDToken: "signed-id-token",
				UserID:  "subject-1",
				Email:   "   ",
				Name:    "User",
			},
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
		{
			name: "mismatched user subject",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					return validClaims, nil
				},
			},
			user: goth.User{
				IDToken: "signed-id-token",
				UserID:  "different-subject",
				Email:   "user@example.com",
				Name:    "User",
			},
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			identity, err := validateGoogleUserData(
				context.Background(),
				tc.validator,
				tc.user,
				tc.expectedNonce,
				tc.expectedAudience,
			)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if identity.Subject != tc.expectedSubject {
				t.Fatalf("subject mismatch: got %q want %q", identity.Subject, tc.expectedSubject)
			}
			if identity.Email != tc.expectedEmail {
				t.Fatalf("email mismatch: got %q want %q", identity.Email, tc.expectedEmail)
			}
			if identity.DisplayName != tc.expectedName {
				t.Fatalf("display name mismatch: got %q want %q", identity.DisplayName, tc.expectedName)
			}
		})
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
		CSRFCookieName:      "csrf_token",
		CSRFHeaderName:      "X-CSRF-Token",
	}
}
