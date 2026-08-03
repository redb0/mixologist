package handlers

import (
	"time"

	"github.com/redb0/mixologist/internal/domain"
)

type CreateIngredientRequest struct {
	Name            string                     `json:"name" binding:"required,min=3,max=512"`
	Description     string                     `json:"description" binding:"max=1024"`
	UnitMeasurement domain.UnitMeasurementEnum `json:"unit_measurement" binding:"required"`
	ABV             domain.ABVEnum             `json:"abv" binding:"required"`
	IngredientType  domain.IngredientTypeEnum  `json:"ingredient_type" binding:"required"`
}

type UpdateIngredientRequest struct {
	Name            *string                     `json:"name" binding:"omitempty,min=3,max=512"`
	Description     *string                     `json:"description" binding:"omitempty,max=1024"`
	UnitMeasurement *domain.UnitMeasurementEnum `json:"unit_measurement"`
	ABV             *domain.ABVEnum             `json:"abv"`
	IngredientType  *domain.IngredientTypeEnum  `json:"ingredient_type"`
}

type IngredientResponse struct {
	ID              uint                       `json:"id"`
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	UnitMeasurement domain.UnitMeasurementEnum `json:"unit_measurement"`
	ABV             domain.ABVEnum             `json:"abv"`
	IngredientType  domain.IngredientTypeEnum  `json:"ingredient_type"`
	HasIcon         bool                       `json:"has_icon"`
	Version         int                        `json:"version"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
}

type IngredientListResponse struct {
	Ingredients   []IngredientResponse `json:"ingredients"`
	NextPageToken string               `json:"nextPageToken"`
	TotalSize     int                  `json:"totalSize"`
}
