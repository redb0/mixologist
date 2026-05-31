package models

import "time"

type Ingredient struct {
	ID              uint      `db:"id"`
	Name            string    `db:"name"`
	Description     string    `db:"description"`
	UnitMeasurement string    `db:"unit_measurement"`
	ABV             string    `db:"abv"`
	IngredientType  string    `db:"ingredient_type"`
	Icon            []byte    `db:"icon"`
	CreatedAt       time.Time `db:"created_at"`
}
