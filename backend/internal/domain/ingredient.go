package domain

import "time"

type UnitMeasurementEnum string

const (
	UnitMl    UnitMeasurementEnum = "мл"
	UnitGram  UnitMeasurementEnum = "гр"
	UnitPiece UnitMeasurementEnum = "шт"
	UnitDash  UnitMeasurementEnum = "дэш"
)

func (u UnitMeasurementEnum) IsValid() bool {
	switch u {
	case UnitMl, UnitGram, UnitPiece, UnitDash:
		return true
	default:
		return false
	}
}

type ABVEnum string

const (
	Free   ABVEnum = "безалкогольный"
	Low    ABVEnum = "слабоалкогольный"
	Strong ABVEnum = "крепкий"
)

func (a ABVEnum) IsValid() bool {
	switch a {
	case Free, Low, Strong:
		return true
	default:
		return false
	}
}

type IngredientTypeEnum string

const (
	StrongPart IngredientTypeEnum = "крепкая часть"
	FreePart   IngredientTypeEnum = "безалкогольная часть"
	Vermouth   IngredientTypeEnum = "вермут"
	Wine       IngredientTypeEnum = "вино"
	Liqueur    IngredientTypeEnum = "ликер"
	Bitters    IngredientTypeEnum = "биттер"
	Syrup      IngredientTypeEnum = "сироп"
	Other      IngredientTypeEnum = "другое"
	Fruit      IngredientTypeEnum = "фрукт"
	Vegetable  IngredientTypeEnum = "овощ"
	Berry      IngredientTypeEnum = "ягода"
)

func (i IngredientTypeEnum) IsValid() bool {
	switch i {
	case StrongPart, FreePart, Vermouth, Wine, Liqueur, Bitters, Syrup, Other, Fruit, Vegetable, Berry:
		return true
	default:
		return false
	}
}

type Ingredient struct {
	ID              uint                `db:"id"`
	Name            string              `db:"name"`
	Description     string              `db:"description"`
	UnitMeasurement UnitMeasurementEnum `db:"unit_measurement"`
	ABV             ABVEnum             `db:"abv"`
	IngredientType  IngredientTypeEnum  `db:"ingredient_type"`
	Icon            []byte              `db:"icon"`
	CreatedAt       time.Time           `db:"created_at"`
}
