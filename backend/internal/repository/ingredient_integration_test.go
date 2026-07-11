package repository

import (
	"context"
	"errors"
	"testing"

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
	assert.Equal(t, createdIngredient.Icon, ingredient.Icon)
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
	assert.Equal(t, []byte{1, 2, 3}, updated.Icon)
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
