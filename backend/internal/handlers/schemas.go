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

type CreateIngredientResponse struct {
	ID              uint                       `json:"id"`
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	UnitMeasurement domain.UnitMeasurementEnum `json:"unit_measurement"`
	ABV             domain.ABVEnum             `json:"abv"`
	IngredientType  domain.IngredientTypeEnum  `json:"ingredient_type"`
	CreatedAt       time.Time                  `json:"created_at"`
}
