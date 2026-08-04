import type { components } from "../../shared/api/generated";
import {
  ABV_OPTIONS,
  INGREDIENT_TYPES,
  UNIT_MEASUREMENTS,
} from "./constants";

export { ABV_OPTIONS, INGREDIENT_TYPES, UNIT_MEASUREMENTS };

export type UnitMeasurement = components["schemas"]["UnitMeasurement"];
export type Abv = components["schemas"]["Abv"];
export type IngredientType = components["schemas"]["IngredientType"];
export type Ingredient = components["schemas"]["Ingredient"];
export type IngredientListResponse =
  components["schemas"]["IngredientListResponse"];
export type CreateIngredientRequest =
  components["schemas"]["CreateIngredientRequest"];
export type UpdateIngredientRequest =
  components["schemas"]["UpdateIngredientRequest"];

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

export type IngredientSortField = components["schemas"]["IngredientSortField"];
export type SortOrder = components["schemas"]["SortOrder"];

export interface IngredientListFilters {
  name?: string;
  ingredient_type?: IngredientType;
  abv?: Abv;
}

export interface IngredientListParams {
  pageSize: number;
  pageToken?: string;
  sort: IngredientSortField;
  order: SortOrder;
  filters: IngredientListFilters;
}
