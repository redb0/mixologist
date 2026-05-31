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
