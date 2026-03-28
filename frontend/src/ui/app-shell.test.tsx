import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { AppShell } from "./app-shell";
import { m } from "@/i18n/messages";

let mockProjectStatus: {
  isLoading: boolean;
  isError: boolean;
  error: null | { message: string };
  data: null | {
    name: string;
    crs: string;
    scenario_count: number;
    run_count: number;
  };
} = {
  isLoading: false,
  isError: false,
  error: null,
  data: null,
};

vi.mock("@/api", () => ({
  useProjectStatus: () => mockProjectStatus,
}));

vi.mock("@/ui/theme-toggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

vi.mock("@/ui/language-toggle", () => ({
  LanguageToggle: () => <div data-testid="language-toggle" />,
}));

describe("AppShell", () => {
  it("shows only Import in the workspace rail when no project is loaded", () => {
    mockProjectStatus = {
      isLoading: false,
      isError: false,
      error: null,
      data: null,
    };

    render(
      <MemoryRouter initialEntries={["/welcome"]}>
        <AppShell>
          <div>content</div>
        </AppShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: m.nav_import() })).toBeVisible();
    expect(screen.queryByRole("link", { name: m.nav_map() })).toBeNull();
    expect(screen.queryByRole("link", { name: m.nav_run() })).toBeNull();
    expect(screen.queryByRole("link", { name: m.nav_results() })).toBeNull();
    expect(screen.queryByRole("link", { name: m.nav_export() })).toBeNull();
  });

  it("shows the full workspace navigation when a project is available", () => {
    mockProjectStatus = {
      isLoading: false,
      isError: false,
      error: null,
      data: {
        name: "Demo Project",
        crs: "EPSG:4326",
        scenario_count: 2,
        run_count: 1,
      },
    };

    render(
      <MemoryRouter initialEntries={["/map"]}>
        <AppShell>
          <div>content</div>
        </AppShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: m.nav_import() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_map() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_run() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_results() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_export() })).toBeVisible();
  });
});
