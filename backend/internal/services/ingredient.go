package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/repository"
)

type IngredientService interface {
	Create(
		ctx context.Context,
		name string,
		description string,
		unitMeasurement domain.UnitMeasurementEnum,
		abv domain.ABVEnum,
		ingredientType domain.IngredientTypeEnum,
	) (*domain.Ingredient, error)
	SetIcon(ctx context.Context, id uint, icon []byte) error
}

type ingredientService struct {
	repo repository.IngredientRepository
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

func (s *ingredientService) SetIcon(ctx context.Context, id uint, icon []byte) error {
	ingredient, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	ingredient.Icon = icon
	return s.repo.Update(ctx, ingredient)
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
