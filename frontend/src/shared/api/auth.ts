import type { components } from "./generated";
import { API_BASE_URL, apiRequest, buildQueryString } from "./client";

export type CurrentUser = components["schemas"]["CurrentUser"];

export const authKeys = {
  all: ["auth"] as const,
  me: () => [...authKeys.all, "me"] as const,
};

export function getCurrentUser(): Promise<CurrentUser> {
  return apiRequest<CurrentUser>("/auth/me");
}

export function logout(): Promise<void> {
  return apiRequest<void>("/auth/logout", { method: "POST" });
}

export function googleLoginUrl(returnTo = "/"): string {
  return `${API_BASE_URL}/auth/google/login${buildQueryString({
    return_to: returnTo,
  })}`;
}
