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
// Сообщение берётся из typed domain-ошибки, а не из полной цепочки err.Error().
func MapError(err error) (status int, clientMessage string) {
	var invalidData *domain.InvalidIngredientDataError
	if errors.As(err, &invalidData) {
		return http.StatusBadRequest, invalidData.Message
	}

	var alreadyExists *domain.AlreadyExistsError
	if errors.As(err, &alreadyExists) {
		return http.StatusConflict, alreadyExists.Message
	}

	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return http.StatusNotFound, notFound.Message
	}

	return http.StatusInternalServerError, internalServerErrorMessage
}

// RespondError пишет JSON {"error": ...}; для 5xx логирует полную цепочку.
func RespondError(c *gin.Context, err error) {
	status, msg := MapError(err)
	if status >= http.StatusInternalServerError {
		slog.Error("internal error", "err", err)
	}
	c.JSON(status, gin.H{"error": msg})
}
