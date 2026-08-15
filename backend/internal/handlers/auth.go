package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gothGoogle "github.com/markbates/goth/providers/google"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
	"github.com/redb0/mixologist/internal/middleware"
	"github.com/redb0/mixologist/internal/services"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateTTL        = 10 * time.Minute
	oauthCallbackTimeout = 10 * time.Second
	returnToMaxLength    = 2048

	CodeOAuthStateInvalid   = "OAUTH_STATE_INVALID"
	CodeOAuthCallbackFailed = "OAUTH_CALLBACK_FAILED"
)

var returnToPattern = regexp.MustCompile(`^/(?:$|[^/\\\s][^\\\s]*)$`)

type AuthController struct {
	authService services.AuthService
	oauthClient GoogleOAuthClient
	authConfig  config.AuthConfig
	now         func() time.Time
}

type GoogleOAuthClient interface {
	BeginAuth(state string, nonce string) (authURL string, sessionData string, err error)
	CompleteAuth(ctx context.Context, sessionData string, code string) (idToken string, err error)
}

type googleOAuthClient struct {
	provider *gothGoogle.Provider
}

type oauthStatePayload struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	ReturnTo     string `json:"return_to"`
	OAuthSession string `json:"oauth_session"`
	ExpiresAt    int64  `json:"expires_at"`
}

type currentUserResponse struct {
	ID          uint            `json:"id"`
	Email       string          `json:"email"`
	DisplayName string          `json:"display_name"`
	AvatarURL   string          `json:"avatar_url,omitempty"`
	Role        domain.UserRole `json:"role"`
}

func NewGoogleOAuthClient(authConfig config.AuthConfig) GoogleOAuthClient {
	return &googleOAuthClient{
		provider: gothGoogle.New(
			authConfig.GoogleClientID,
			authConfig.GoogleClientSecret,
			authConfig.GoogleCallbackURL,
			"openid",
			"email",
			"profile",
		),
	}
}

func (c *googleOAuthClient) BeginAuth(state string, nonce string) (string, string, error) {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(nonce) == "" {
		return "", "", domain.NewErrInvalidAuthData("state и nonce обязательны")
	}

	session, err := c.provider.BeginAuth(state)
	if err != nil {
		return "", "", err
	}
	authURL, err := session.GetAuthURL()
	if err != nil {
		return "", "", err
	}
	authURL, err = appendQueryParam(authURL, "nonce", nonce)
	if err != nil {
		return "", "", err
	}

	return authURL, session.Marshal(), nil
}

func (c *googleOAuthClient) CompleteAuth(ctx context.Context, sessionData string, code string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	provider := *c.provider
	provider.HTTPClient = contextHTTPClient(ctx)

	session, err := provider.UnmarshalSession(sessionData)
	if err != nil {
		return "", err
	}
	if _, err = session.Authorize(&provider, url.Values{"code": []string{code}}); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", err
	}

	googleSession, ok := session.(*gothGoogle.Session)
	if !ok {
		return "", errors.New("unexpected google session type")
	}
	if strings.TrimSpace(googleSession.IDToken) == "" {
		return "", errors.New("id_token missing from oauth response")
	}
	return googleSession.IDToken, nil
}

func contextHTTPClient(ctx context.Context) *http.Client {
	return &http.Client{
		Transport: contextRoundTripper{ctx: ctx, base: http.DefaultTransport},
	}
}

type contextRoundTripper struct {
	ctx  context.Context
	base http.RoundTripper
}

func (rt contextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.base.RoundTrip(req.WithContext(rt.ctx))
}

func NewAuthController(
	authService services.AuthService,
	oauthClient GoogleOAuthClient,
	authConfig config.AuthConfig,
) *AuthController {
	if strings.TrimSpace(authConfig.SessionCookieSecret) == "" {
		panic("auth controller dependency AuthConfig.SessionCookieSecret is required")
	}
	return &AuthController{
		authService: authService,
		oauthClient: oauthClient,
		authConfig:  authConfig,
		now:         time.Now,
	}
}

func (c *AuthController) StartGoogleLogin(ctx *gin.Context) {
	returnTo, err := validateReturnTo(ctx.Query("return_to"))
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		httperr.WriteError(ctx, domain.NewErrServiceUnavailable("не удалось создать oauth state"))
		return
	}
	nonce, err := randomToken(32)
	if err != nil {
		httperr.WriteError(ctx, domain.NewErrServiceUnavailable("не удалось создать oauth nonce"))
		return
	}

	authURL, oauthSession, err := c.oauthClient.BeginAuth(state, nonce)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	cookieValue, err := buildOAuthStateCookie(oauthStatePayload{
		State:        state,
		Nonce:        nonce,
		ReturnTo:     returnTo,
		OAuthSession: oauthSession,
		ExpiresAt:    c.now().UTC().Add(oauthStateTTL).Unix(),
	}, c.authConfig.SessionCookieSecret)
	if err != nil {
		httperr.WriteError(ctx, domain.NewErrServiceUnavailable("не удалось сохранить oauth state"))
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		oauthStateCookieName,
		cookieValue,
		int(oauthStateTTL.Seconds()),
		"/",
		c.authConfig.SessionCookieDomain,
		c.authConfig.SessionCookieSecure,
		true,
	)
	ctx.Redirect(http.StatusFound, authURL)
}

func (c *AuthController) HandleGoogleCallback(ctx *gin.Context) {
	if strings.TrimSpace(ctx.Query("error")) != "" {
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
		return
	}

	code := strings.TrimSpace(ctx.Query("code"))
	state := strings.TrimSpace(ctx.Query("state"))
	if code == "" || state == "" {
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
		return
	}

	stateCookie, err := ctx.Cookie(oauthStateCookieName)
	if err != nil {
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthStateInvalid, "Некорректный или просроченный OAuth state")
		return
	}

	statePayload, err := parseOAuthStateCookie(stateCookie, c.authConfig.SessionCookieSecret)
	if err != nil || c.now().UTC().Unix() >= statePayload.ExpiresAt || statePayload.State != state {
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthStateInvalid, "Некорректный или просроченный OAuth state")
		return
	}

	oauthCtx, cancel := context.WithTimeout(ctx.Request.Context(), oauthCallbackTimeout)
	defer cancel()

	idToken, err := c.oauthClient.CompleteAuth(oauthCtx, statePayload.OAuthSession, code)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			httperr.WriteError(ctx, domain.NewErrServiceUnavailable("не удалось завершить вход через Google"))
			return
		}
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
		return
	}

	identity, err := c.authService.GoogleIdentityFromIDToken(
		oauthCtx,
		idToken,
		statePayload.Nonce,
	)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	user, err := c.authService.UpsertGoogleUser(ctx.Request.Context(), identity, c.now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
			return
		}
		httperr.WriteError(ctx, err)
		return
	}

	sessionToken, _, err := c.authService.CreateSession(
		ctx.Request.Context(),
		user.ID,
		c.authConfig.SessionTTL,
		c.now().UTC(),
		map[string]any{
			"user_agent": ctx.Request.UserAgent(),
			"ip":         ctx.ClientIP(),
		},
	)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	c.setSessionCookie(ctx, sessionToken, int(c.authConfig.SessionTTL.Seconds()))
	middleware.SetCSRFCookie(ctx, c.authConfig, sessionToken, c.now().UTC())
	c.clearCookie(ctx, oauthStateCookieName, true)
	ctx.Redirect(http.StatusFound, statePayload.ReturnTo)
}

func (c *AuthController) GetCurrentUser(ctx *gin.Context) {
	user, ok := middleware.CurrentUser(ctx)
	if !ok {
		httperr.WriteError(ctx, domain.NewErrUnauthorized("требуется аутентификация"))
		return
	}

	ctx.JSON(http.StatusOK, currentUserResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
	})
}

func (c *AuthController) Logout(ctx *gin.Context) {
	rawToken, err := ctx.Cookie(c.authConfig.SessionCookieName)
	if err != nil {
		httperr.WriteError(ctx, domain.NewErrUnauthorized("требуется аутентификация"))
		return
	}

	if err = c.authService.RevokeSessionByRawToken(ctx.Request.Context(), rawToken, c.now().UTC()); err != nil {
		// Даже при ошибке отзыва очищаем client-side cookies, чтобы клиент не зацикливался
		// на отправке заведомо невалидной сессии.
		c.clearCookie(ctx, c.authConfig.SessionCookieName, true)
		c.clearCookie(ctx, c.authConfig.CSRFCookieName, false)
		httperr.WriteError(ctx, err)
		return
	}

	c.clearCookie(ctx, c.authConfig.SessionCookieName, true)
	c.clearCookie(ctx, c.authConfig.CSRFCookieName, false)
	ctx.Status(http.StatusNoContent)
}

func (c *AuthController) setSessionCookie(ctx *gin.Context, value string, maxAge int) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		c.authConfig.SessionCookieName,
		value,
		maxAge,
		"/",
		c.authConfig.SessionCookieDomain,
		c.authConfig.SessionCookieSecure,
		true,
	)
}

func (c *AuthController) clearCookie(ctx *gin.Context, name string, httpOnly bool) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, "", -1, "/", c.authConfig.SessionCookieDomain, c.authConfig.SessionCookieSecure, httpOnly)
}

func validateReturnTo(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "/", nil
	}
	if len(raw) > returnToMaxLength {
		return "", domain.NewErrInvalidAuthData("параметр return_to не должен превышать 2048 символов")
	}
	if !returnToPattern.MatchString(raw) {
		return "", domain.NewErrInvalidAuthData("параметр return_to должен быть относительным путем")
	}
	return raw, nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildOAuthStateCookie(payload oauthStatePayload, secret string) (string, error) {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(encodedPayload)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err = mac.Write([]byte(payloadPart)); err != nil {
		return "", err
	}
	signaturePart := hex.EncodeToString(mac.Sum(nil))
	return payloadPart + "." + signaturePart, nil
}

func parseOAuthStateCookie(value string, secret string) (oauthStatePayload, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return oauthStatePayload{}, errors.New("invalid oauth state cookie format")
	}
	payloadPart := parts[0]
	signaturePart := parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(payloadPart)); err != nil {
		return oauthStatePayload{}, err
	}
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signaturePart), []byte(expectedSignature)) {
		return oauthStatePayload{}, errors.New("invalid oauth state cookie signature")
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return oauthStatePayload{}, err
	}
	var payload oauthStatePayload
	if err = json.Unmarshal(rawPayload, &payload); err != nil {
		return oauthStatePayload{}, err
	}
	if payload.State == "" || payload.Nonce == "" || payload.ReturnTo == "" || payload.OAuthSession == "" {
		return oauthStatePayload{}, errors.New("oauth state cookie payload is incomplete")
	}
	return payload, nil
}

func respondAuthFlowError(c *gin.Context, status int, code string, message string) {
	httperr.Write(c, status, code, message)
}

func appendQueryParam(rawURL string, key string, value string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set(key, value)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}
