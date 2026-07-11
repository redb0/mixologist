package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/repository"
)

const MaxIconSize = 512 * 1024 // 512 KB

type UpdateIngredientPatch struct {
	Name            *string
	Description     *string
	UnitMeasurement *domain.UnitMeasurementEnum
	ABV             *domain.ABVEnum
	IngredientType  *domain.IngredientTypeEnum
}

type IngredientService interface {
	GetByID(ctx context.Context, id uint) (*domain.Ingredient, error)
	Create(
		ctx context.Context,
		name string,
		description string,
		unitMeasurement domain.UnitMeasurementEnum,
		abv domain.ABVEnum,
		ingredientType domain.IngredientTypeEnum,
	) (*domain.Ingredient, error)
	Update(ctx context.Context, id uint, patch UpdateIngredientPatch) (*domain.Ingredient, error)
	Delete(ctx context.Context, id uint) error
	SetIcon(ctx context.Context, id uint, icon []byte) error
}

type ingredientService struct {
	repo repository.IngredientRepository
}

func (s *ingredientService) GetByID(ctx context.Context, id uint) (*domain.Ingredient, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ingredientService) Create(
	ctx context.Context,
	name string,
	description string,
	unitMeasurement domain.UnitMeasurementEnum,
	abv domain.ABVEnum,
	ingredientType domain.IngredientTypeEnum,
) (*domain.Ingredient, error) {
	ingredient := &domain.Ingredient{
		Name:            name,
		Description:     description,
		UnitMeasurement: unitMeasurement,
		ABV:             abv,
		IngredientType:  ingredientType,
	}
	if err := s.validateIngredient(ingredient); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.NewErrInvalidIngredientData("неверные данные ингредиента"), err)
	}
	return s.repo.Create(ctx, ingredient)
}

func (s *ingredientService) Update(ctx context.Context, id uint, patch UpdateIngredientPatch) (*domain.Ingredient, error) {
	if patch.Name == nil &&
		patch.Description == nil &&
		patch.UnitMeasurement == nil &&
		patch.ABV == nil &&
		patch.IngredientType == nil {
		return nil, domain.NewErrInvalidIngredientData("нет полей для обновления")
	}

	ingredient, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if patch.Name != nil {
		ingredient.Name = *patch.Name
	}
	if patch.Description != nil {
		ingredient.Description = *patch.Description
	}
	if patch.UnitMeasurement != nil {
		ingredient.UnitMeasurement = *patch.UnitMeasurement
	}
	if patch.ABV != nil {
		ingredient.ABV = *patch.ABV
	}
	if patch.IngredientType != nil {
		ingredient.IngredientType = *patch.IngredientType
	}

	if err := s.validateIngredient(ingredient); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.NewErrInvalidIngredientData("неверные данные ингредиента"), err)
	}

	if err := s.repo.Update(ctx, ingredient); err != nil {
		return nil, err
	}
	return ingredient, nil
}

func (s *ingredientService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *ingredientService) SetIcon(ctx context.Context, id uint, icon []byte) error {
	if len(icon) == 0 {
		return domain.NewErrInvalidIngredientData("иконка не может быть пустой")
	}
	if len(icon) > MaxIconSize {
		return domain.NewErrInvalidIngredientData("иконка слишком большая (макс. 512 KB)")
	}
	if !isAllowedIconFormat(icon) {
		return domain.NewErrInvalidIngredientData("иконка должна быть в формате PNG или JPEG")
	}

	return s.repo.UpdateIcon(ctx, id, icon)
}

func isAllowedIconFormat(icon []byte) bool {
	switch http.DetectContentType(icon) {
	case "image/png", "image/jpeg":
		return true
	default:
		return false
	}
}

func (s *ingredientService) validateIngredient(ingredient *domain.Ingredient) error {
	if !ingredient.UnitMeasurement.IsValid() {
		return errors.New("неверная единица измерения")
	}
	if !ingredient.ABV.IsValid() {
		return errors.New("неверная крепость")
	}
	if !ingredient.IngredientType.IsValid() {
		return errors.New("неверный тип ингредиента")
	}
	return nil
}

func NewIngredientService(repo repository.IngredientRepository) IngredientService {
	return &ingredientService{repo: repo}
}
