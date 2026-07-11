import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api/client";
import { listIngredients } from "./api";

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("ingredients api", () => {
  it("получает список ингредиентов", async () => {
    server.use(
      http.get("/api/ingredients", () =>
        HttpResponse.json([
          {
            id: 1,
            name: "Джин",
            description: "",
            unit_measurement: "мл",
            abv: "крепкий",
            ingredient_type: "крепкая часть",
            has_icon: false,
            created_at: "2026-07-11T12:00:00Z",
          },
        ]),
      ),
    );

    await expect(listIngredients()).resolves.toEqual([
      expect.objectContaining({ id: 1, name: "Джин" }),
    ]);
  });

  it("возвращает сообщение backend при ошибке", async () => {
    server.use(
      http.get("/api/ingredients", () =>
        HttpResponse.json(
          { error: "внутренняя ошибка сервера" },
          { status: 500 },
        ),
      ),
    );

    await expect(listIngredients()).rejects.toEqual(
      new ApiError(500, "внутренняя ошибка сервера"),
    );
  });
});
