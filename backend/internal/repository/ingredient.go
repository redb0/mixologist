package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/models"
)

var (
	ErrIngredientAlreadyExists = errors.New("ингредиент уже существует")
)

type IngredientRepository interface {
	Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error)
	GetByID(ctx context.Context, id uint) (*domain.Ingredient, error)
	Update(ctx context.Context, ingredient *domain.Ingredient) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*domain.Ingredient, error)
}

type ingredientRepository struct {
	db *sqlx.DB
}

func NewIngredientRepository(db *sqlx.DB) IngredientRepository {
	return &ingredientRepository{db: db}
}

func (r *ingredientRepository) Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
	query := `
		INSERT INTO ingredients (name, description, unit_measurement, abv, ingredient_type, icon)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	err := r.db.QueryRowxContext(
		ctx,
		query,
		ingredient.Name,
		ingredient.Description,
		ingredient.UnitMeasurement,
		ingredient.ABV,
		ingredient.IngredientType,
		ingredient.Icon,
	).Scan(&ingredient.ID, &ingredient.CreatedAt)
	if err != nil {
		return nil, ParseDBError(err)
	}
	return ingredient, nil
}

func (r *ingredientRepository) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	query := `
		SELECT
			id,
			name,
			description,
			unit_measurement,
			abv,
			ingredient_type,
			icon,
			created_at
		FROM ingredients
		WHERE id = $1
	`
	var ingredient models.Ingredient
	err := r.db.GetContext(
		ctx,
		&ingredient,
		query,
		id,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewErrNotFound("Ингредиент не найден")
		}
		return nil, fmt.Errorf("ошибка получения ингредиента по ID %d: %w", id, err)
	}
	ingredientDomain := toDomainIngredient(&ingredient)
	return ingredientDomain, nil
}

func (r *ingredientRepository) Update(ctx context.Context, ingredient *domain.Ingredient) error {
	query := `
		UPDATE ingredients
		SET
			name = :name,
			description = :description,
			unit_measurement = :unit_measurement,
			abv = :abv,
			ingredient_type = :ingredient_type,
			icon = :icon
		WHERE id = :id
	`
	result, err := r.db.NamedExecContext(ctx, query, ingredient)
	if err != nil {
		return fmt.Errorf("ошибка обновления ингредиента по ID %d: %w", ingredient.ID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества обновленных строк: %w", err)
	}
	if rowsAffected == 0 {
		return domain.NewErrNotFound("Ингредиент не найден")
	}
	return nil
}

func (r *ingredientRepository) Delete(ctx context.Context, id uint) error {
	query := `
		DELETE FROM ingredients
		WHERE id = :id
	`
	result, err := r.db.NamedExecContext(ctx, query, map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("ошибка удаления ингредиента по ID %d: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества удаленных строк: %w", err)
	}
	if rowsAffected == 0 {
		return domain.NewErrNotFound("Ингредиент не найден")
	}
	return nil
}

func (r *ingredientRepository) List(ctx context.Context) ([]*domain.Ingredient, error) {
	query := `
		SELECT
			id,
			name,
			description,
			unit_measurement,
			abv,
			ingredient_type,
			icon,
			created_at
		FROM ingredients
		ORDER BY created_at DESC
	`
	var ingredients []*models.Ingredient
	err := r.db.SelectContext(ctx, &ingredients, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка ингредиентов: %w", err)
	}
	ingredientsDomain := make([]*domain.Ingredient, len(ingredients))
	for i, ingredient := range ingredients {
		ingredientsDomain[i] = toDomainIngredient(ingredient)
	}
	return ingredientsDomain, nil
}

func toDomainIngredient(ingredient *models.Ingredient) *domain.Ingredient {
	return &domain.Ingredient{
		ID:              ingredient.ID,
		Name:            ingredient.Name,
		Description:     ingredient.Description,
		UnitMeasurement: domain.UnitMeasurementEnum(ingredient.UnitMeasurement),
		ABV:             domain.ABVEnum(ingredient.ABV),
		IngredientType:  domain.IngredientTypeEnum(ingredient.IngredientType),
		Icon:            ingredient.Icon,
		CreatedAt:       ingredient.CreatedAt,
	}
}
