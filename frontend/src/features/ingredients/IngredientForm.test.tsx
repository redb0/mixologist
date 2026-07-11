import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { IngredientForm } from "./IngredientForm";
import { EMPTY_INGREDIENT_VALUES } from "./types";

describe("IngredientForm", () => {
  it("не отправляет форму с коротким названием", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(
      <IngredientForm
        defaultValues={{ ...EMPTY_INGREDIENT_VALUES, name: "Р" }}
        isSubmitting={false}
        submitLabel="Создать"
        onSubmit={onSubmit}
        onCancel={() => undefined}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Создать" }));

    expect(await screen.findByText("Минимум 3 символа")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
