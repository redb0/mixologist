package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"google.golang.org/api/idtoken"

	"github.com/redb0/mixologist/internal/domain"
)

// IDTokenClaims — проверенные claims Google ID token.
type IDTokenClaims struct {
	Subject       string
	Nonce         string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// IDTokenValidator проверяет подпись и стандартные claims Google ID token.
type IDTokenValidator interface {
	Validate(ctx context.Context, rawIDToken string, audience string) (IDTokenClaims, error)
}

type googleIDTokenValidator struct{}

func NewGoogleIDTokenValidator() IDTokenValidator {
	return googleIDTokenValidator{}
}

func (v googleIDTokenValidator) Validate(
	ctx context.Context,
	rawIDToken string,
	audience string,
) (IDTokenClaims, error) {
	payload, err := idtoken.Validate(ctx, rawIDToken, audience)
	if err != nil {
		return IDTokenClaims{}, mapIDTokenValidationError(err)
	}

	nonce, _ := payload.Claims["nonce"].(string)
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	return IDTokenClaims{
		Subject:       payload.Subject,
		Nonce:         nonce,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
		Picture:       picture,
	}, nil
}

func (s *authService) GoogleIdentityFromIDToken(
	ctx context.Context,
	rawIDToken string,
	expectedNonce string,
) (domain.GoogleIdentity, error) {
	claims, err := s.idTokenValidator.Validate(ctx, rawIDToken, s.googleClientID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrServiceUnavailable) {
			return domain.GoogleIdentity{}, err
		}
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if strings.TrimSpace(claims.Nonce) == "" || claims.Nonce != expectedNonce {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if !claims.EmailVerified {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}
	if strings.TrimSpace(claims.Subject) == "" ||
		strings.TrimSpace(claims.Email) == "" ||
		strings.TrimSpace(claims.Name) == "" {
		return domain.GoogleIdentity{}, domain.NewErrUnauthorized("невалидный identity token")
	}

	return domain.GoogleIdentity{
		Subject:     claims.Subject,
		Email:       normalizeEmail(claims.Email),
		DisplayName: strings.TrimSpace(claims.Name),
		AvatarURL:   strings.TrimSpace(claims.Picture),
	}, nil
}

func mapIDTokenValidationError(err error) error {
	if isIDTokenInfrastructureError(err) {
		return fmt.Errorf("%w: %v", domain.NewErrServiceUnavailable("не удалось проверить identity token"), err)
	}
	return domain.NewErrUnauthorized("невалидный identity token")
}

func isIDTokenInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return true
	}
	return strings.Contains(err.Error(), "idtoken: unable to retrieve cert")
}
