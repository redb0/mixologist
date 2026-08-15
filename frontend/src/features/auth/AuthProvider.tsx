import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo, type ReactNode } from "react";
import {
  authKeys,
  getCurrentUser,
  logout as logoutRequest,
  type CurrentUser,
} from "../../shared/api/auth";
import { ApiError } from "../../shared/api/client";
import { AuthContext, type AuthState } from "./auth-context";

async function fetchCurrentUser(): Promise<CurrentUser | null> {
  try {
    return await getCurrentUser();
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return null;
    }
    throw error;
  }
}

function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error;
  }
  const message =
    error instanceof Error ? error.message : "Не удалось восстановить сессию";
  return new ApiError(500, "UNKNOWN", message);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: authKeys.me(),
    queryFn: fetchCurrentUser,
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });

  const refresh = useCallback(async () => {
    await queryClient.refetchQueries({ queryKey: authKeys.me() });
  }, [queryClient]);

  const logout = useCallback(async () => {
    try {
      await logoutRequest();
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 401)) {
        throw error;
      }
    }
    queryClient.setQueryData(authKeys.me(), null);
  }, [queryClient]);

  const value = useMemo<AuthState>(() => {
    if (query.isPending) {
      return {
        status: "loading",
        user: null,
        error: null,
        refresh,
        logout,
      };
    }
    if (query.isError) {
      return {
        status: "error",
        user: null,
        error: toApiError(query.error),
        refresh,
        logout,
      };
    }
    if (query.data) {
      return {
        status: "authenticated",
        user: query.data,
        error: null,
        refresh,
        logout,
      };
    }
    return {
      status: "anonymous",
      user: null,
      error: null,
      refresh,
      logout,
    };
  }, [
    logout,
    query.data,
    query.error,
    query.isError,
    query.isPending,
    refresh,
  ]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
