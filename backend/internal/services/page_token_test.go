package services

import (
	"testing"
	"time"

	"encoding/base64"

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

func TestDecodePageToken_InvalidBase64(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByCreatedAt, Order: domain.Desc}
	_, err := DecodePageToken(domain.IngredientPageToken("%%%"), params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}

func TestDecodePageToken_InvalidJSON(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByCreatedAt, Order: domain.Desc}
	_, err := DecodePageToken(domain.IngredientPageToken("not-json"), params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}

func TestDecodePageToken_OrderMismatch(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByCreatedAt, Order: domain.Desc}
	item := domain.Ingredient{ID: 1, CreatedAt: time.Now().UTC()}
	token := EncodePageToken(domain.ByCreatedAt, domain.Asc, params.PageSize, item)

	_, err := DecodePageToken(token, params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}

func TestDecodePageToken_InvalidVersion(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByCreatedAt, Order: domain.Desc}
	raw := []byte(`{"v":2,"sort":"created_at","order":"desc","page_size":25,"id":1,"created_at":"2026-01-01T00:00:00Z"}`)
	token := domain.IngredientPageToken(base64.RawURLEncoding.EncodeToString(raw))

	_, err := DecodePageToken(token, params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}

func TestDecodePageToken_MissingCursorFields(t *testing.T) {
	params := domain.IngredientListParams{PageSize: 25, Sort: domain.ByName, Order: domain.Asc}
	raw := []byte(`{"v":1,"sort":"name","order":"asc","page_size":25,"id":1}`)
	token := domain.IngredientPageToken(base64.RawURLEncoding.EncodeToString(raw))

	_, err := DecodePageToken(token, params)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidPageToken)
}
