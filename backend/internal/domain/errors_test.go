package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
)

func TestDomainErrors_IsAsAndMessage(t *testing.T) {
	const msg = "детальное сообщение"

	tests := []struct {
		name     string
		newErr   func(string) error
		sentinel error
		checkAs  func(t *testing.T, err error, wantMsg string)
	}{
		{
			name:     "NotFound",
			newErr:   domain.NewErrNotFound,
			sentinel: domain.ErrNotFound,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.NotFoundError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → NotFoundError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "AlreadyExists",
			newErr:   domain.NewErrAlreadyExists,
			sentinel: domain.ErrAlreadyExists,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.AlreadyExistsError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → AlreadyExistsError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "VersionConflict",
			newErr:   domain.NewErrVersionConflict,
			sentinel: domain.ErrVersionConflict,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.VersionConflictError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → VersionConflictError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "InvalidIngredientData",
			newErr:   domain.NewErrInvalidIngredientData,
			sentinel: domain.ErrInvalidIngredientData,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.InvalidIngredientDataError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → InvalidIngredientDataError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "InvalidPageToken",
			newErr:   domain.NewErrInvalidPageToken,
			sentinel: domain.ErrInvalidPageToken,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.InvalidPageTokenError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → InvalidPageTokenError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "InvalidID",
			newErr:   domain.NewErrInvalidID,
			sentinel: domain.ErrInvalidID,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.InvalidIDError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → InvalidIDError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "ResourceInUse",
			newErr:   domain.NewErrResourceInUse,
			sentinel: domain.ErrResourceInUse,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.ResourceInUseError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → ResourceInUseError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "ServiceUnavailable",
			newErr:   domain.NewErrServiceUnavailable,
			sentinel: domain.ErrServiceUnavailable,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.ServiceUnavailableError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → ServiceUnavailableError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "Unauthorized",
			newErr:   domain.NewErrUnauthorized,
			sentinel: domain.ErrUnauthorized,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.UnauthorizedError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → UnauthorizedError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "Forbidden",
			newErr:   domain.NewErrForbidden,
			sentinel: domain.ErrForbidden,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.ForbiddenError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → ForbiddenError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
		{
			name:     "InvalidAuthData",
			newErr:   domain.NewErrInvalidAuthData,
			sentinel: domain.ErrInvalidAuthData,
			checkAs: func(t *testing.T, err error, wantMsg string) {
				t.Helper()
				var target *domain.InvalidAuthDataError
				if !errors.As(err, &target) || target.Message != wantMsg {
					t.Fatalf("errors.As → InvalidAuthDataError: got %+v, want Message=%q", target, wantMsg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.newErr(msg)

			if err.Error() != msg {
				t.Fatalf("Error(): got %q, want %q", err.Error(), msg)
			}
			if !errors.Is(err, tt.sentinel) {
				t.Fatalf("errors.Is(err, %v) = false", tt.sentinel)
			}

			wrapped := fmt.Errorf("слой выше: %w", err)
			if !errors.Is(wrapped, tt.sentinel) {
				t.Fatalf("errors.Is(wrapped, %v) = false", tt.sentinel)
			}

			tt.checkAs(t, wrapped, msg)
		})
	}
}
