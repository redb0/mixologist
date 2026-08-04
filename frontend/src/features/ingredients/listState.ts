import type { GridSortModel } from "@mui/x-data-grid";
import type {
  IngredientListParams,
  IngredientSortField,
  SortOrder,
} from "./types";

export const DEFAULT_LIST_PARAMS: IngredientListParams = {
  pageSize: 25,
  pageToken: "",
  sort: "created_at",
  order: "desc",
  filters: {},
};

export function listQueryKey(params: IngredientListParams) {
  return ["ingredients", "list", params] as const;
}

export function dataGridSortToApi(sortModel: GridSortModel): {
  sort: IngredientSortField;
  order: SortOrder;
} {
  if (sortModel.length === 0) {
    return { sort: "created_at", order: "desc" };
  }
  const { field, sort } = sortModel[0];
  return {
    sort: field === "name" ? "name" : "created_at",
    order: sort === "asc" ? "asc" : "desc",
  };
}

export function apiSortToDataGrid(
  sort: IngredientSortField,
  order: SortOrder,
): GridSortModel {
  return [{ field: sort === "name" ? "name" : "created_at", sort: order }];
}

export function normalizeListParams(params: IngredientListParams): IngredientListParams {
  return {
    pageSize: params.pageSize,
    pageToken: params.pageToken ?? "",
    sort: params.sort,
    order: params.order,
    filters: {
      name: params.filters.name?.trim() || undefined,
      ingredient_type: params.filters.ingredient_type,
      abv: params.filters.abv,
    },
  };
}
