package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
)

type mockIDTokenValidator struct {
	validate func(ctx context.Context, rawIDToken string, audience string) (IDTokenClaims, error)
}

func (m *mockIDTokenValidator) Validate(
	ctx context.Context,
	rawIDToken string,
	audience string,
) (IDTokenClaims, error) {
	if m.validate == nil {
		panic("unexpected call to Validate")
	}
	return m.validate(ctx, rawIDToken, audience)
}

func newTestAuthService(
	users *mockUserRepository,
	sessions *mockSessionRepository,
	adminEmails []string,
	validator IDTokenValidator,
) AuthService {
	return NewAuthService(users, sessions, adminEmails, validator, "google-client-id")
}

func TestAuthService_GoogleIdentityFromIDToken(t *testing.T) {
	validClaims := IDTokenClaims{
		Subject:       "subject-1",
		Nonce:         "nonce-1",
		Email:         "User@Example.com",
		EmailVerified: true,
		Name:          " User ",
		Picture:       "https://example.com/avatar.jpg",
	}

	tests := []struct {
		name          string
		validator     IDTokenValidator
		idToken       string
		expectedNonce string
		expectError   bool
		wantErrIs     error
		wantSubject   string
		wantEmail     string
		wantName      string
		wantAvatar    string
	}{
		{
			name: "success",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					if idToken != "signed-id-token" {
						t.Fatalf("unexpected id token: %q", idToken)
					}
					if audience != "google-client-id" {
						t.Fatalf("unexpected audience: %q", audience)
					}
					return validClaims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			wantSubject:   "subject-1",
			wantEmail:     "user@example.com",
			wantName:      "User",
			wantAvatar:    "https://example.com/avatar.jpg",
		},
		{
			name: "invalid token signature",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					return IDTokenClaims{}, errors.New("invalid signature")
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
		{
			name: "google certs unavailable",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					return IDTokenClaims{}, domain.NewErrServiceUnavailable("не удалось проверить identity token")
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrServiceUnavailable,
		},
		{
			name: "nonce mismatch",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					return validClaims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "other-nonce",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
		{
			name: "empty nonce",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					claims := validClaims
					claims.Nonce = ""
					return claims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
		{
			name: "email not verified",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					claims := validClaims
					claims.EmailVerified = false
					return claims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
		{
			name: "empty email",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					claims := validClaims
					claims.Email = "   "
					return claims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
		{
			name: "empty name",
			validator: &mockIDTokenValidator{
				validate: func(ctx context.Context, idToken string, audience string) (IDTokenClaims, error) {
					claims := validClaims
					claims.Name = "   "
					return claims, nil
				},
			},
			idToken:       "signed-id-token",
			expectedNonce: "nonce-1",
			expectError:   true,
			wantErrIs:     domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestAuthService(&mockUserRepository{}, &mockSessionRepository{}, nil, tt.validator)
			identity, err := service.GoogleIdentityFromIDToken(context.Background(), tt.idToken, tt.expectedNonce)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("error mismatch: got %v want %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if identity.Subject != tt.wantSubject {
				t.Fatalf("subject mismatch: got %q want %q", identity.Subject, tt.wantSubject)
			}
			if identity.Email != tt.wantEmail {
				t.Fatalf("email mismatch: got %q want %q", identity.Email, tt.wantEmail)
			}
			if identity.DisplayName != tt.wantName {
				t.Fatalf("display name mismatch: got %q want %q", identity.DisplayName, tt.wantName)
			}
			if identity.AvatarURL != tt.wantAvatar {
				t.Fatalf("avatar mismatch: got %q want %q", identity.AvatarURL, tt.wantAvatar)
			}
		})
	}
}

func TestMapIDTokenValidationError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantErrIs error
	}{
		{
			name:      "deadline fetching google certs",
			err:       context.DeadlineExceeded,
			wantErrIs: domain.ErrServiceUnavailable,
		},
		{
			name: "network timeout",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: timeoutError{},
			},
			wantErrIs: domain.ErrServiceUnavailable,
		},
		{
			name: "url error wrapping network failure",
			err: &url.Error{
				Op:  "Get",
				URL: "https://www.googleapis.com/oauth2/v3/certs",
				Err: fmt.Errorf("connection refused"),
			},
			wantErrIs: domain.ErrServiceUnavailable,
		},
		{
			name:      "google cert endpoint status",
			err:       errors.New("idtoken: unable to retrieve cert, got status code 503"),
			wantErrIs: domain.ErrServiceUnavailable,
		},
		{
			name:      "malformed cert payload",
			err:       &json.SyntaxError{},
			wantErrIs: domain.ErrServiceUnavailable,
		},
		{
			name:      "expired token",
			err:       errors.New("idtoken: token expired: now=1, expires=0"),
			wantErrIs: domain.ErrUnauthorized,
		},
		{
			name:      "audience mismatch",
			err:       errors.New("idtoken: audience provided does not match aud claim in the JWT"),
			wantErrIs: domain.ErrUnauthorized,
		},
		{
			name:      "invalid signature",
			err:       errors.New("crypto/rsa: verification error"),
			wantErrIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapIDTokenValidationError(tt.err)
			if !errors.Is(got, tt.wantErrIs) {
				t.Fatalf("error mismatch: got %v want %v", got, tt.wantErrIs)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
