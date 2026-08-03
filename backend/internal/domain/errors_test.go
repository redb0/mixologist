package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
)

func TestNewErrNotFound_IsUnwrapped(t *testing.T) {
	err := domain.NewErrNotFound("missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("errors.Is(..., ErrNotFound) должна быть истинной для ошибки без обёртки")
	}
}

func TestNewErrNotFound_IsWrappedFmt(t *testing.T) {
	err := fmt.Errorf("context: %w", domain.NewErrNotFound("missing"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("errors.Is должна находить ErrNotFound в цепочке fmt.Errorf %%w")
	}
}

func TestNewErrNotFound_AsMessage(t *testing.T) {
	inner := domain.NewErrNotFound("нет такого ресурса")
	wrapped := fmt.Errorf("слой выше: %w", inner)
	var nf *domain.NotFoundError
	if !errors.As(wrapped, &nf) || nf.Message != "нет такого ресурса" {
		t.Fatalf("ожидали извлечь NotFoundError с Message через errors.As; got %+v", nf)
	}
}

func TestNewErrVersionConflict_IsUnwrapped(t *testing.T) {
	err := domain.NewErrVersionConflict("stale version")
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatal("errors.Is(..., ErrVersionConflict) должна быть истинной для ошибки без обёртки")
	}
}

func TestNewErrVersionConflict_AsMessage(t *testing.T) {
	inner := domain.NewErrVersionConflict("конфликт версии ингредиента")
	wrapped := fmt.Errorf("слой выше: %w", inner)
	var conflict *domain.VersionConflictError
	if !errors.As(wrapped, &conflict) || conflict.Message != "конфликт версии ингредиента" {
		t.Fatalf("ожидали извлечь VersionConflictError с Message через errors.As; got %+v", conflict)
	}
}
