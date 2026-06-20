package services

import (
	"context"
	"errors"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
)

type mockIngredientRepo struct {
	getByID func(ctx context.Context, id uint) (*domain.Ingredient, error)
}

func (m *mockIngredientRepo) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	return m.getByID(ctx, id)
}

func (m *mockIngredientRepo) Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
	panic("unexpected call")
}

func (m *mockIngredientRepo) Update(ctx context.Context, ingredient *domain.Ingredient) error {
	panic("unexpected call")
}

func (m *mockIngredientRepo) Delete(ctx context.Context, id uint) error {
	panic("unexpected call")
}

func (m *mockIngredientRepo) List(ctx context.Context) ([]*domain.Ingredient, error) {
	panic("unexpected call")
}

func TestIngredientService_GetByID(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return &domain.Ingredient{ID: id}, nil
		},
	}
	service := NewIngredientService(repo)

	// Проверяем правильное делегирование в репозиторий
	ingredient, err := service.GetByID(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, uint(1), ingredient.ID)
}

func TestIngredientService_GetByID_NotFound(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return nil, domain.NewErrNotFound("Ингредиент не найден")
		},
	}
	service := NewIngredientService(repo)

	// Проверяем правильное возвращение ошибки NotFound
	ingredient, err := service.GetByID(context.Background(), 1)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Nil(t, ingredient)
}
