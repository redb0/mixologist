package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/lib/pq"
	"github.com/redb0/mixologist/internal/domain"
)

const (
	serviceUnavailableMessage = "сервис временно недоступен"
	validationDataMessage     = "некорректные данные запроса"
	resourceInUseMessage      = "ингредиент используется и не может быть удалён"
)

func ParseDBError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewErrNotFound("запись не найдена")
	}

	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "context canceled") ||
		strings.Contains(err.Error(), "deadline exceeded") {
		return domain.NewErrServiceUnavailable(serviceUnavailableMessage)
	}

	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection refused") {
		return domain.NewErrServiceUnavailable(serviceUnavailableMessage)
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		if pgErr.Detail != "" {
			slog.Debug("postgres error detail", "code", string(pgErr.Code), "detail", pgErr.Detail)
		}
		switch pgErr.Code {
		case "23505": // unique violation
			return domain.NewErrAlreadyExists("запись уже существует")
		case "23503": // foreign key violation
			return domain.NewErrResourceInUse(resourceInUseMessage)
		case "23514": // check violation
			return domain.NewErrInvalidIngredientData(validationDataMessage)
		case "40P01":
			return domain.NewErrServiceUnavailable(serviceUnavailableMessage)
		case "57014": // query canceled (timeout)
			return domain.NewErrServiceUnavailable(serviceUnavailableMessage)
		}
	}

	return err
}
