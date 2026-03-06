import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

/**
 * Smoke E2E test: verifies the app shell loads and the primary navigation works.
 * Run with: npx playwright test
 */

test.describe("App shell", () => {
  test("loads and redirects to /map", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/map/);
    await expect(page.locator("text=AconiQ")).toBeVisible();
  });

  test("sidebar navigation items are present", async ({ page }) => {
    await page.goto("/map");
    for (const label of ["Map", "Import", "Run", "Results", "Export"]) {
      await expect(page.getByRole("link", { name: label })).toBeVisible();
    }
  });

  test("navigates to Import page", async ({ page }) => {
    await page.goto("/map");
    await page.getByRole("link", { name: "Import" }).click();
    await expect(page).toHaveURL(/\/import/);
    await expect(page.getByText("Import GeoJSON")).toBeVisible();
  });
});

test.describe("Keyboard navigation", () => {
  test("sidebar links are reachable by Tab", async ({ page }) => {
    await page.goto("/map");
    // Tab through focusable elements and find at least one nav link.
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    const focused = page.locator(":focus");
    await expect(focused).toBeVisible();
  });
});

test.describe("Accessibility: axe-core", () => {
  test("/map has no critical axe violations", async ({ page }) => {
    await page.goto("/map");
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    expect(results.violations).toHaveLength(0);
  });

  test("/import has no critical axe violations", async ({ page }) => {
    await page.goto("/import");
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    expect(results.violations).toHaveLength(0);
  });
});
