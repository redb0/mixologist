package handlers

import "github.com/redb0/mixologist/internal/domain"

func toIngredientResponse(ingredient domain.Ingredient) IngredientResponse {
	return IngredientResponse{
		ID:              ingredient.ID,
		Name:            ingredient.Name,
		Description:     ingredient.Description,
		UnitMeasurement: ingredient.UnitMeasurement,
		ABV:             ingredient.ABV,
		IngredientType:  ingredient.IngredientType,
		HasIcon:         ingredient.HasIcon,
		Version:         ingredient.Version,
		CreatedAt:       ingredient.CreatedAt,
		UpdatedAt:       ingredient.UpdatedAt,
	}
}

func toIngredientResponsePtr(ingredient *domain.Ingredient) IngredientResponse {
	return toIngredientResponse(*ingredient)
}

func toIngredientListResponse(page domain.IngredientPage) IngredientListResponse {
	ingredients := make([]IngredientResponse, len(page.Items))
	for i, ingredient := range page.Items {
		ingredients[i] = toIngredientResponse(ingredient)
	}
	return IngredientListResponse{
		Ingredients:   ingredients,
		NextPageToken: string(page.NextPageToken),
		TotalSize:     page.TotalSize,
	}
}
