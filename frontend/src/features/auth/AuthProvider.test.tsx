import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { useState, type ReactNode } from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { AuthProvider } from "./AuthProvider.tsx";
import { useAuth } from "./auth-context.ts";
import { ApiError } from "../../shared/api/client.ts";

const server = setupServer();

const adminUser = {
  id: 7,
  email: "admin@example.com",
  display_name: "Admin",
  role: "admin" as const,
};

function Probe() {
  const auth = useAuth();
  const [logoutError, setLogoutError] = useState<string | null>(null);
  return (
    <div>
      <p>status:{auth.status}</p>
      {auth.user ? <p>email:{auth.user.email}</p> : null}
      {auth.error ? <p>error:{auth.error.code}</p> : null}
      {logoutError ? <p>logout-error:{logoutError}</p> : null}
      <button
        type="button"
        onClick={() => {
          void auth.logout().catch((error: unknown) => {
            if (error instanceof ApiError) {
              setLogoutError(error.code);
            }
          });
        }}
      >
        Выйти
      </button>
      <button type="button" onClick={() => void auth.refresh()}>
        Повторить
      </button>
    </div>
  );
}

function renderAuth(ui: ReactNode = <Probe />) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <AuthProvider>{ui}</AuthProvider>
    </QueryClientProvider>,
  );
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  server.resetHandlers();
  localStorage.clear();
  sessionStorage.clear();
  document.cookie =
    "csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/";
});
afterAll(() => server.close());

describe("AuthProvider", () => {
  it("восстанавливает пользователя через /auth/me", async () => {
    server.use(http.get("/api/v1/auth/me", () => HttpResponse.json(adminUser)));

    renderAuth();

    expect(screen.getByText("status:loading")).toBeInTheDocument();
    expect(await screen.findByText("status:authenticated")).toBeInTheDocument();
    expect(screen.getByText("email:admin@example.com")).toBeInTheDocument();
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("считает пользователя anonymous при 401", async () => {
    server.use(
      http.get("/api/v1/auth/me", () =>
        HttpResponse.json(
          {
            error: {
              code: "UNAUTHORIZED",
              message: "требуется аутентификация",
            },
          },
          { status: 401 },
        ),
      ),
    );

    renderAuth();

    expect(await screen.findByText("status:anonymous")).toBeInTheDocument();
  });

  it("показывает error, если restore не удался", async () => {
    server.use(
      http.get("/api/v1/auth/me", () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL_ERROR",
              message: "Внутренняя ошибка сервера",
            },
          },
          { status: 500 },
        ),
      ),
    );

    renderAuth();

    expect(await screen.findByText("status:error")).toBeInTheDocument();
    expect(screen.getByText("error:INTERNAL_ERROR")).toBeInTheDocument();
  });

  it("logout вызывает backend и очищает состояние", async () => {
    document.cookie = "csrf_token=csrf-from-cookie";
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(adminUser)),
      http.post(
        "/api/v1/auth/logout",
        () => new HttpResponse(null, { status: 204 }),
      ),
    );
    const user = userEvent.setup();
    renderAuth();
    expect(await screen.findByText("status:authenticated")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Выйти" }));

    expect(await screen.findByText("status:anonymous")).toBeInTheDocument();
    expect(
      screen.queryByText("email:admin@example.com"),
    ).not.toBeInTheDocument();
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("не сбрасывает сессию, если logout вернул 403", async () => {
    document.cookie = "csrf_token=csrf-from-cookie";
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(adminUser)),
      http.post("/api/v1/auth/logout", () =>
        HttpResponse.json(
          {
            error: {
              code: "CSRF_TOKEN_INVALID",
              message: "Некорректный CSRF token",
            },
          },
          { status: 403 },
        ),
      ),
    );
    const user = userEvent.setup();
    renderAuth();
    expect(await screen.findByText("status:authenticated")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Выйти" }));

    expect(
      await screen.findByText("logout-error:CSRF_TOKEN_INVALID"),
    ).toBeInTheDocument();
    expect(screen.getByText("status:authenticated")).toBeInTheDocument();
    expect(screen.getByText("email:admin@example.com")).toBeInTheDocument();
  });
});
