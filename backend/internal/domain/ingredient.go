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

type IngredientSort string

const (
	ByName      IngredientSort = "name"
	ByCreatedAt IngredientSort = "created_at"
)

type SortOrder string

const (
	Asc  SortOrder = "asc"
	Desc SortOrder = "desc"
)

type IngredientPageToken string

type IngredientPage struct {
	Items         []Ingredient
	NextPageToken IngredientPageToken
	TotalSize     int
}

type IngredientFilters struct {
	Name           *string
	ABV            *ABVEnum
	IngredientType *IngredientTypeEnum
}

type IngredientListParams struct {
	PageSize  int
	PageToken IngredientPageToken
	Sort      IngredientSort
	Order     SortOrder
	Filters   IngredientFilters
}

type IngredientListCursor struct {
	ID        uint
	Name      string
	CreatedAt time.Time
}

type Ingredient struct {
	ID              uint                `db:"id"`
	Name            string              `db:"name"`
	Description     string              `db:"description"`
	UnitMeasurement UnitMeasurementEnum `db:"unit_measurement"`
	ABV             ABVEnum             `db:"abv"`
	IngredientType  IngredientTypeEnum  `db:"ingredient_type"`
	Icon            []byte              `db:"icon"`
	HasIcon         bool                `db:"has_icon"`
	Version         int                 `db:"version"`
	CreatedAt       time.Time           `db:"created_at"`
	UpdatedAt       time.Time           `db:"updated_at"`
}
