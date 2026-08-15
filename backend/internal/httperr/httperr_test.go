package httperr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
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
			wantCode:   httperr.CodeValidationError,
			wantMsg:    "нет полей для обновления",
		},
		{
			name:       "invalid auth data",
			err:        domain.NewErrInvalidAuthData("email обязателен"),
			wantStatus: http.StatusBadRequest,
			wantCode:   httperr.CodeValidationError,
			wantMsg:    "email обязателен",
		},
		{
			name:       "invalid id",
			err:        domain.NewErrInvalidID("Неверный ID ингредиента"),
			wantStatus: http.StatusBadRequest,
			wantCode:   httperr.CodeInvalidID,
			wantMsg:    "Неверный ID ингредиента",
		},
		{
			name:       "invalid page token",
			err:        domain.NewErrInvalidPageToken("Некорректный или несовместимый pageToken"),
			wantStatus: http.StatusBadRequest,
			wantCode:   httperr.CodeInvalidPageToken,
			wantMsg:    "Некорректный или несовместимый pageToken",
		},
		{
			name:       "already exists",
			err:        domain.NewErrAlreadyExists("запись уже существует"),
			wantStatus: http.StatusConflict,
			wantCode:   httperr.CodeAlreadyExists,
			wantMsg:    "запись уже существует",
		},
		{
			name:       "version conflict",
			err:        domain.NewErrVersionConflict("конфликт версии ингредиента"),
			wantStatus: http.StatusConflict,
			wantCode:   httperr.CodeVersionConflict,
			wantMsg:    "конфликт версии ингредиента",
		},
		{
			name:       "resource in use",
			err:        domain.NewErrResourceInUse("Ингредиент используется и не может быть удалён"),
			wantStatus: http.StatusConflict,
			wantCode:   httperr.CodeResourceInUse,
			wantMsg:    "Ингредиент используется и не может быть удалён",
		},
		{
			name:       "unauthorized",
			err:        domain.NewErrUnauthorized("сессия недействительна"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   httperr.CodeUnauthorized,
			wantMsg:    "сессия недействительна",
		},
		{
			name:       "forbidden",
			err:        domain.NewErrForbidden("доступ запрещен"),
			wantStatus: http.StatusForbidden,
			wantCode:   httperr.CodeForbidden,
			wantMsg:    "доступ запрещен",
		},
		{
			name:       "not found",
			err:        domain.NewErrNotFound("Ингредиент не найден"),
			wantStatus: http.StatusNotFound,
			wantCode:   httperr.CodeNotFound,
			wantMsg:    "Ингредиент не найден",
		},
		{
			name:       "service unavailable",
			err:        domain.NewErrServiceUnavailable("Сервис временно недоступен"),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   httperr.CodeServiceUnavailable,
			wantMsg:    "Сервис временно недоступен",
		},
		{
			name:       "unknown error",
			err:        errors.New("connection reset by peer"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   httperr.CodeInternalError,
			wantMsg:    httperr.InternalServerErrorMessage,
		},
		{
			name:       "unknown wrapped",
			err:        fmt.Errorf("db: %w", errors.New("timeout")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   httperr.CodeInternalError,
			wantMsg:    httperr.InternalServerErrorMessage,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := httperr.Map(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

func TestWriteAndWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("X-Request-ID", "req-1")

	httperr.Write(c, http.StatusBadRequest, httperr.CodeValidationError, "bad input")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body httperr.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, httperr.CodeValidationError, body.Error.Code)
	assert.Equal(t, "bad input", body.Error.Message)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	httperr.WriteError(c, domain.NewErrNotFound("не найдено"))

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, httperr.CodeNotFound, body.Error.Code)
}

func TestAbortAndAbortError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	httperr.Abort(c, http.StatusForbidden, httperr.CodeForbidden, "запрещено")

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
	var body httperr.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, httperr.CodeForbidden, body.Error.Code)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	httperr.AbortError(c, domain.NewErrUnauthorized("нет доступа"))

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, httperr.CodeUnauthorized, body.Error.Code)
}
