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
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "invalid ingredient data",
			err:        domain.NewErrInvalidIngredientData("нет полей для обновления"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeValidationError,
			wantMsg:    "нет полей для обновления",
		},
		{
			name:       "invalid id",
			err:        domain.NewErrInvalidID("Неверный ID ингредиента"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeInvalidID,
			wantMsg:    "Неверный ID ингредиента",
		},
		{
			name:       "invalid page token",
			err:        domain.NewErrInvalidPageToken("Некорректный или несовместимый pageToken"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeInvalidPageToken,
			wantMsg:    "Некорректный или несовместимый pageToken",
		},
		{
			name:       "already exists",
			err:        domain.NewErrAlreadyExists("запись уже существует"),
			wantStatus: http.StatusConflict,
			wantCode:   CodeAlreadyExists,
			wantMsg:    "запись уже существует",
		},
		{
			name:       "version conflict",
			err:        domain.NewErrVersionConflict("конфликт версии ингредиента"),
			wantStatus: http.StatusConflict,
			wantCode:   CodeVersionConflict,
			wantMsg:    "конфликт версии ингредиента",
		},
		{
			name:       "resource in use",
			err:        domain.NewErrResourceInUse("Ингредиент используется и не может быть удалён"),
			wantStatus: http.StatusConflict,
			wantCode:   CodeResourceInUse,
			wantMsg:    "Ингредиент используется и не может быть удалён",
		},
		{
			name:       "not found",
			err:        domain.NewErrNotFound("Ингредиент не найден"),
			wantStatus: http.StatusNotFound,
			wantCode:   CodeNotFound,
			wantMsg:    "Ингредиент не найден",
		},
		{
			name:       "service unavailable",
			err:        domain.NewErrServiceUnavailable("Сервис временно недоступен"),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeServiceUnavailable,
			wantMsg:    "Сервис временно недоступен",
		},
		{
			name:       "unknown error",
			err:        errors.New("connection reset by peer"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalError,
			wantMsg:    internalServerErrorMessage,
		},
		{
			name:       "unknown wrapped",
			err:        fmt.Errorf("db: %w", errors.New("timeout")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalError,
			wantMsg:    internalServerErrorMessage,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := MapError(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}
