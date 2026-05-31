package domain_test

import (
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestUnitMeasurementEnum_IsValid(t *testing.T) {
	tests := []struct {
		name string
		unit domain.UnitMeasurementEnum
		want bool
	}{
		{name: "ml", unit: domain.UnitMl, want: true},
		{name: "gram", unit: domain.UnitGram, want: true},
		{name: "piece", unit: domain.UnitPiece, want: true},
		{name: "dash", unit: domain.UnitDash, want: true},
		{name: "invalid", unit: domain.UnitMeasurementEnum("invalid"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.unit.IsValid()
			assert.Equal(t, test.want, got)
		})
	}
}

func TestABVEnum_IsValid(t *testing.T) {
	tests := []struct {
		name string
		abv  domain.ABVEnum
		want bool
	}{
		{name: "free", abv: domain.Free, want: true},
		{name: "low", abv: domain.Low, want: true},
		{name: "strong", abv: domain.Strong, want: true},
		{name: "invalid", abv: domain.ABVEnum("invalid"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.abv.IsValid()
			assert.Equal(t, test.want, got)
		})
	}
}

func TestIngredientTypeEnum_IsValid(t *testing.T) {
	tests := []struct {
		name           string
		ingredientType domain.IngredientTypeEnum
		want           bool
	}{
		{name: "strong part", ingredientType: domain.StrongPart, want: true},
		{name: "free part", ingredientType: domain.FreePart, want: true},
		{name: "vermouth", ingredientType: domain.Vermouth, want: true},
		{name: "wine", ingredientType: domain.Wine, want: true},
		{name: "liqueur", ingredientType: domain.Liqueur, want: true},
		{name: "bitters", ingredientType: domain.Bitters, want: true},
		{name: "syrup", ingredientType: domain.Syrup, want: true},
		{name: "other", ingredientType: domain.Other, want: true},
		{name: "fruit", ingredientType: domain.Fruit, want: true},
		{name: "vegetable", ingredientType: domain.Vegetable, want: true},
		{name: "berry", ingredientType: domain.Berry, want: true},
		{name: "invalid", ingredientType: domain.IngredientTypeEnum("invalid"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ingredientType.IsValid()
			assert.Equal(t, test.want, got)
		})
	}
}
