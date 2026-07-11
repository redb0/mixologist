export const UNIT_MEASUREMENTS = ["мл", "гр", "шт", "дэш"] as const;
export const ABV_OPTIONS = [
  "безалкогольный",
  "слабоалкогольный",
  "крепкий",
] as const;
export const INGREDIENT_TYPES = [
  "крепкая часть",
  "безалкогольная часть",
  "вермут",
  "вино",
  "ликер",
  "биттер",
  "сироп",
  "другое",
  "фрукт",
  "овощ",
  "ягода",
] as const;

export type UnitMeasurement = (typeof UNIT_MEASUREMENTS)[number];
export type Abv = (typeof ABV_OPTIONS)[number];
export type IngredientType = (typeof INGREDIENT_TYPES)[number];

export interface Ingredient {
  id: number;
  name: string;
  description: string;
  unit_measurement: UnitMeasurement;
  abv: Abv;
  ingredient_type: IngredientType;
  has_icon: boolean;
  created_at: string;
}

export interface IngredientFormValues {
  name: string;
  description: string;
  unit_measurement: UnitMeasurement;
  abv: Abv;
  ingredient_type: IngredientType;
}

export const EMPTY_INGREDIENT_VALUES: IngredientFormValues = {
  name: "",
  description: "",
  unit_measurement: "мл",
  abv: "безалкогольный",
  ingredient_type: "другое",
};

export type CreateIngredientRequest = IngredientFormValues;
export type UpdateIngredientRequest = Partial<IngredientFormValues>;
