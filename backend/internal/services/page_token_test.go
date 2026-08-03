package services

import (
	"testing"
	"time"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodePageToken_CreatedAt(t *testing.T) {
	params := domain.IngredientListParams{
		PageSize: 25,
		Sort:     domain.ByCreatedAt,
		Order:    domain.Desc,
	}
	item := domain.Ingredient{
		ID:        42,
		Name:      "Джин",
		CreatedAt: time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
	}

	token := EncodePageToken(params.Sort, params.Order, params.PageSize, item)
	require.NotEmpty(t, token)

	keyset, err := DecodePageToken(token, params)
	require.NoError(t, err)
	require.NotNil(t, keyset)
	assert.Equal(t, item.ID, keyset.ID)
	assert.Equal(t, item.CreatedAt, keyset.CreatedAt)
}

func TestEncodeDecodePageToken_Name(t *testing.T) {
	params := domain.IngredientListParams{
		PageSize: 25,
		Sort:     domain.ByName,
		Order:    domain.Asc,
	}
	item := domain.Ingredient{
		ID:   7,
		Name: "Джин",
	}

	token := EncodePageToken(params.Sort, params.Order, params.PageSize, item)
	keyset, err := DecodePageToken(token, params)
	require.NoError(t, err)
	require.NotNil(t, keyset)
	assert.Equal(t, "джин", keyset.Name)
}

func TestDecodePageToken_Empty(t *testing.T) {
	keyset, err := DecodePageToken("", domain.IngredientListParams{})
	require.NoError(t, err)
	assert.Nil(t, keyset)
}

func TestDecodePageToken_PageSizeMismatch(t *testing.T) {
	params := domain.IngredientListParams{
		PageSize: 25,
		Sort:     domain.ByCreatedAt,
		Order:    domain.Desc,
	}
	item := domain.Ingredient{ID: 1, Name: "A", CreatedAt: time.Now().UTC()}
	token := EncodePageToken(params.Sort, params.Order, params.PageSize, item)

	mismatchedParams := params
	mismatchedParams.PageSize = 50

	_, err := DecodePageToken(token, mismatchedParams)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}

func TestDecodePageToken_SortMismatch(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByCreatedAt, Order: domain.Desc}
	item := domain.Ingredient{ID: 1, Name: "A", CreatedAt: time.Now().UTC()}
	token := EncodePageToken(domain.ByName, domain.Asc, params.PageSize, item)

	_, err := DecodePageToken(token, params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}
