package repository

import "github.com/redb0/mixologist/internal/domain"

func defaultListParams() domain.IngredientListParams {
	return domain.IngredientListParams{
		PageSize: 25,
		Sort:     domain.ByCreatedAt,
		Order:    domain.Desc,
	}
}

func ptrString(v string) *string {
	return &v
}

func ptrABV(v domain.ABVEnum) *domain.ABVEnum {
	return &v
}

func ptrIngredientType(v domain.IngredientTypeEnum) *domain.IngredientTypeEnum {
	return &v
}
