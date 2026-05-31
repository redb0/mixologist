package domain

import "errors"

// ErrNotFound — sentinel для ошибок «ресурс не найден» (errors.Is(err, ErrNotFound)).
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")

	ErrInvalidIngredientData = errors.New("invalid ingredient data")
)

// NotFoundError несёт человекочитаемое сообщение и обёртывает ErrNotFound для errors.Is после fmt.Errorf(..., %w).
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// NewErrNotFound возвращает ошибку с сообщением для логов/ответа API и с возможностью
// проверки errors.Is(..., ErrNotFound) или errors.As(..., (*NotFoundError)(nil)).
func NewErrNotFound(message string) error {
	return &NotFoundError{Message: message}
}

type AlreadyExistsError struct {
	Message string
}

func (e *AlreadyExistsError) Error() string {
	return e.Message
}

func (e *AlreadyExistsError) Unwrap() error {
	return ErrAlreadyExists
}

func NewErrAlreadyExists(message string) error {
	return &AlreadyExistsError{Message: message}
}

type InvalidIngredientDataError struct {
	Message string
}

func (e *InvalidIngredientDataError) Error() string {
	return e.Message
}

func (e *InvalidIngredientDataError) Unwrap() error {
	return ErrInvalidIngredientData
}

func NewErrInvalidIngredientData(message string) error {
	return &InvalidIngredientDataError{Message: message}
}
