package domain

import "errors"

// ErrNotFound — sentinel для ошибок «ресурс не найден» (errors.Is(err, ErrNotFound)).
var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrVersionConflict = errors.New("version conflict")

	ErrInvalidIngredientData = errors.New("invalid ingredient data")
	ErrInvalidPageToken      = errors.New("invalid page token")
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

type VersionConflictError struct {
	Message string
}

func (e *VersionConflictError) Error() string {
	return e.Message
}

func (e *VersionConflictError) Unwrap() error {
	return ErrVersionConflict
}

func NewErrVersionConflict(message string) error {
	return &VersionConflictError{Message: message}
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

type InvalidPageTokenError struct {
	Message string
}

func (e *InvalidPageTokenError) Error() string {
	return e.Message
}

func (e *InvalidPageTokenError) Unwrap() error {
	return ErrInvalidPageToken
}

func NewErrInvalidPageToken(message string) error {
	return &InvalidPageTokenError{Message: message}
}
