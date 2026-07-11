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
	getByID    func(ctx context.Context, id uint) (*domain.Ingredient, error)
	create     func(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error)
	update     func(ctx context.Context, ingredient *domain.Ingredient) error
	updateIcon func(ctx context.Context, id uint, icon []byte) error
	delete     func(ctx context.Context, id uint) error
	list       func(ctx context.Context) ([]*domain.Ingredient, error)
}

func (m *mockIngredientRepo) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	if m.getByID == nil {
		panic("unexpected call to GetByID")
	}
	return m.getByID(ctx, id)
}

func (m *mockIngredientRepo) Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
	if m.create == nil {
		panic("unexpected call to Create")
	}
	return m.create(ctx, ingredient)
}

func (m *mockIngredientRepo) Update(ctx context.Context, ingredient *domain.Ingredient) error {
	if m.update == nil {
		panic("unexpected call to Update")
	}
	return m.update(ctx, ingredient)
}

func (m *mockIngredientRepo) UpdateIcon(ctx context.Context, id uint, icon []byte) error {
	if m.updateIcon == nil {
		panic("unexpected call to UpdateIcon")
	}
	return m.updateIcon(ctx, id, icon)
}

func (m *mockIngredientRepo) Delete(ctx context.Context, id uint) error {
	if m.delete == nil {
		panic("unexpected call to Delete")
	}
	return m.delete(ctx, id)
}

func (m *mockIngredientRepo) List(ctx context.Context) ([]*domain.Ingredient, error) {
	if m.list == nil {
		panic("unexpected call to List")
	}
	return m.list(ctx)
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

func TestIngredientService_Create(t *testing.T) {
	var created *domain.Ingredient
	repo := &mockIngredientRepo{
		create: func(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
			created = ingredient
			ingredient.ID = 1
			return ingredient, nil
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Create(
		context.Background(),
		"Джин",
		"London dry gin",
		domain.UnitMl,
		domain.Strong,
		domain.StrongPart,
	)
	require.NoError(t, err)
	assert.Equal(t, uint(1), ingredient.ID)
	assert.Equal(t, "Джин", ingredient.Name)
	assert.Equal(t, "London dry gin", ingredient.Description)
	assert.Equal(t, domain.UnitMl, ingredient.UnitMeasurement)
	assert.Equal(t, domain.Strong, ingredient.ABV)
	assert.Equal(t, domain.StrongPart, ingredient.IngredientType)
	assert.Equal(t, "Джин", created.Name)
	assert.Equal(t, "London dry gin", created.Description)
	assert.Equal(t, domain.UnitMl, created.UnitMeasurement)
	assert.Equal(t, domain.Strong, created.ABV)
	assert.Equal(t, domain.StrongPart, created.IngredientType)
}

func TestIngredientService_Create_InvalidEnum(t *testing.T) {
	service := NewIngredientService(&mockIngredientRepo{})

	cases := []struct {
		name            string
		unitMeasurement domain.UnitMeasurementEnum
		abv             domain.ABVEnum
		ingredientType  domain.IngredientTypeEnum
	}{
		{
			name:            "invalid unit",
			unitMeasurement: domain.UnitMeasurementEnum("invalid"),
			abv:             domain.Strong,
			ingredientType:  domain.StrongPart,
		},
		{
			name:            "invalid abv",
			unitMeasurement: domain.UnitMl,
			abv:             domain.ABVEnum("invalid"),
			ingredientType:  domain.StrongPart,
		},
		{
			name:            "invalid type",
			unitMeasurement: domain.UnitMl,
			abv:             domain.Strong,
			ingredientType:  domain.IngredientTypeEnum("invalid"),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ingredient, err := service.Create(
				context.Background(),
				"Джин",
				"описание",
				tt.unitMeasurement,
				tt.abv,
				tt.ingredientType,
			)
			assert.Nil(t, ingredient)
			assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
		})
	}
}

func TestIngredientService_Create_RepoError(t *testing.T) {
	repo := &mockIngredientRepo{
		create: func(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
			return nil, domain.NewErrAlreadyExists("запись уже существует")
		},
	}
	service := NewIngredientService(repo)

	ingredient, err := service.Create(
		context.Background(),
		"Джин",
		"London dry gin",
		domain.UnitMl,
		domain.Strong,
		domain.StrongPart,
	)
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
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

func TestIngredientService_Delete(t *testing.T) {
	var deletedID uint
	repo := &mockIngredientRepo{
		delete: func(ctx context.Context, id uint) error {
			deletedID = id
			return nil
		},
	}
	service := NewIngredientService(repo)

	err := service.Delete(context.Background(), 7)
	assert.NoError(t, err)
	assert.Equal(t, uint(7), deletedID)
}

func TestIngredientService_Delete_NotFound(t *testing.T) {
	repo := &mockIngredientRepo{
		delete: func(ctx context.Context, id uint) error {
			return domain.NewErrNotFound("Ингредиент не найден")
		},
	}
	service := NewIngredientService(repo)

	err := service.Delete(context.Background(), 42)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestIngredientService_SetIcon(t *testing.T) {
	cases := []struct {
		name string
		icon []byte
	}{
		{name: "png", icon: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
		{name: "jpeg", icon: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var gotID uint
			var gotIcon []byte
			repo := &mockIngredientRepo{
				updateIcon: func(ctx context.Context, id uint, icon []byte) error {
					gotID = id
					gotIcon = icon
					return nil
				},
			}
			service := NewIngredientService(repo)

			err := service.SetIcon(context.Background(), 1, tt.icon)
			require.NoError(t, err)
			assert.Equal(t, uint(1), gotID)
			assert.Equal(t, tt.icon, gotIcon)
		})
	}
}

func TestIngredientService_SetIcon_Empty(t *testing.T) {
	service := NewIngredientService(&mockIngredientRepo{})

	err := service.SetIcon(context.Background(), 1, nil)
	assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
	assert.Equal(t, "иконка не может быть пустой", err.Error())
}

func TestIngredientService_SetIcon_TooLarge(t *testing.T) {
	service := NewIngredientService(&mockIngredientRepo{})

	err := service.SetIcon(context.Background(), 1, make([]byte, MaxIconSize+1))
	assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
	assert.Equal(t, "иконка слишком большая (макс. 512 KB)", err.Error())
}

func TestIngredientService_SetIcon_InvalidFormat(t *testing.T) {
	service := NewIngredientService(&mockIngredientRepo{})

	cases := []struct {
		name string
		icon []byte
	}{
		{name: "gif", icon: []byte("GIF89a")},
		{name: "raw", icon: []byte{1, 2, 3}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SetIcon(context.Background(), 1, tt.icon)
			assert.True(t, errors.Is(err, domain.ErrInvalidIngredientData))
			assert.Equal(t, "иконка должна быть в формате PNG или JPEG", err.Error())
		})
	}
}

func TestIngredientService_SetIcon_NotFound(t *testing.T) {
	repo := &mockIngredientRepo{
		updateIcon: func(ctx context.Context, id uint, icon []byte) error {
			return domain.NewErrNotFound("Ингредиент не найден")
		},
	}
	service := NewIngredientService(repo)

	err := service.SetIcon(context.Background(), 42, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestIngredientService_List(t *testing.T) {
	expected := []*domain.Ingredient{
		sampleIngredient(2),
		sampleIngredient(1),
	}
	repo := &mockIngredientRepo{
		list: func(ctx context.Context) ([]*domain.Ingredient, error) {
			return expected, nil
		},
	}
	service := NewIngredientService(repo)

	list, err := service.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expected, list)
	assert.Len(t, list, 2)
	assert.Equal(t, uint(2), list[0].ID)
	assert.Equal(t, uint(1), list[1].ID)
}

func TestIngredientService_List_Empty(t *testing.T) {
	repo := &mockIngredientRepo{
		list: func(ctx context.Context) ([]*domain.Ingredient, error) {
			return []*domain.Ingredient{}, nil
		},
	}
	service := NewIngredientService(repo)

	list, err := service.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.NotNil(t, list)
}

func TestIngredientService_List_RepoError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &mockIngredientRepo{
		list: func(ctx context.Context) ([]*domain.Ingredient, error) {
			return nil, repoErr
		},
	}
	service := NewIngredientService(repo)

	list, err := service.List(context.Background())
	assert.Nil(t, list)
	assert.ErrorIs(t, err, repoErr)
}
