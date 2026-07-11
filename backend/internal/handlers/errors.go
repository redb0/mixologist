package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
)

const internalServerErrorMessage = "внутренняя ошибка сервера"

// MapError переводит domain/repo ошибки в HTTP-статус и безопасное сообщение клиенту.
func MapError(err error) (status int, clientMessage string) {
	switch {
	case errors.Is(err, domain.ErrInvalidIngredientData):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusInternalServerError, internalServerErrorMessage
	}
}

// RespondError пишет JSON {"error": ...}; для 5xx логирует полную цепочку.
func RespondError(c *gin.Context, err error) {
	status, msg := MapError(err)
	if status >= http.StatusInternalServerError {
		slog.Error("internal error", "err", err)
	}
	c.JSON(status, gin.H{"error": msg})
}
