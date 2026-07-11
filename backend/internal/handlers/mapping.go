package handlers

import "github.com/redb0/mixologist/internal/domain"

func toIngredientResponse(ingredient *domain.Ingredient) IngredientResponse {
	return IngredientResponse{
		ID:              ingredient.ID,
		Name:            ingredient.Name,
		Description:     ingredient.Description,
		UnitMeasurement: ingredient.UnitMeasurement,
		ABV:             ingredient.ABV,
		IngredientType:  ingredient.IngredientType,
		HasIcon:         ingredient.HasIcon || len(ingredient.Icon) > 0,
		CreatedAt:       ingredient.CreatedAt,
	}
}

func toIngredientsResponse(ingredients []*domain.Ingredient) []IngredientResponse {
	responses := make([]IngredientResponse, len(ingredients))
	for i, ingredient := range ingredients {
		responses[i] = toIngredientResponse(ingredient)
	}
	return responses
}
