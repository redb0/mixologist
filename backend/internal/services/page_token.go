package services

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/redb0/mixologist/internal/domain"
)

const pageTokenSchemaVersion = 1

type pageTokenPayload struct {
	V         int       `json:"v"`
	Sort      string    `json:"sort"`
	Order     string    `json:"order"`
	PageSize  int       `json:"page_size"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	ID        uint      `json:"id"`
}

func EncodePageToken(
	sort domain.IngredientSort,
	order domain.SortOrder,
	pageSize int,
	item domain.Ingredient,
) domain.IngredientPageToken {
	payload := pageTokenPayload{
		V:        pageTokenSchemaVersion,
		Sort:     string(sort),
		Order:    string(order),
		PageSize: pageSize,
		ID:       item.ID,
	}
	switch sort {
	case domain.ByName:
		payload.Name = strings.ToLower(item.Name)
	default:
		payload.CreatedAt = item.CreatedAt.UTC()
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return domain.IngredientPageToken(base64.RawURLEncoding.EncodeToString(raw))
}

func DecodePageToken(
	token domain.IngredientPageToken,
	params domain.IngredientListParams,
) (*domain.IngredientListCursor, error) {
	if token == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(string(token))
	if err != nil {
		return nil, domain.NewErrInvalidPageToken("Некорректный или несовместимый pageToken")
	}

	var payload pageTokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, domain.NewErrInvalidPageToken("Некорректный или несовместимый pageToken")
	}
	if err := validatePageTokenPayload(payload, params); err != nil {
		return nil, err
	}

	return &domain.IngredientListCursor{
		ID:        payload.ID,
		Name:      payload.Name,
		CreatedAt: payload.CreatedAt.UTC(),
	}, nil
}

func validatePageTokenPayload(payload pageTokenPayload, params domain.IngredientListParams) error {
	err := domain.NewErrInvalidPageToken("Некорректный или несовместимый pageToken")
	if payload.V != pageTokenSchemaVersion {
		return err
	}
	if domain.IngredientSort(payload.Sort) != params.Sort {
		return err
	}
	if domain.SortOrder(payload.Order) != params.Order {
		return err
	}
	if payload.PageSize != params.PageSize {
		return err
	}
	if payload.ID == 0 {
		return err
	}
	switch params.Sort {
	case domain.ByName:
		if payload.Name == "" {
			return err
		}
	case domain.ByCreatedAt:
		if payload.CreatedAt.IsZero() {
			return err
		}
	}
	return nil
}
