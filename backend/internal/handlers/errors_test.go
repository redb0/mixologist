package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMapError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "invalid ingredient data",
			err:        domain.NewErrInvalidIngredientData("нет полей для обновления"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "нет полей для обновления",
		},
		{
			name:       "invalid ingredient data wrapped",
			err:        fmt.Errorf("%w: %v", domain.NewErrInvalidIngredientData("неверные данные ингредиента"), errors.New("неверная крепость")),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "неверные данные ингредиента",
		},
		{
			name:       "already exists",
			err:        domain.NewErrAlreadyExists("запись уже существует"),
			wantStatus: http.StatusConflict,
			wantMsg:    "запись уже существует",
		},
		{
			name:       "already exists with pg detail wrap",
			err:        fmt.Errorf("%w: Key (name)=(Джин) already exists.", domain.NewErrAlreadyExists("запись уже существует")),
			wantStatus: http.StatusConflict,
			wantMsg:    "запись уже существует",
		},
		{
			name:       "not found",
			err:        domain.NewErrNotFound("Ингредиент не найден"),
			wantStatus: http.StatusNotFound,
			wantMsg:    "Ингредиент не найден",
		},
		{
			name:       "not found wrapped",
			err:        fmt.Errorf("repo: %w", domain.NewErrNotFound("Ингредиент не найден")),
			wantStatus: http.StatusNotFound,
			wantMsg:    "Ингредиент не найден",
		},
		{
			name:       "unknown error",
			err:        errors.New("connection reset by peer"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    internalServerErrorMessage,
		},
		{
			name:       "unknown wrapped",
			err:        fmt.Errorf("db: %w", errors.New("timeout")),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    internalServerErrorMessage,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			status, msg := MapError(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}
