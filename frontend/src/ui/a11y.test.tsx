/**
 * Accessibility smoke tests using axe-core.
 * Each test renders a component and checks for zero axe violations.
 */
import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import axe from "axe-core";
import type { RunOptions } from "axe-core";
import { ErrorBoundary } from "./error-boundary";
import { AppShell } from "./app-shell";

vi.mock("@/api/hooks", () => ({
  useProjectStatus: () => ({
    isLoading: false,
    isError: false,
    error: null,
    data: { name: "Demo", crs: "EPSG:4326", scenario_count: 1, run_count: 0 },
  }),
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

async function expectAccessible(container: Element, options: RunOptions = {}) {
  const results = await axe.run(container, options);
  if (results.violations.length > 0) {
    const summary = results.violations
      .map((v) => `[${v.id}] ${v.description} (${v.impact ?? "unknown"})`)
      .join("\n");
    throw new Error(
      `axe found ${String(results.violations.length)} violation(s):\n${summary}`,
    );
  }
  expect(results.violations).toHaveLength(0);
}

describe("Accessibility: ErrorBoundary", () => {
  it("default children state has no axe violations", async () => {
    const { container } = render(
      <ErrorBoundary>
        <main>
          <h1>Test page</h1>
          <p>Content here.</p>
        </main>
      </ErrorBoundary>,
    );
    await expectAccessible(container);
  });

  it("error fallback UI has no axe violations", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    function Throw(): never {
      throw new Error("boom");
    }
    const { container } = render(
      <ErrorBoundary>
        <Throw />
      </ErrorBoundary>,
    );
    await expectAccessible(container);
    spy.mockRestore();
  });
});

describe("Accessibility: AppShell", () => {
  // The structural rules the E2E baseline (e2e/a11y.spec.ts) once listed as
  // known violations: one top-level <main>, every element inside a landmark,
  // and contiguous heading levels. Pinned here too, where a run costs
  // milliseconds rather than a browser.
  it("has no landmark or heading-order violations", async () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/map"]}>
        <AppShell>
          <h2>Page</h2>
          <p>Content here.</p>
        </AppShell>
      </MemoryRouter>,
    );
    await expectAccessible(container, {
      runOnly: {
        type: "rule",
        values: [
          "landmark-no-duplicate-main",
          "landmark-main-is-top-level",
          "landmark-unique",
          "landmark-one-main",
          "region",
          "heading-order",
          "page-has-heading-one",
        ],
      },
    });
  });
});
