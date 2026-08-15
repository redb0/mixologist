import { createContext, useContext } from "react";
import type { CurrentUser } from "../../shared/api/auth";
import type { ApiError } from "../../shared/api/client";

export type AuthStatus = "loading" | "authenticated" | "anonymous" | "error";

export type AuthState = {
  status: AuthStatus;
  user: CurrentUser | null;
  error: ApiError | null;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
};

export const AuthContext = createContext<AuthState | undefined>(undefined);

export function useAuth(): AuthState {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return value;
}
