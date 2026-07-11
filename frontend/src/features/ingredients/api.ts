import { API_BASE_URL, apiRequest } from "../../shared/api/client";
import type {
  CreateIngredientRequest,
  Ingredient,
  UpdateIngredientRequest,
} from "./types";

const jsonHeaders = { "Content-Type": "application/json" };

export const ingredientKeys = {
  all: ["ingredients"] as const,
  detail: (id: number) => ["ingredients", id] as const,
};

export function listIngredients(): Promise<Ingredient[]> {
  return apiRequest<Ingredient[]>("/ingredients");
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
