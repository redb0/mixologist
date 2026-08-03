package repository

import (
	"fmt"
	"strings"

	"github.com/redb0/mixologist/internal/domain"
)

func escapeILIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func buildFiltersClause(filters domain.IngredientFilters, startArg int) (string, []any) {
	var parts []string
	var args []any
	arg := startArg

	if filters.Name != nil && len(*filters.Name) > 0 {
		parts = append(parts, fmt.Sprintf(`name ILIKE $%d ESCAPE '\'`, arg))
		args = append(args, "%"+escapeILIKE(*filters.Name)+"%")
		arg += 1
	}
	if filters.ABV != nil {
		parts = append(parts, fmt.Sprintf("abv = $%d", arg))
		args = append(args, *filters.ABV)
		arg += 1
	}
	if filters.IngredientType != nil {
		parts = append(parts, fmt.Sprintf("ingredient_type = $%d", arg))
		args = append(args, *filters.IngredientType)
		arg += 1
	}
	return strings.Join(parts, " AND "), args
}

func buildOrderByClause(sort domain.IngredientSort, order domain.SortOrder) string {
	direction := "DESC"
	if order == domain.Asc {
		direction = "ASC"
	}
	query := "ORDER BY created_at DESC, id DESC"
	switch sort {
	case domain.ByName:
		query = fmt.Sprintf("ORDER BY LOWER(name) %s, id %s", direction, direction)
	case domain.ByCreatedAt:
		query = fmt.Sprintf("ORDER BY created_at %s, id %s", direction, direction)
	}
	return query
}

func buildPaginationClause(
	sort domain.IngredientSort,
	order domain.SortOrder,
	pageSize int,
	keyset *domain.IngredientListCursor,
	startArg int,
) (keysetClause string, limitClause string, args []any) {
	arg := startArg
	args = make([]any, 0, 3)

	if keyset != nil {
		cmp := "<"
		if order == domain.Asc {
			cmp = ">"
		}

		switch sort {
		case domain.ByName:
			keysetClause = fmt.Sprintf("(LOWER(name), id) %s ($%d, $%d)", cmp, arg, arg+1)
			args = append(args, keyset.Name, keyset.ID)
		default:
			keysetClause = fmt.Sprintf("(created_at, id) %s ($%d, $%d)", cmp, arg, arg+1)
			args = append(args, keyset.CreatedAt, keyset.ID)
		}
		arg += 2
	}

	limitClause = fmt.Sprintf("LIMIT $%d", arg)
	args = append(args, pageSize+1)
	return keysetClause, limitClause, args
}

func buildIngredientListQuery(params domain.IngredientListParams, keyset *domain.IngredientListCursor) (string, []any) {
	filterClause, filterArgs := buildFiltersClause(params.Filters, 1)
	keysetClause, limitClause, paginationArgs := buildPaginationClause(
		params.Sort,
		params.Order,
		params.PageSize,
		keyset,
		len(filterArgs)+1,
	)

	var whereParts []string
	if filterClause != "" {
		whereParts = append(whereParts, filterClause)
	}
	if keysetClause != "" {
		whereParts = append(whereParts, keysetClause)
	}

	query := strings.TrimSpace(`--sql
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
		FROM ingredients`)

	if len(whereParts) > 0 {
		query += "\nWHERE " + strings.Join(whereParts, " AND ")
	}

	query += "\n" + buildOrderByClause(params.Sort, params.Order)
	query += "\n" + limitClause

	args := make([]any, 0, len(filterArgs)+len(paginationArgs))
	args = append(args, filterArgs...)
	args = append(args, paginationArgs...)

	return query, args
}

func buildIngredientCountQuery(filters domain.IngredientFilters) (string, []any) {
	filterClause, args := buildFiltersClause(filters, 1)
	query := "SELECT COUNT(*) FROM ingredients"
	if filterClause != "" {
		query += " WHERE " + filterClause
	}
	return query, args
}
