import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { ApiError } from "./client.ts";
import { getCurrentUser, googleLoginUrl, logout } from "./auth.ts";

const server = setupServer();

const currentUser = {
  id: 7,
  email: "admin@example.com",
  display_name: "Admin",
  avatar_url: "https://example.com/avatar.png",
  role: "admin" as const,
};

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => {
  server.resetHandlers();
  document.cookie =
    "csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/";
});
afterAll(() => server.close());

describe("auth api", () => {
  it("восстанавливает текущего пользователя по cookie-сессии", async () => {
    server.use(
      http.get("/api/v1/auth/me", () => HttpResponse.json(currentUser)),
    );

    await expect(getCurrentUser()).resolves.toEqual(currentUser);
  });

  it("пробрасывает 401 без сессии", async () => {
    server.use(
      http.get("/api/v1/auth/me", () =>
        HttpResponse.json(
          {
            error: {
              code: "UNAUTHORIZED",
              message: "требуется аутентификация",
              request_id: "req-auth",
            },
          },
          { status: 401 },
        ),
      ),
    );

    await expect(getCurrentUser()).rejects.toEqual(
      new ApiError(401, "UNAUTHORIZED", "требуется аутентификация", "req-auth"),
    );
  });

  it("вызывает logout с CSRF-заголовком из cookie", async () => {
    document.cookie = "csrf_token=csrf-from-cookie";
    let csrfHeader: string | null = null;

    server.use(
      http.post("/api/v1/auth/logout", ({ request }) => {
        csrfHeader = request.headers.get("X-CSRF-Token");
        return new HttpResponse(null, { status: 204 });
      }),
    );

    await expect(logout()).resolves.toBeUndefined();
    expect(csrfHeader).toBe("csrf-from-cookie");
  });

  it("собирает URL входа через Google с return_to", () => {
    expect(googleLoginUrl("/auth/callback")).toBe(
      "/api/v1/auth/google/login?return_to=%2Fauth%2Fcallback",
    );
  });
});
