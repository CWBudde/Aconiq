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
    expect(screen.queryByRole("link", { name: m.nav_model() })).toBeNull();
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
      <MemoryRouter initialEntries={["/model"]}>
        <AppShell>
          <div>content</div>
        </AppShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: m.nav_import() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_model() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_run() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_results() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_export() })).toBeVisible();
  });
});

describe("AppShell landmarks", () => {
  function renderShell(path = "/model", children?: React.ReactNode) {
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
    expect(
      within(nav).getByRole("link", { name: m.nav_model() }),
    ).toBeVisible();
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
    expect(
      screen.getByRole("link", { name: m.nav_model() }),
    ).not.toHaveAttribute("aria-current");
  });

  it("marks a section's link as current on a path below it", () => {
    // The rail matched by exact equality until the run routes were
    // parameterised, so selecting a run un-highlighted the whole rail.
    renderShell("/results/run-1");
    expect(screen.getByRole("link", { name: m.nav_results() })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("link", { name: m.nav_export() }),
    ).not.toHaveAttribute("aria-current");
  });

  it("marks nothing current on a URL the route table sends to not-found", () => {
    // `/settings/typo` and `/results/a/b` both render NotFoundPage, so the
    // rail must not claim them: a plain prefix match lit Settings and Results
    // on pages that do not exist.
    const { unmount } = renderShell("/settings/typo");
    expect(
      screen.getByRole("link", { name: m.nav_settings() }),
    ).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      m.nav_workspace(),
    );
    unmount();

    renderShell("/results/a/b");
    expect(
      screen.getByRole("link", { name: m.nav_results() }),
    ).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      m.nav_workspace(),
    );
  });

  it("titles the header from the matched route, not from an exact path", () => {
    renderShell("/results/run-1");
    // The shell renders exactly one h1, in the header.
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      m.nav_results(),
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
      "/model",
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
