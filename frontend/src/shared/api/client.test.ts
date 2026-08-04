import { describe, expect, it } from "vitest";
import { ApiError, buildQueryString } from "./client";

describe("buildQueryString", () => {
  it("пропускает пустые значения", () => {
    expect(
      buildQueryString({
        pageSize: 25,
        pageToken: "",
        name: undefined,
        sort: "created_at",
      }),
    ).toBe("?pageSize=25&sort=created_at");
  });
});

describe("ApiError parsing", () => {
  it("распознаёт structured error envelope", async () => {
    const { apiRequest } = await import("./client");
    const originalFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response(
        JSON.stringify({
          error: {
            code: "NOT_FOUND",
            message: "Ингредиент не найден",
            request_id: "req-1",
          },
        }),
        { status: 404 },
      );

    await expect(apiRequest("/ingredients/42")).rejects.toEqual(
      new ApiError(404, "NOT_FOUND", "Ингредиент не найден", "req-1"),
    );
    globalThis.fetch = originalFetch;
  });

  it("поддерживает legacy string error", async () => {
    const { apiRequest } = await import("./client");
    const originalFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ error: "внутренняя ошибка сервера" }), {
        status: 500,
      });

    await expect(apiRequest("/ingredients")).rejects.toMatchObject({
      status: 500,
      message: "внутренняя ошибка сервера",
    });
    globalThis.fetch = originalFetch;
  });

  it("возвращает undefined для 204", async () => {
    const { apiRequest } = await import("./client");
    const originalFetch = globalThis.fetch;
    globalThis.fetch = async () => new Response(null, { status: 204 });

    await expect(apiRequest<void>("/ingredients/1")).resolves.toBeUndefined();
    globalThis.fetch = originalFetch;
  });
});
