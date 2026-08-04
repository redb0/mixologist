package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
)

const internalServerErrorMessage = "Внутренняя ошибка сервера"

const (
	CodeValidationError    = "VALIDATION_ERROR"
	CodeInvalidID          = "INVALID_ID"
	CodeInvalidPageToken   = "INVALID_PAGE_TOKEN"
	CodeNotFound           = "NOT_FOUND"
	CodeAlreadyExists      = "ALREADY_EXISTS"
	CodeVersionConflict    = "VERSION_CONFLICT"
	CodeResourceInUse      = "RESOURCE_IN_USE"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ErrorBody struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id"`
	Details   []ErrorDetail `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type mappedError struct {
	status  int
	code    string
	message string
}

// MapError переводит domain/repo ошибки в HTTP-статус, код и безопасное сообщение.
func MapError(err error) (status int, code string, message string) {
	mapped := mapError(err)
	return mapped.status, mapped.code, mapped.message
}

func mapError(err error) mappedError {
	var invalidID *domain.InvalidIDError
	if errors.As(err, &invalidID) {
		return mappedError{http.StatusBadRequest, CodeInvalidID, invalidID.Message}
	}

	var invalidPageToken *domain.InvalidPageTokenError
	if errors.As(err, &invalidPageToken) {
		return mappedError{http.StatusBadRequest, CodeInvalidPageToken, invalidPageToken.Message}
	}

	var invalidData *domain.InvalidIngredientDataError
	if errors.As(err, &invalidData) {
		return mappedError{http.StatusBadRequest, CodeValidationError, invalidData.Message}
	}

	var alreadyExists *domain.AlreadyExistsError
	if errors.As(err, &alreadyExists) {
		return mappedError{http.StatusConflict, CodeAlreadyExists, alreadyExists.Message}
	}

	var versionConflict *domain.VersionConflictError
	if errors.As(err, &versionConflict) {
		return mappedError{http.StatusConflict, CodeVersionConflict, versionConflict.Message}
	}

	var resourceInUse *domain.ResourceInUseError
	if errors.As(err, &resourceInUse) {
		return mappedError{http.StatusConflict, CodeResourceInUse, resourceInUse.Message}
	}

	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return mappedError{http.StatusNotFound, CodeNotFound, notFound.Message}
	}

	var serviceUnavailable *domain.ServiceUnavailableError
	if errors.As(err, &serviceUnavailable) {
		return mappedError{http.StatusServiceUnavailable, CodeServiceUnavailable, serviceUnavailable.Message}
	}

	return mappedError{http.StatusInternalServerError, CodeInternalError, internalServerErrorMessage}
}

// RespondError пишет structured error envelope; для 5xx логирует полную цепочку.
func RespondError(c *gin.Context, err error) {
	requestID := requestid.Get(c)
	status, code, message := MapError(err)
	if status >= http.StatusInternalServerError {
		slog.Error("internal error", "err", err, "request_id", requestID)
	}
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}
