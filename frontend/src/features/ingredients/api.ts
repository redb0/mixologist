import {
  API_BASE_URL,
  apiRequest,
  buildQueryString,
} from "../../shared/api/client";
import type {
  CreateIngredientRequest,
  Ingredient,
  IngredientListParams,
  IngredientListResponse,
  UpdateIngredientRequest,
} from "./types";
import { normalizeListParams } from "./listState";

const jsonHeaders = { "Content-Type": "application/json" };

export const ingredientKeys = {
  all: ["ingredients"] as const,
  lists: () => [...ingredientKeys.all, "list"] as const,
  list: (params: IngredientListParams) =>
    [...ingredientKeys.lists(), normalizeListParams(params)] as const,
  detail: (id: number) => [...ingredientKeys.all, id] as const,
};

function listQuery(params: IngredientListParams): string {
  const normalized = normalizeListParams(params);
  return buildQueryString({
    pageSize: normalized.pageSize,
    pageToken: normalized.pageToken,
    sort: normalized.sort,
    order: normalized.order,
    name: normalized.filters.name,
    ingredient_type: normalized.filters.ingredient_type,
    abv: normalized.filters.abv,
  });
}

export function listIngredients(
  params: IngredientListParams,
): Promise<IngredientListResponse> {
  return apiRequest<IngredientListResponse>(`/ingredients${listQuery(params)}`);
}

export function getIngredient(id: number): Promise<Ingredient> {
  return apiRequest<Ingredient>(`/ingredients/${id}`);
}

export function createIngredient(
  payload: CreateIngredientRequest,
): Promise<Ingredient> {
  return apiRequest<Ingredient>("/ingredients", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}

export function updateIngredient(
  id: number,
  payload: UpdateIngredientRequest,
): Promise<Ingredient> {
  return apiRequest<Ingredient>(`/ingredients/${id}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
}

export function deleteIngredient(id: number): Promise<void> {
  return apiRequest<void>(`/ingredients/${id}`, { method: "DELETE" });
}

export function uploadIngredientIcon(id: number, icon: File): Promise<void> {
  return apiRequest<void>(`/ingredients/${id}/icon`, {
    method: "PUT",
    headers: { "Content-Type": "application/octet-stream" },
    body: icon,
  });
}

export function ingredientIconUrl(id: number): string {
  return `${API_BASE_URL}/ingredients/${id}/icon`;
}
