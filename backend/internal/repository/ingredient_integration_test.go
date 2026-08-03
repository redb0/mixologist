package repository

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IngredientRepositoryTestSuite struct {
	suite.Suite
	pgContainer *testutil.PostgresContainer
	repository  IngredientRepository
	ctx         context.Context
}

func (suite *IngredientRepositoryTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := testutil.SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer
	suite.repository = NewIngredientRepository(suite.pgContainer.DB)
}

func (suite *IngredientRepositoryTestSuite) TearDownTest() {
	err := testutil.TruncateIngredients(suite.pgContainer.DB)
	if err != nil {
		suite.T().Fatalf("Не удалось очистить таблицу ingredients: %v", err)
	}
}

func (suite *IngredientRepositoryTestSuite) TearDownSuite() {
	if suite.pgContainer == nil {
		return
	}
	if suite.pgContainer.DB != nil {
		_ = suite.pgContainer.DB.Close()
	}
	if err := suite.pgContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatalf("Не удалось завершить контейнер postgres: %s", err)
	}
}

func (suite *IngredientRepositoryTestSuite) TestCreate() {
	t := suite.T()

	ingredient, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Тестовый ингредиент",
			Description:     "Описание ингредиента",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.StrongPart,
			Icon:            []byte{1, 2, 3},
		},
	)
	assert.NoError(t, err)
	assert.NotZero(t, ingredient.ID)
	assert.NotZero(t, ingredient.CreatedAt)

	var realIngredient domain.Ingredient
	err = suite.pgContainer.DB.Get(
		&realIngredient,
		`SELECT id, name, description, unit_measurement, abv, ingredient_type, icon, created_at
		FROM ingredients
		WHERE id = $1`,
		ingredient.ID,
	)
	assert.NoError(t, err)

	assert.Equal(t, ingredient.Name, realIngredient.Name)
	assert.Equal(t, ingredient.Description, realIngredient.Description)
	assert.Equal(t, ingredient.UnitMeasurement, realIngredient.UnitMeasurement)
	assert.Equal(t, ingredient.ABV, realIngredient.ABV)
	assert.Equal(t, ingredient.IngredientType, realIngredient.IngredientType)
	assert.Equal(t, ingredient.Icon, realIngredient.Icon)
}

func (suite *IngredientRepositoryTestSuite) TestCreate_DuplicateName() {
	t := suite.T()

	_, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "London dry gin",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	cases := []struct {
		testName string
		name     string
	}{
		{testName: "same name", name: "Джин"},
		{testName: "lower case name", name: "джин"},
	}

	for i, tt := range cases {
		suite.Run(tt.testName, func() {
			ingredient, err := suite.repository.Create(
				suite.ctx,
				&domain.Ingredient{
					Name:            tt.name,
					Description:     "Джин №" + strconv.Itoa(i),
					UnitMeasurement: domain.UnitMl,
					ABV:             domain.Strong,
					IngredientType:  domain.StrongPart,
				},
			)
			assert.Nil(t, ingredient)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
		})
	}
}

func (suite *IngredientRepositoryTestSuite) TestCreate_MinimalFields() {
	t := suite.T()

	ingredient, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Тоник",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.FreePart,
		},
	)
	assert.NoError(t, err)
	assert.NotZero(t, ingredient.ID)
	assert.NotZero(t, ingredient.CreatedAt)

	got, err := suite.repository.GetByID(suite.ctx, ingredient.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Тоник", got.Name)
	assert.Empty(t, got.Description)
	assert.False(t, got.HasIcon)
	assert.Equal(t, domain.UnitMl, got.UnitMeasurement)
	assert.Equal(t, domain.Free, got.ABV)
	assert.Equal(t, domain.FreePart, got.IngredientType)
	assert.True(t, ingredient.CreatedAt.Location() == time.UTC)
}

func (suite *IngredientRepositoryTestSuite) TestList() {
	t := suite.T()

	first, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "Первый",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
			Icon:            []byte{0x89, 0x50, 0x4E, 0x47},
		},
	)
	assert.NoError(t, err)

	second, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Ром",
			Description:     "Второй",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	list, _, err := suite.repository.List(suite.ctx, defaultListParams(), nil)
	assert.NoError(t, err)
	assert.Len(t, list.Items, 2)
	// ORDER BY created_at DESC — последний созданный первым
	assert.Equal(t, second.ID, list.Items[0].ID)
	assert.Equal(t, first.ID, list.Items[1].ID)
	assert.Equal(t, "Ром", list.Items[0].Name)
	assert.Equal(t, "Джин", list.Items[1].Name)
	assert.False(t, list.Items[0].HasIcon)
	assert.True(t, list.Items[1].HasIcon)
	assert.Equal(t, 2, list.TotalSize)
}

func (suite *IngredientRepositoryTestSuite) TestList_Empty() {
	t := suite.T()

	list, _, err := suite.repository.List(suite.ctx, defaultListParams(), nil)
	assert.NoError(t, err)
	assert.Empty(t, list.Items)
	assert.Equal(t, 0, list.TotalSize)
}

func (suite *IngredientRepositoryTestSuite) TestList_WithFilters() {
	t := suite.T()

	_, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Апельсиновый сок",
			Description:     "Свежий",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.FreePart,
		},
	)
	assert.NoError(t, err)

	_, err = suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Тоник",
			Description:     "Газированный",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.FreePart,
		},
	)
	assert.NoError(t, err)

	_, err = suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "Крепкий",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	list, hasMore, err := suite.repository.List(
		suite.ctx,
		domain.IngredientListParams{
			PageSize: 25,
			Sort:     domain.ByCreatedAt,
			Order:    domain.Desc,
			Filters: domain.IngredientFilters{
				Name:           ptrString("сок"),
				ABV:            ptrABV(domain.Free),
				IngredientType: ptrIngredientType(domain.FreePart),
			},
		},
		nil,
	)
	assert.NoError(t, err)
	assert.False(t, hasMore)
	assert.Equal(t, 1, list.TotalSize)
	assert.Len(t, list.Items, 1)
	assert.Equal(t, "Апельсиновый сок", list.Items[0].Name)
}

func (suite *IngredientRepositoryTestSuite) TestList_KeysetPagination() {
	t := suite.T()

	first, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Апероль",
			Description:     "Первый",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Low,
			IngredientType:  domain.Liqueur,
		},
	)
	assert.NoError(t, err)

	second, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Бурбон",
			Description:     "Второй",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	third, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Вермут",
			Description:     "Третий",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Low,
			IngredientType:  domain.Vermouth,
		},
	)
	assert.NoError(t, err)

	params := domain.IngredientListParams{
		PageSize: 2,
		Sort:     domain.ByCreatedAt,
		Order:    domain.Desc,
	}

	page1, hasMore, err := suite.repository.List(suite.ctx, params, nil)
	assert.NoError(t, err)
	assert.True(t, hasMore)
	assert.Len(t, page1.Items, 2)
	assert.Equal(t, third.ID, page1.Items[0].ID)
	assert.Equal(t, second.ID, page1.Items[1].ID)
	assert.Equal(t, 3, page1.TotalSize)

	cursor := &domain.IngredientListCursor{
		ID:        page1.Items[len(page1.Items)-1].ID,
		CreatedAt: page1.Items[len(page1.Items)-1].CreatedAt,
	}

	page2, hasMore, err := suite.repository.List(suite.ctx, params, cursor)
	assert.NoError(t, err)
	assert.False(t, hasMore)
	assert.Len(t, page2.Items, 1)
	assert.Equal(t, first.ID, page2.Items[0].ID)
	assert.Equal(t, 3, page2.TotalSize)
}

func (suite *IngredientRepositoryTestSuite) TestGetByID_NotFound() {
	t := suite.T()

	ingredient, err := suite.repository.GetByID(suite.ctx, 42)
	assert.Nil(t, ingredient)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestGetByID() {
	t := suite.T()

	createdIngredient, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Тестовый ингредиент",
			Description:     "Описание ингредиента",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)
	assert.NotZero(t, createdIngredient.ID)
	assert.NotZero(t, createdIngredient.CreatedAt)

	ingredient, err := suite.repository.GetByID(suite.ctx, createdIngredient.ID)
	assert.NoError(t, err)
	assert.Equal(t, createdIngredient.Name, ingredient.Name)
	assert.Equal(t, createdIngredient.Description, ingredient.Description)
	assert.Equal(t, createdIngredient.UnitMeasurement, ingredient.UnitMeasurement)
	assert.Equal(t, createdIngredient.ABV, ingredient.ABV)
	assert.Equal(t, createdIngredient.IngredientType, ingredient.IngredientType)
	assert.False(t, ingredient.HasIcon)
	assert.Equal(t, createdIngredient.CreatedAt, ingredient.CreatedAt)
}

func (suite *IngredientRepositoryTestSuite) TestUpdate() {
	t := suite.T()

	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "Старое описание",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
			Icon:            []byte{1, 2, 3},
		},
	)
	assert.NoError(t, err)

	created.Name = "Джин London Dry"
	created.Description = "Новое описание"
	created.ABV = domain.Low
	initialVersion := created.Version
	initialUpdatedAt := created.UpdatedAt
	err = suite.repository.Update(suite.ctx, created)
	assert.NoError(t, err)

	updated, err := suite.repository.GetByID(suite.ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Джин London Dry", updated.Name)
	assert.Equal(t, "Новое описание", updated.Description)
	assert.Equal(t, domain.UnitMl, updated.UnitMeasurement)
	assert.Equal(t, domain.Low, updated.ABV)
	assert.Equal(t, domain.StrongPart, updated.IngredientType)
	assert.True(t, updated.HasIcon)
	assert.Equal(t, initialVersion+1, updated.Version)
	assert.True(t, updated.UpdatedAt.After(initialUpdatedAt) || updated.UpdatedAt.Equal(initialUpdatedAt))
}

func (suite *IngredientRepositoryTestSuite) TestUpdate_NotFound() {
	t := suite.T()

	err := suite.repository.Update(suite.ctx, &domain.Ingredient{
		ID:              42,
		Name:            "Несуществующий",
		Description:     "Описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Free,
		IngredientType:  domain.Other,
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestUpdate_DuplicateName() {
	t := suite.T()

	_, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Ром",
			Description:     "Белый ром",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	vodka, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Водка",
			Description:     "Чистая водка",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	vodka.Name = "Ром"
	err = suite.repository.Update(suite.ctx, vodka)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
}

func (suite *IngredientRepositoryTestSuite) TestUpdate_VersionConflict() {
	t := suite.T()

	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Текила",
			Description:     "Blanco",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	stale := *created

	created.Description = "Reposado"
	err = suite.repository.Update(suite.ctx, created)
	assert.NoError(t, err)

	stale.Description = "Anejo"
	err = suite.repository.Update(suite.ctx, &stale)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrVersionConflict))
}

func (suite *IngredientRepositoryTestSuite) TestUpdateIcon() {
	t := suite.T()

	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "London dry gin",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)
	assert.Empty(t, created.Icon)

	icon := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	err = suite.repository.UpdateIcon(suite.ctx, created.ID, icon)
	assert.NoError(t, err)

	updated, err := suite.repository.GetIcon(suite.ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, icon, updated)
}

func (suite *IngredientRepositoryTestSuite) TestUpdateIcon_NotFound() {
	t := suite.T()

	err := suite.repository.UpdateIcon(suite.ctx, 42, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestGetIcon() {
	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "London dry gin",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	suite.Require().NoError(err)

	icon := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	suite.Require().NoError(suite.repository.UpdateIcon(suite.ctx, created.ID, icon))

	got, err := suite.repository.GetIcon(suite.ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal(icon, got)
}

func (suite *IngredientRepositoryTestSuite) TestGetIcon_NotFound() {
	icon, err := suite.repository.GetIcon(suite.ctx, 42)
	suite.Nil(icon)
	suite.True(errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestGetIcon_Empty() {
	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Тоник",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.FreePart,
		},
	)
	suite.Require().NoError(err)

	icon, err := suite.repository.GetIcon(suite.ctx, created.ID)
	suite.Nil(icon)
	suite.True(errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestDelete() {
	t := suite.T()

	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Джин",
			Description:     "London dry gin",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	err = suite.repository.Delete(suite.ctx, created.ID)
	assert.NoError(t, err)

	ingredient, err := suite.repository.GetByID(suite.ctx, created.ID)
	assert.Nil(t, ingredient)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestDelete_NotFound() {
	t := suite.T()

	err := suite.repository.Delete(suite.ctx, 42)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientRepositoryTestSuite) TestDelete_Twice() {
	t := suite.T()

	created, err := suite.repository.Create(
		suite.ctx,
		&domain.Ingredient{
			Name:            "Ром",
			Description:     "Белый ром",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
		},
	)
	assert.NoError(t, err)

	err = suite.repository.Delete(suite.ctx, created.ID)
	assert.NoError(t, err)

	err = suite.repository.Delete(suite.ctx, created.ID)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestIngredientRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(IngredientRepositoryTestSuite))
}
