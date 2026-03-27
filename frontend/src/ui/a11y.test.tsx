/**
 * Accessibility smoke tests using axe-core.
 * Each test renders a component and checks for zero axe violations.
 */
import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import axe from "axe-core";
import { ErrorBoundary } from "./error-boundary";

async function expectAccessible(container: Element) {
  const results = await axe.run(container);
  if (results.violations.length > 0) {
    const summary = results.violations
      .map(
        (v) => `[${v.id}] ${v.description} (${String(v.impact ?? "unknown")})`,
      )
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
    function Throw() {
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
