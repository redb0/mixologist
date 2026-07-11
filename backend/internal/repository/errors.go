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

var (
	ErrForeignKeyViolation = errors.New("нарушение внешнего ключа")
	ErrCheckViolation      = errors.New("нарушение ограничения CHECK")
	ErrDeadlock            = errors.New("ошибка deadlock")
	ErrQueryCanceled       = errors.New("запрос отменен")
	ErrConnectionFailed    = errors.New("не удалось установить соединение с базой данных")
)

func ParseDBError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewErrNotFound("запись не найдена")
	}

	// Отмена / дедлайн запроса (typed context и строковые обёртки драйверов).
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "context canceled") ||
		strings.Contains(err.Error(), "deadline exceeded") {
		return ErrQueryCanceled
	}

	// Проблемы соединения с БД.
	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection refused") {
		return ErrConnectionFailed
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
			return ErrForeignKeyViolation
		case "23514": // check violation
			return ErrCheckViolation
		case "40P01":
			return ErrDeadlock
		case "57014": // query canceled (timeout)
			return ErrQueryCanceled
		}
	}

	return err
}
