import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api/client";
import { listIngredients } from "./api";
import { DEFAULT_LIST_PARAMS } from "./listState";

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("ingredients api", () => {
  it("получает страницу ингредиентов", async () => {
    server.use(
      http.get("/api/v1/ingredients", ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get("pageSize")).toBe("25");
        return HttpResponse.json({
          ingredients: [
            {
              id: 1,
              name: "Джин",
              description: "",
              unit_measurement: "мл",
              abv: "крепкий",
              ingredient_type: "крепкая часть",
              has_icon: false,
              version: 1,
              created_at: "2026-07-11T12:00:00Z",
              updated_at: "2026-07-11T12:00:00Z",
            },
          ],
          nextPageToken: "",
          totalSize: 1,
        });
      }),
    );

    const page = await listIngredients(DEFAULT_LIST_PARAMS);
    expect(page.ingredients[0]).toMatchObject({ id: 1, name: "Джин" });
    expect(page.totalSize).toBe(1);
  });

  it("возвращает structured error backend", async () => {
    server.use(
      http.get("/api/v1/ingredients", () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL_ERROR",
              message: "Внутренняя ошибка сервера",
              request_id: "req-42",
            },
          },
          { status: 500 },
        ),
      ),
    );

    await expect(listIngredients(DEFAULT_LIST_PARAMS)).rejects.toEqual(
      new ApiError(
        500,
        "INTERNAL_ERROR",
        "Внутренняя ошибка сервера",
        "req-42",
      ),
    );
  });
});
