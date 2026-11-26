package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lib/pq"
	"github.com/redb0/mixologist/internal/testutil"
	"github.com/stretchr/testify/suite"
)

const missingIngredientID = 1_000_000

type RecipeSchemaTestSuite struct {
	suite.Suite
	pgContainer *testutil.PostgresContainer
	ctx         context.Context
}

type recipeSchemaFixture struct {
	authorID    int
	recipeID    int
	usedID      int
	availableID int
}

func (suite *RecipeSchemaTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := testutil.SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer
}

func (suite *RecipeSchemaTestSuite) TearDownTest() {
	if err := testutil.TruncateIngredients(suite.pgContainer.DB); err != nil {
		suite.T().Fatalf("Не удалось очистить таблицу ingredients: %v", err)
	}
	if err := testutil.TruncateAuthTables(suite.pgContainer.DB); err != nil {
		suite.T().Fatalf("Не удалось очистить auth таблицы: %v", err)
	}
}

func (suite *RecipeSchemaTestSuite) TearDownSuite() {
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

func (suite *RecipeSchemaTestSuite) queryID(query string, args ...any) int {
	var id int
	suite.Require().NoError(suite.pgContainer.DB.QueryRowContext(suite.ctx, query, args...).Scan(&id))
	return id
}

func (suite *RecipeSchemaTestSuite) exec(query string, args ...any) error {
	_, err := suite.pgContainer.DB.ExecContext(suite.ctx, query, args...)
	return err
}

func (suite *RecipeSchemaTestSuite) insertIngredient(name string) int {
	return suite.queryID(`
		INSERT INTO ingredients (name, unit_measurement, abv, ingredient_type)
		VALUES ($1, 'мл', 'крепкий', 'крепкая часть')
		RETURNING id
	`, name)
}

// setupRecipe создаёт черновик с одним ингредиентом и одним шагом на позиции 0.
func (suite *RecipeSchemaTestSuite) setupRecipe() recipeSchemaFixture {
	authorID := suite.queryID(`
		INSERT INTO users (google_subject, email, display_name, role)
		VALUES ('recipe-author', 'author@example.com', 'Author', 'admin')
		RETURNING id
	`)
	f := recipeSchemaFixture{
		authorID:    authorID,
		recipeID:    suite.queryID(`INSERT INTO recipes (author_id, title) VALUES ($1, 'Негрони') RETURNING id`, authorID),
		usedID:      suite.insertIngredient("Джин"),
		availableID: suite.insertIngredient("Вермут"),
	}
	suite.Require().NoError(suite.exec(`
		INSERT INTO recipe_ingredients (recipe_id, ingredient_id, position, amount, unit)
		VALUES ($1, $2, 0, 30, 'мл')
	`, f.recipeID, f.usedID))
	suite.Require().NoError(suite.exec(`
		INSERT INTO recipe_instructions (recipe_id, position, markdown) VALUES ($1, 0, 'Смешать')
	`, f.recipeID))
	return f
}

func (suite *RecipeSchemaTestSuite) TestNewRecipeIsDraftWithoutPublishedAt() {
	f := suite.setupRecipe()

	var status string
	var publishedAtIsNull bool
	err := suite.pgContainer.DB.QueryRowContext(suite.ctx, `
		SELECT status, published_at IS NULL FROM recipes WHERE id = $1
	`, f.recipeID).Scan(&status, &publishedAtIsNull)

	suite.Require().NoError(err)
	suite.Equal("draft", status)
	suite.True(publishedAtIsNull)
}

func (suite *RecipeSchemaTestSuite) TestConstraintViolations() {
	f := suite.setupRecipe()

	const insertRecipeIngredient = `
		INSERT INTO recipe_ingredients (recipe_id, ingredient_id, position, amount, unit)
		VALUES ($1, $2, $3, $4, 'мл')
	`
	const insertInstruction = `INSERT INTO recipe_instructions (recipe_id, position, markdown) VALUES ($1, $2, $3)`

	tests := []struct {
		name       string
		query      string
		args       []any
		errName    string
		constraint string
	}{
		{
			name:       "published без published_at",
			query:      `INSERT INTO recipes (author_id, title, status) VALUES ($1, 'Т', 'published')`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_published_at_check",
		},
		{
			name:       "draft с published_at",
			query:      `INSERT INTO recipes (author_id, title, status, published_at) VALUES ($1, 'Т', 'draft', now())`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_published_at_check",
		},
		{
			name:       "название из пробельных символов",
			query:      `INSERT INTO recipes (author_id, title) VALUES ($1, E' \t ')`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_title_length_check",
		},
		{
			name:       "название длиннее 200 символов",
			query:      `INSERT INTO recipes (author_id, title) VALUES ($1, repeat('а', 201))`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_title_length_check",
		},
		{
			name:       "описание длиннее 5000 символов",
			query:      `INSERT INTO recipes (author_id, title, description) VALUES ($1, 'Т', repeat('а', 5001))`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_description_length_check",
		},
		{
			name:       "нулевые порции",
			query:      `INSERT INTO recipes (author_id, title, servings) VALUES ($1, 'Т', 0)`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_servings_check",
		},
		{
			name:       "отрицательное время приготовления",
			query:      `INSERT INTO recipes (author_id, title, prep_time_minutes) VALUES ($1, 'Т', -1)`,
			args:       []any{f.authorID},
			errName:    "check_violation",
			constraint: "recipes_prep_time_minutes_check",
		},
		{
			name:       "повтор ингредиента в рецепте",
			query:      insertRecipeIngredient,
			args:       []any{f.recipeID, f.usedID, 1, 30},
			errName:    "unique_violation",
			constraint: "recipe_ingredients_ingredient_unique",
		},
		{
			name:       "занятая позиция ингредиента",
			query:      insertRecipeIngredient,
			args:       []any{f.recipeID, f.availableID, 0, 30},
			errName:    "unique_violation",
			constraint: "recipe_ingredients_pkey",
		},
		{
			name:       "несуществующий ингредиент",
			query:      insertRecipeIngredient,
			args:       []any{f.recipeID, missingIngredientID, 1, 30},
			errName:    "foreign_key_violation",
			constraint: "recipe_ingredients_ingredient_id_fkey",
		},
		{
			name:       "нулевое количество",
			query:      insertRecipeIngredient,
			args:       []any{f.recipeID, f.availableID, 1, 0},
			errName:    "check_violation",
			constraint: "recipe_ingredients_amount_check",
		},
		{
			name:       "пустой шаг",
			query:      insertInstruction,
			args:       []any{f.recipeID, 1, "  "},
			errName:    "check_violation",
			constraint: "recipe_instructions_markdown_length_check",
		},
		{
			name:       "шаг длиннее 10000 символов",
			query:      `INSERT INTO recipe_instructions (recipe_id, position, markdown) VALUES ($1, 1, repeat('а', 10001))`,
			args:       []any{f.recipeID},
			errName:    "check_violation",
			constraint: "recipe_instructions_markdown_length_check",
		},
		{
			name:       "занятая позиция шага",
			query:      insertInstruction,
			args:       []any{f.recipeID, 0, "Процедить"},
			errName:    "unique_violation",
			constraint: "recipe_instructions_pkey",
		},
		{
			name:       "удаление ингредиента, используемого в рецепте",
			query:      `DELETE FROM ingredients WHERE id = $1`,
			args:       []any{f.usedID},
			errName:    "foreign_key_violation",
			constraint: "recipe_ingredients_ingredient_id_fkey",
		},
		{
			name:       "удаление автора рецепта",
			query:      `DELETE FROM users WHERE id = $1`,
			args:       []any{f.authorID},
			errName:    "foreign_key_violation",
			constraint: "recipes_author_id_fkey",
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			err := suite.exec(tt.query, tt.args...)

			var pqErr *pq.Error
			suite.Require().True(errors.As(err, &pqErr), "ожидалась ошибка PostgreSQL, получено: %v", err)
			suite.Equal(tt.errName, pqErr.Code.Name())
			suite.Equal(tt.constraint, pqErr.Constraint)
		})
	}
}

func (suite *RecipeSchemaTestSuite) TestDeleteRecipeCascadesChildren() {
	f := suite.setupRecipe()

	err := suite.exec(`DELETE FROM recipes WHERE id = $1`, f.recipeID)

	suite.Require().NoError(err)
	children := suite.queryID(`
		SELECT (SELECT count(*) FROM recipe_ingredients) + (SELECT count(*) FROM recipe_instructions)
	`)
	suite.Zero(children)
}

func (suite *RecipeSchemaTestSuite) TestMigrationDownAndUpAgain() {
	migrationsDir := filepath.Join("..", "..", "migrations")
	down, err := os.ReadFile(filepath.Join(migrationsDir, "000001_create_ingredient_table.down.sql"))
	suite.Require().NoError(err)
	up, err := os.ReadFile(filepath.Join(migrationsDir, "000001_create_ingredient_table.up.sql"))
	suite.Require().NoError(err)

	downErr := suite.exec(string(down))
	leftovers := suite.queryID(`
		SELECT (SELECT count(*) FROM pg_tables WHERE schemaname = 'public')
			+ (SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
				WHERE n.nspname = 'public' AND t.typtype = 'e')
	`)
	upErr := suite.exec(string(up))

	suite.Require().NoError(downErr)
	suite.Zero(leftovers)
	suite.Require().NoError(upErr)
}

func TestRecipeSchemaTestSuite(t *testing.T) {
	suite.Run(t, new(RecipeSchemaTestSuite))
}
