import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { AppRoutes } from "./App";
import { AuthProvider } from "./features/auth/AuthProvider";

const server = setupServer();

const adminUser = {
  id: 1,
  email: "admin@example.com",
  display_name: "Админ",
  role: "admin" as const,
};

const regularUser = {
  id: 2,
  email: "user@example.com",
  display_name: "Иван",
  role: "user" as const,
};

function renderApp(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
});
afterEach(() => {
  server.resetHandlers();
  localStorage.clear();
  sessionStorage.clear();
  document.cookie =
    "csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/";
});
afterAll(() => server.close());

describe("auth routes", () => {
  it("показывает login anonymous-пользователю", async () => {
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

    renderApp("/ingredients");

    const login = await screen.findByRole("link", {
      name: "Войти через Google",
    });
    expect(login).toHaveAttribute(
      "href",
      "/api/v1/auth/google/login?return_to=%2Fauth%2Fcallback",
    );
  });

  it("не пускает user в admin routes", async () => {
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(regularUser)),
    );

    renderApp("/ingredients");

    expect(
      await screen.findByRole("heading", { name: "Добро пожаловать, Иван" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Ингредиенты" }),
    ).not.toBeInTheDocument();
  });

  it("показывает admin UI ингредиентов администратору", async () => {
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(adminUser)),
      http.get("/api/v1/ingredients", () =>
        HttpResponse.json({
          ingredients: [],
          nextPageToken: "",
          totalSize: 0,
        }),
      ),
    );

    renderApp("/ingredients");

    expect(
      await screen.findByRole("heading", { name: "Ингредиенты" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Добавить ингредиент" }),
    ).toBeInTheDocument();
  });

  it("logout вызывает backend и возвращает на login", async () => {
    document.cookie = "csrf_token=csrf-from-cookie";
    let logoutCalled = false;
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(regularUser)),
      http.post("/api/v1/auth/logout", () => {
        logoutCalled = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    renderApp("/");

    expect(
      await screen.findByRole("heading", { name: "Добро пожаловать, Иван" }),
    ).toBeInTheDocument();

    await user.click(screen.getAllByRole("link", { name: "Выйти" })[0]);

    expect(
      await screen.findByRole("link", { name: "Войти через Google" }),
    ).toBeInTheDocument();
    expect(logoutCalled).toBe(true);
  });
});
