package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

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
	GetIcon(ctx context.Context, id uint) ([]byte, error)
	Update(ctx context.Context, ingredient *domain.Ingredient) error
	UpdateIcon(ctx context.Context, id uint, icon []byte) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, params domain.IngredientListParams, keyset *domain.IngredientListCursor) (domain.IngredientPage, bool, error)
}

type ingredientRepository struct {
	db *sqlx.DB
}

func NewIngredientRepository(db *sqlx.DB) IngredientRepository {
	return &ingredientRepository{db: db}
}

func (r *ingredientRepository) Create(ctx context.Context, ingredient *domain.Ingredient) (*domain.Ingredient, error) {
	query := `--sql
		INSERT INTO ingredients (name, description, unit_measurement, abv, ingredient_type, icon)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, version, created_at, updated_at
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
	).Scan(&ingredient.ID, &ingredient.Version, &ingredient.CreatedAt, &ingredient.UpdatedAt)
	if err != nil {
		return nil, ParseDBError(err)
	}
	ingredient.CreatedAt = ingredient.CreatedAt.UTC()
	ingredient.UpdatedAt = ingredient.UpdatedAt.UTC()
	return ingredient, nil
}

func (r *ingredientRepository) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	query := `--sql
		SELECT
			id,
			name,
			description,
			unit_measurement,
			abv,
			ingredient_type,
			(icon IS NOT NULL AND octet_length(icon) > 0) AS has_icon,
			version,
			created_at,
			updated_at
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
		return nil, ParseDBError(err)
	}
	ingredientDomain := toDomainIngredient(&ingredient)
	return ingredientDomain, nil
}

func (r *ingredientRepository) GetIcon(ctx context.Context, id uint) ([]byte, error) {
	var icon []byte
	err := r.db.GetContext(ctx, &icon, `SELECT icon FROM ingredients WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewErrNotFound("Ингредиент не найден")
		}
		return nil, ParseDBError(err)
	}
	if len(icon) == 0 {
		return nil, domain.NewErrNotFound("Иконка ингредиента не найдена")
	}
	return icon, nil
}

func (r *ingredientRepository) Update(ctx context.Context, ingredient *domain.Ingredient) error {
	query := `--sql
		UPDATE ingredients
		SET
			name = :name,
			description = :description,
			unit_measurement = :unit_measurement,
			abv = :abv,
			ingredient_type = :ingredient_type,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = :id AND version = :version
		RETURNING version, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, ingredient)
	if err != nil {
		return ParseDBError(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("ошибка закрытия rows: %v", err)
		}
	}()

	if !rows.Next() {
		var exists bool
		if err := r.db.GetContext(
			ctx,
			&exists,
			`SELECT EXISTS(SELECT 1 FROM ingredients WHERE id = $1)`,
			ingredient.ID,
		); err != nil {
			return ParseDBError(err)
		}
		if !exists {
			return domain.NewErrNotFound("Ингредиент не найден")
		}
		return domain.NewErrVersionConflict("конфликт версии ингредиента")
	}
	if err := rows.Scan(&ingredient.Version, &ingredient.UpdatedAt); err != nil {
		return fmt.Errorf("ошибка чтения обновлённого ингредиента: %w", err)
	}
	ingredient.UpdatedAt = ingredient.UpdatedAt.UTC()
	return rows.Err()
}

func (r *ingredientRepository) UpdateIcon(ctx context.Context, id uint, icon []byte) error {
	query := `--sql
		UPDATE ingredients
		SET icon = $1
		WHERE id = $2
	`
	result, err := r.db.ExecContext(ctx, query, icon, id)
	if err != nil {
		return ParseDBError(err)
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
	query := `--sql
		DELETE FROM ingredients
		WHERE id = :id
	`
	result, err := r.db.NamedExecContext(ctx, query, map[string]any{"id": id})
	if err != nil {
		return ParseDBError(err)
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

func (r *ingredientRepository) List(
	ctx context.Context,
	params domain.IngredientListParams,
	keyset *domain.IngredientListCursor,
) (domain.IngredientPage, bool, error) {
	query, args := buildIngredientListQuery(params, keyset)
	countQuery, countArgs := buildIngredientCountQuery(params.Filters)

	var totalSize int
	if err := r.db.GetContext(ctx, &totalSize, countQuery, countArgs...); err != nil {
		return domain.IngredientPage{}, false, ParseDBError(err)
	}

	var ingredients []*models.Ingredient
	if err := r.db.SelectContext(ctx, &ingredients, query, args...); err != nil {
		return domain.IngredientPage{}, false, ParseDBError(err)
	}

	hasMore := len(ingredients) > params.PageSize
	if hasMore {
		ingredients = ingredients[:params.PageSize]
	}

	items := make([]domain.Ingredient, len(ingredients))
	for i, ingredient := range ingredients {
		items[i] = *toDomainIngredient(ingredient)
	}

	return domain.IngredientPage{
		Items:         items,
		NextPageToken: "",
		TotalSize:     totalSize,
	}, hasMore, nil
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
		HasIcon:         ingredient.HasIcon,
		Version:         ingredient.Version,
		CreatedAt:       ingredient.CreatedAt.UTC(),
		UpdatedAt:       ingredient.UpdatedAt.UTC(),
	}
}
