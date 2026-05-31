package repository

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	*postgres.PostgresContainer
	DB *sqlx.DB
}

func SetupContainerAndMigrations(ctx context.Context) (*PostgresContainer, error) {
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("mixologist_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		return nil, err
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}

	testDB, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Println("ошибка при соединении с базой данных", err)
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}

	if err = applyMigrations(testDB); err != nil {
		_ = testDB.Close()
		testDB = nil
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}
	return &PostgresContainer{
		PostgresContainer: pgContainer,
		DB:                testDB,
	}, nil
}

func applyMigrations(db *sqlx.DB) error {
	migrationsDir := filepath.Join("..", "..", "migrations")
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, migrationFile := range files {
		sqlBytes, readErr := os.ReadFile(migrationFile)
		if readErr != nil {
			return readErr
		}
		if _, execErr := db.Exec(string(sqlBytes)); execErr != nil {
			return execErr
		}
	}
	return nil
}

type IngredientRepositoryTestSuite struct {
	suite.Suite
	pgContainer *PostgresContainer
	repository  IngredientRepository
	ctx         context.Context
}

func (suite *IngredientRepositoryTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer
	suite.repository = NewIngredientRepository(suite.pgContainer.DB)
}

func (suite *IngredientRepositoryTestSuite) TearDownTest() {
	_, err := suite.pgContainer.DB.Exec(`TRUNCATE TABLE ingredients RESTART IDENTITY CASCADE`)
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

func TestIngredientRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(IngredientRepositoryTestSuite))
}
