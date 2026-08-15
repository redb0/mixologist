import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { AuthErrorPage } from "./AuthErrorPage";

describe("AuthErrorPage", () => {
  it("показывает message из query, а не подмену UNAUTHORIZED", () => {
    render(
      <MemoryRouter
        initialEntries={[
          "/auth/error?code=UNAUTHORIZED&message=%D0%BD%D0%B5%D0%B2%D0%B0%D0%BB%D0%B8%D0%B4%D0%BD%D1%8B%D0%B9%20identity%20token",
        ]}
      >
        <Routes>
          <Route path="/auth/error" element={<AuthErrorPage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("невалидный identity token")).toBeInTheDocument();
    expect(
      screen.queryByText("Требуется вход в систему."),
    ).not.toBeInTheDocument();
  });
});
