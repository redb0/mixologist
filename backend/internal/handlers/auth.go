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

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	gothGoogle "github.com/markbates/goth/providers/google"
	"google.golang.org/api/idtoken"

	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/middleware"
	"github.com/redb0/mixologist/internal/services"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateTTL        = 10 * time.Minute
	returnToMaxLength    = 2048

	CodeOAuthStateInvalid   = "OAUTH_STATE_INVALID"
	CodeOAuthCallbackFailed = "OAUTH_CALLBACK_FAILED"
	CodeCSRFTokenInvalid    = "CSRF_TOKEN_INVALID"
)

var returnToPattern = regexp.MustCompile(`^/(?:$|[^/\\\s][^\\\s]*)$`)

type AuthController struct {
	authService      services.AuthService
	oauthClient      GoogleOAuthClient
	idTokenValidator GoogleIDTokenValidator
	authConfig       config.AuthConfig
	now              func() time.Time
}

type GoogleOAuthClient interface {
	BeginAuth(state string, nonce string) (authURL string, sessionData string, err error)
	CompleteAuth(ctx context.Context, sessionData string, code string) (goth.User, error)
}

type googleOAuthClient struct {
	provider *gothGoogle.Provider
}

type GoogleIDTokenValidator interface {
	Validate(ctx context.Context, idToken string, audience string) (googleIDTokenClaims, error)
}

type googleIDTokenValidator struct{}

type googleIDTokenClaims struct {
	Subject       string
	Nonce         string
	EmailVerified bool
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

func NewGoogleIDTokenValidator() GoogleIDTokenValidator {
	return googleIDTokenValidator{}
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

func (c *googleOAuthClient) CompleteAuth(ctx context.Context, sessionData string, code string) (goth.User, error) {
	session, err := c.provider.UnmarshalSession(sessionData)
	if err != nil {
		return goth.User{}, err
	}
	if _, err = session.Authorize(c.provider, url.Values{"code": []string{code}}); err != nil {
		return goth.User{}, err
	}
	return c.provider.FetchUser(session)
}

func (v googleIDTokenValidator) Validate(
	ctx context.Context,
	rawIDToken string,
	audience string,
) (googleIDTokenClaims, error) {
	payload, err := idtoken.Validate(ctx, rawIDToken, audience)
	if err != nil {
		return googleIDTokenClaims{}, err
	}

	nonce, _ := payload.Claims["nonce"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	return googleIDTokenClaims{
		Subject:       payload.Subject,
		Nonce:         nonce,
		EmailVerified: emailVerified,
	}, nil
}

func NewAuthController(
	authService services.AuthService,
	oauthClient GoogleOAuthClient,
	authConfig config.AuthConfig,
) *AuthController {
	return &AuthController{
		authService:      authService,
		oauthClient:      oauthClient,
		idTokenValidator: NewGoogleIDTokenValidator(),
		authConfig:       authConfig,
		now:              time.Now,
	}
}

func (c *AuthController) StartGoogleLogin(ctx *gin.Context) {
	returnTo, err := validateReturnTo(ctx.Query("return_to"))
	if err != nil {
		RespondError(ctx, err)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		RespondError(ctx, domain.NewErrServiceUnavailable("не удалось создать oauth state"))
		return
	}
	nonce, err := randomToken(32)
	if err != nil {
		RespondError(ctx, domain.NewErrServiceUnavailable("не удалось создать oauth nonce"))
		return
	}

	authURL, oauthSession, err := c.oauthClient.BeginAuth(state, nonce)
	if err != nil {
		RespondError(ctx, err)
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
		RespondError(ctx, domain.NewErrServiceUnavailable("не удалось сохранить oauth state"))
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

	userData, err := c.oauthClient.CompleteAuth(ctx.Request.Context(), statePayload.OAuthSession, code)
	if err != nil {
		respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
		return
	}

	identity, err := validateGoogleUserData(
		ctx.Request.Context(),
		c.idTokenValidator,
		userData,
		statePayload.Nonce,
		c.authConfig.GoogleClientID,
	)
	if err != nil {
		RespondError(ctx, err)
		return
	}

	user, err := c.authService.UpsertGoogleUser(ctx.Request.Context(), identity, c.now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			respondAuthFlowError(ctx, http.StatusBadRequest, CodeOAuthCallbackFailed, "Не удалось завершить вход через Google")
			return
		}
		RespondError(ctx, err)
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
		RespondError(ctx, err)
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
		RespondError(ctx, domain.NewErrUnauthorized("требуется аутентификация"))
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
		RespondError(ctx, domain.NewErrUnauthorized("требуется аутентификация"))
		return
	}

	if err = c.authService.RevokeSessionByRawToken(ctx.Request.Context(), rawToken, c.now().UTC()); err != nil {
		// Даже при ошибке отзыва очищаем client-side cookies, чтобы клиент не зацикливался
		// на отправке заведомо невалидной сессии.
		c.clearCookie(ctx, c.authConfig.SessionCookieName, true)
		c.clearCookie(ctx, c.authConfig.CSRFCookieName, false)
		RespondError(ctx, err)
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

func validateGoogleUserData(
	ctx context.Context,
	validator GoogleIDTokenValidator,
	user goth.User,
	expectedNonce string,
	expectedAudience string,
) (domain.GoogleIdentity, error) {
	claims, err := validator.Validate(ctx, user.IDToken, expectedAudience)
	if err != nil {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if strings.TrimSpace(claims.Nonce) == "" || claims.Nonce != expectedNonce {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if !claims.EmailVerified {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if strings.TrimSpace(claims.Subject) == "" ||
		strings.TrimSpace(user.UserID) == "" ||
		strings.TrimSpace(user.Email) == "" ||
		strings.TrimSpace(user.Name) == "" {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if strings.TrimSpace(user.UserID) != claims.Subject {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}

	return domain.GoogleIdentity{
		Subject:     claims.Subject,
		Email:       strings.ToLower(strings.TrimSpace(user.Email)),
		DisplayName: strings.TrimSpace(user.Name),
		AvatarURL:   strings.TrimSpace(user.AvatarURL),
	}, nil
}

func respondAuthFlowError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestid.Get(c),
		},
	})
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
