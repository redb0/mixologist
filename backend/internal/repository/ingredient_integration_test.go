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

	list, err := suite.repository.List(suite.ctx)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	// ORDER BY created_at DESC — последний созданный первым
	assert.Equal(t, second.ID, list[0].ID)
	assert.Equal(t, first.ID, list[1].ID)
	assert.Equal(t, "Ром", list[0].Name)
	assert.Equal(t, "Джин", list[1].Name)
	assert.False(t, list[0].HasIcon)
	assert.True(t, list[1].HasIcon)
}

func (suite *IngredientRepositoryTestSuite) TestList_Empty() {
	t := suite.T()

	list, err := suite.repository.List(suite.ctx)
	assert.NoError(t, err)
	assert.Empty(t, list)
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
