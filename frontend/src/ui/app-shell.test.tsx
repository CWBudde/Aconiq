import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
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

vi.mock("@/api/hooks", () => ({
  useProjectStatus: () => mockProjectStatus,
}));

vi.mock("@/ui/theme-toggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

vi.mock("@/ui/language-toggle", () => ({
  LanguageToggle: () => <div data-testid="language-toggle" />,
}));

vi.mock("@/ui/save-status", () => ({
  SaveStatus: () => <div data-testid="save-status" />,
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

describe("AppShell landmarks", () => {
  function renderShell(path = "/map", children?: React.ReactNode) {
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
    return render(
      <MemoryRouter initialEntries={[path]}>
        <AppShell>{children ?? <div>content</div>}</AppShell>
      </MemoryRouter>,
    );
  }

  it("renders exactly one main landmark, and the page inside it", () => {
    renderShell();
    const main = screen.getAllByRole("main");
    expect(main).toHaveLength(1);
    expect(within(main[0] as HTMLElement).getByText("content")).toBeVisible();
  });

  it("puts the whole rail inside one labelled navigation landmark", () => {
    renderShell();
    const nav = screen.getByRole("navigation", { name: m.nav_primary_label() });
    // Logo, workspace links and footer links all sit inside it.
    expect(within(nav).getByText("AconiQ")).toBeVisible();
    expect(within(nav).getByRole("link", { name: m.nav_map() })).toBeVisible();
    expect(
      within(nav).getByRole("link", { name: m.nav_settings() }),
    ).toBeVisible();
  });

  it("marks the active link as the current page", () => {
    renderShell("/run");
    expect(screen.getByRole("link", { name: m.nav_run() })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: m.nav_map() })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("offers a skip link to the content as the first focusable element", () => {
    const { container } = renderShell();
    const skip = screen.getByRole("link", { name: m.action_skip_to_content() });
    expect(skip).toHaveAttribute("href", "#main-content");
    const first = container.querySelector("a, button, [tabindex]");
    expect(first).toBe(skip);
    const target = container.querySelector("#main-content");
    expect(target).not.toBeNull();
    expect(target).toHaveAttribute("tabindex", "-1");
    expect(within(target as HTMLElement).getByText("content")).toBeVisible();
  });

  it("toggles the sidebar on Ctrl+B, but not from inside a text field", () => {
    const { container } = renderShell(
      "/map",
      <input aria-label="height" defaultValue="5" />,
    );
    const rail = container.querySelector("[data-state]");
    expect(rail).toHaveAttribute("data-state", "expanded");

    fireEvent.keyDown(screen.getByLabelText("height"), {
      key: "b",
      ctrlKey: true,
    });
    expect(rail).toHaveAttribute("data-state", "expanded");

    fireEvent.keyDown(window, { key: "b", ctrlKey: true });
    expect(rail).toHaveAttribute("data-state", "collapsed");
  });
});
