package services

import (
	"context"
	"errors"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockIngredientRepo struct {
	getByID func(ctx context.Context, id uint) (*domain.Ingredient, error)
	update  func(ctx context.Context, ingredient *domain.Ingredient) error
}

func (m *mockIngredientRepo) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	if m.getByID == nil {
		panic("unexpected call to GetByID")
	}
	return m.getByID(ctx, id)
}

func (m *mockIngredientRepo) Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
	panic("unexpected call")
}

func (m *mockIngredientRepo) Update(ctx context.Context, ingredient *domain.Ingredient) error {
	if m.update == nil {
		panic("unexpected call to Update")
	}
	return m.update(ctx, ingredient)
}

func (m *mockIngredientRepo) Delete(ctx context.Context, id uint) error {
	panic("unexpected call")
}

func (m *mockIngredientRepo) List(ctx context.Context) ([]*domain.Ingredient, error) {
	panic("unexpected call")
}

func ptr[T any](v T) *T {
	return &v
}

func sampleIngredient(id uint) *domain.Ingredient {
	return &domain.Ingredient{
		ID:              id,
		Name:            "Джин",
		Description:     "London dry gin",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	}
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

func TestIngredientService_Update_SingleField(t *testing.T) {
	var updated *domain.Ingredient
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return sampleIngredient(id), nil
		},
		update: func(ctx context.Context, ingredient *domain.Ingredient) error {
			updated = ingredient
			return nil
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Update(context.Background(), 1, UpdateIngredientPatch{
		Name: ptr("Джин London Dry"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Джин London Dry", ingredient.Name)
	assert.Equal(t, "London dry gin", ingredient.Description)
	assert.Equal(t, "Джин London Dry", updated.Name)
}

func TestIngredientService_Update_MultipleFields(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return sampleIngredient(id), nil
		},
		update: func(ctx context.Context, ingredient *domain.Ingredient) error {
			return nil
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Update(context.Background(), 1, UpdateIngredientPatch{
		Name:            ptr("Ром"),
		Description:     ptr("Белый ром"),
		UnitMeasurement: ptr(domain.UnitGram),
		ABV:             ptr(domain.Low),
		IngredientType:  ptr(domain.Liqueur),
	})
	require.NoError(t, err)
	assert.Equal(t, "Ром", ingredient.Name)
	assert.Equal(t, "Белый ром", ingredient.Description)
	assert.Equal(t, domain.UnitGram, ingredient.UnitMeasurement)
	assert.Equal(t, domain.Low, ingredient.ABV)
	assert.Equal(t, domain.Liqueur, ingredient.IngredientType)
}

func TestIngredientService_Update_EmptyPatch(t *testing.T) {
	service := NewIngredientService(&mockIngredientRepo{})

	ingredient, err := service.Update(context.Background(), 1, UpdateIngredientPatch{})
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
	assert.Equal(t, "нет полей для обновления", err.Error())
}

func TestIngredientService_Update_InvalidEnum(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return sampleIngredient(id), nil
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Update(context.Background(), 1, UpdateIngredientPatch{
		ABV: ptr(domain.ABVEnum("неверная")),
	})
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
}

func TestIngredientService_Update_NotFound(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return nil, domain.NewErrNotFound("Ингредиент не найден")
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Update(context.Background(), 42, UpdateIngredientPatch{
		Name: ptr("Ром"),
	})
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestIngredientService_Update_DuplicateName(t *testing.T) {
	repo := &mockIngredientRepo{
		getByID: func(ctx context.Context, id uint) (*domain.Ingredient, error) {
			return sampleIngredient(id), nil
		},
		update: func(ctx context.Context, ingredient *domain.Ingredient) error {
			return domain.NewErrAlreadyExists("запись уже существует")
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Update(context.Background(), 1, UpdateIngredientPatch{
		Name: ptr("Ром"),
	})
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
}
