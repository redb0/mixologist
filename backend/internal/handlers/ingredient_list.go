package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
)

func parseIngredientListParams(ctx *gin.Context) (domain.IngredientListParams, error) {
	params := domain.IngredientListParams{
		PageToken: domain.IngredientPageToken(ctx.Query("pageToken")),
		Sort:      domain.IngredientSort(ctx.Query("sort")),
		Order:     domain.SortOrder(ctx.Query("order")),
	}

	if pageSizeRaw := ctx.Query("pageSize"); pageSizeRaw != "" {
		pageSize, err := strconv.Atoi(pageSizeRaw)
		if err != nil {
			return domain.IngredientListParams{}, domain.NewErrInvalidIngredientData("недопустимое значение pageSize")
		}
		if pageSize <= 0 {
			return domain.IngredientListParams{}, domain.NewErrInvalidIngredientData("pageSize должен быть от 1 до 100")
		}
		params.PageSize = pageSize
	}

	if name := ctx.Query("name"); name != "" {
		params.Filters.Name = &name
	}
	if abvRaw := ctx.Query("abv"); abvRaw != "" {
		abv := domain.ABVEnum(abvRaw)
		params.Filters.ABV = &abv
	}
	if typeRaw := ctx.Query("ingredient_type"); typeRaw != "" {
		ingredientType := domain.IngredientTypeEnum(typeRaw)
		params.Filters.IngredientType = &ingredientType
	}

	return params, nil
}
