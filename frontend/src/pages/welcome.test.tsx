import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import WelcomePage from "./welcome";
import { m } from "@/i18n/messages";

vi.mock("@/api", () => ({
  useProjectStatus: () => ({
    isLoading: false,
    isError: false,
    error: null,
    data: null,
  }),
}));

describe("WelcomePage", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("renders a friendly overview and entry points", () => {
    render(
      <MemoryRouter>
        <WelcomePage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("heading", { name: m.heading_welcome() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: m.action_start_import() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: m.action_open_workspace() }),
    ).toBeInTheDocument();
    expect(screen.getByText(m.msg_welcome_intro())).toBeInTheDocument();
  });
});
