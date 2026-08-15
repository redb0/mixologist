package services

import (
	"strings"
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeListParams(t *testing.T) {
	params := normalizeListParams(domain.IngredientListParams{})
	assert.Equal(t, defaultPageSize, params.PageSize)
	assert.Equal(t, domain.ByCreatedAt, params.Sort)
	assert.Equal(t, domain.Desc, params.Order)

	custom := normalizeListParams(domain.IngredientListParams{
		PageSize: 10,
		Sort:     domain.ByName,
		Order:    domain.Asc,
	})
	assert.Equal(t, 10, custom.PageSize)
	assert.Equal(t, domain.ByName, custom.Sort)
	assert.Equal(t, domain.Asc, custom.Order)
}

func TestValidateListParams(t *testing.T) {
	validABV := domain.Strong
	validType := domain.StrongPart
	validName := "джин"

	tests := []struct {
		name    string
		params  domain.IngredientListParams
		wantErr bool
	}{
		{
			name: "valid defaults",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
			},
		},
		{
			name: "valid with filters",
			params: domain.IngredientListParams{
				PageSize: 50,
				Sort:     domain.ByName,
				Order:    domain.Asc,
				Filters: domain.IngredientFilters{
					Name:           &validName,
					ABV:            &validABV,
					IngredientType: &validType,
				},
			},
		},
		{
			name: "page size too small",
			params: domain.IngredientListParams{
				PageSize: 0,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
			},
			wantErr: true,
		},
		{
			name: "page size too large",
			params: domain.IngredientListParams{
				PageSize: 101,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
			},
			wantErr: true,
		},
		{
			name: "invalid sort",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.IngredientSort("bad"),
				Order:    domain.Desc,
			},
			wantErr: true,
		},
		{
			name: "invalid order",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.SortOrder("bad"),
			},
			wantErr: true,
		},
		{
			name: "invalid abv filter",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
				Filters: domain.IngredientFilters{
					ABV: ptr(domain.ABVEnum("bad")),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid ingredient type filter",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
				Filters: domain.IngredientFilters{
					IngredientType: ptr(domain.IngredientTypeEnum("bad")),
				},
			},
			wantErr: true,
		},
		{
			name: "empty name filter",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
				Filters: domain.IngredientFilters{
					Name: ptr(""),
				},
			},
			wantErr: true,
		},
		{
			name: "name filter too long",
			params: domain.IngredientListParams{
				PageSize: 25,
				Sort:     domain.ByCreatedAt,
				Order:    domain.Desc,
				Filters: domain.IngredientFilters{
					Name: ptr(strings.Repeat("a", maxNameFilter+1)),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListParams(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrInvalidIngredientData)
				return
			}
			require.NoError(t, err)
		})
	}
}
