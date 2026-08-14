package httperr

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
)

const InternalServerErrorMessage = "Внутренняя ошибка сервера"

const (
	CodeValidationError    = "VALIDATION_ERROR"
	CodeInvalidID          = "INVALID_ID"
	CodeInvalidPageToken   = "INVALID_PAGE_TOKEN"
	CodeNotFound           = "NOT_FOUND"
	CodeAlreadyExists      = "ALREADY_EXISTS"
	CodeVersionConflict    = "VERSION_CONFLICT"
	CodeResourceInUse      = "RESOURCE_IN_USE"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeCSRFTokenInvalid   = "CSRF_TOKEN_INVALID"
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

// Map переводит domain/repo ошибки в HTTP-статус, код и безопасное сообщение.
func Map(err error) (status int, code string, message string) {
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

	var invalidAuthData *domain.InvalidAuthDataError
	if errors.As(err, &invalidAuthData) {
		return mappedError{http.StatusBadRequest, CodeValidationError, invalidAuthData.Message}
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

	var unauthorized *domain.UnauthorizedError
	if errors.As(err, &unauthorized) {
		return mappedError{http.StatusUnauthorized, CodeUnauthorized, unauthorized.Message}
	}

	var forbidden *domain.ForbiddenError
	if errors.As(err, &forbidden) {
		return mappedError{http.StatusForbidden, CodeForbidden, forbidden.Message}
	}

	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		return mappedError{http.StatusNotFound, CodeNotFound, notFound.Message}
	}

	var serviceUnavailable *domain.ServiceUnavailableError
	if errors.As(err, &serviceUnavailable) {
		return mappedError{http.StatusServiceUnavailable, CodeServiceUnavailable, serviceUnavailable.Message}
	}

	return mappedError{http.StatusInternalServerError, CodeInternalError, InternalServerErrorMessage}
}

// Write пишет structured error envelope без Abort.
func Write(c *gin.Context, status int, code, message string) {
	write(c, false, status, code, message)
}

// WriteError пишет structured error envelope из domain-ошибки без Abort.
func WriteError(c *gin.Context, err error) {
	status, code, message := Map(err)
	logInternal(c, err, status)
	Write(c, status, code, message)
}

// Abort прерывает обработку и пишет structured error envelope.
func Abort(c *gin.Context, status int, code, message string) {
	write(c, true, status, code, message)
}

// AbortError прерывает обработку и пишет structured error envelope из domain-ошибки.
func AbortError(c *gin.Context, err error) {
	status, code, message := Map(err)
	logInternal(c, err, status)
	Abort(c, status, code, message)
}

func write(c *gin.Context, abort bool, status int, code, message string) {
	body := ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestid.Get(c),
		},
	}
	if abort {
		c.AbortWithStatusJSON(status, body)
		return
	}
	c.JSON(status, body)
}

func logInternal(c *gin.Context, err error, status int) {
	if status < http.StatusInternalServerError {
		return
	}
	slog.Error("internal error", "err", err, "request_id", requestid.Get(c))
}
