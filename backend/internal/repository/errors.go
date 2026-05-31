package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "connection refused") {
		return ErrConnectionFailed
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique violation
			return fmt.Errorf("%w: %s", domain.NewErrAlreadyExists("запись уже существует"), pgErr.Detail)
		case "23503": // foreign key violation
			return fmt.Errorf("%w: %s", ErrForeignKeyViolation, pgErr.Detail)
		case "23514": // check violation
			return fmt.Errorf("%w: %s", ErrCheckViolation, pgErr.Detail)
		case "40P01":
			return fmt.Errorf("%w: %s", ErrDeadlock, pgErr.Detail)
		case "57014": // query canceled (timeout)
			return fmt.Errorf("%w: %s", ErrQueryCanceled, pgErr.Detail)
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ErrQueryCanceled
	}
	if errors.Is(err, context.Canceled) {
		return ErrQueryCanceled
	}

	return err
}
