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
	setCookie := strings.Join(w.Header().Values("Set-Cookie"), ";")
	if !strings.Contains(setCookie, testAuthConfig().SessionCookieName+"=") {
		t.Fatalf("session cookie missing: %q", setCookie)
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

func TestValidateGoogleUserData(t *testing.T) {
	testCases := []struct {
		name             string
		validator        GoogleIDTokenValidator
		user             goth.User
		expectedNonce    string
		expectedAudience string
		expectError      bool
		expectedSubject  string
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
					return googleIDTokenClaims{
						Subject:       "subject-1",
						Nonce:         "nonce-1",
						EmailVerified: true,
					}, nil
				},
			},
			user: goth.User{
				IDToken:   "signed-id-token",
				UserID:    "subject-1",
				Email:     "user@example.com",
				Name:      "User",
				AvatarURL: "https://example.com/avatar.jpg",
			},
			expectedNonce:    "nonce-1",
			expectedAudience: "google-client-id",
			expectError:      false,
			expectedSubject:  "subject-1",
		},
		{
			name: "invalid token signature",
			validator: &mockGoogleIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error) {
					return googleIDTokenClaims{}, errors.New("invalid signature")
				},
			},
			user: goth.User{
				IDToken: "unsigned-token",
				UserID:  "subject-1",
				Email:   "user@example.com",
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
					return googleIDTokenClaims{
						Subject:       "subject-1",
						Nonce:         "nonce-1",
						EmailVerified: true,
					}, nil
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
		})
	}
}

func TestValidateReturnTo_TooLong(t *testing.T) {
	_, err := validateReturnTo("/" + strings.Repeat("a", returnToMaxLength))
	if err == nil {
		t.Fatal("expected error for too long return_to")
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
		CSRFCookieName:      "csrf_token",
		CSRFHeaderName:      "X-CSRF-Token",
	}
}
