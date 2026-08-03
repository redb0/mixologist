package services

import (
	"unicode/utf8"

	"github.com/redb0/mixologist/internal/domain"
)

const (
	defaultPageSize = 25
	minPageSize     = 1
	maxPageSize     = 100
	maxNameFilter   = 512
)

func normalizeListParams(params domain.IngredientListParams) domain.IngredientListParams {
	if params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}
	if params.Sort == "" {
		params.Sort = domain.ByCreatedAt
	}
	if params.Order == "" {
		params.Order = domain.Desc
	}
	return params
}

func validateListParams(params domain.IngredientListParams) error {
	if params.PageSize < minPageSize || params.PageSize > maxPageSize {
		return domain.NewErrInvalidIngredientData("pageSize должен быть от 1 до 100")
	}
	switch params.Sort {
	case domain.ByName, domain.ByCreatedAt:
	default:
		return domain.NewErrInvalidIngredientData("недопустимое значение sort")
	}
	switch params.Order {
	case domain.Asc, domain.Desc:
	default:
		return domain.NewErrInvalidIngredientData("недопустимое значение order")
	}
	if params.Filters.ABV != nil && !params.Filters.ABV.IsValid() {
		return domain.NewErrInvalidIngredientData("недопустимое значение abv")
	}
	if params.Filters.IngredientType != nil && !params.Filters.IngredientType.IsValid() {
		return domain.NewErrInvalidIngredientData("недопустимое значение ingredient_type")
	}
	if params.Filters.Name != nil {
		nameLen := utf8.RuneCountInString(*params.Filters.Name)
		if nameLen == 0 || nameLen > maxNameFilter {
			return domain.NewErrInvalidIngredientData("недопустимое значение name")
		}
	}
	return nil
}
