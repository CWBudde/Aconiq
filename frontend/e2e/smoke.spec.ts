import { test, expect } from "@playwright/test";
import { appPath, message, navLink, useLocale, waitForPage } from "./app";

/**
 * Smoke E2E test: verifies the app shell loads and the primary navigation works.
 * Run with: just fe-e2e
 *
 * The dev server runs in WASM mode (see playwright.config.ts), so the browser
 * backend always reports a project and the full workspace rail is present.
 */

const WORKSPACE_NAV = [
  "nav_model",
  "nav_import",
  "nav_run",
  "nav_results",
  "nav_export",
] as const;

test.describe("App shell", () => {
  test("loads and redirects / to /welcome under the base path", async ({
    page,
  }) => {
    await page.goto(appPath("/"));
    await expect(page).toHaveURL(/\/Aconiq\/welcome$/);
    await expect(page.getByText("AconiQ", { exact: true })).toBeVisible();
  });

  test("workspace navigation items are present", async ({ page }) => {
    await useLocale(page, "en");
    await page.goto(appPath("/model"));
    await waitForPage(page);
    for (const key of WORKSPACE_NAV) {
      await expect(navLink(page, message("en", key))).toBeVisible();
    }
  });

  test("navigates to Import page", async ({ page }) => {
    await useLocale(page, "en");
    await page.goto(appPath("/model"));
    await waitForPage(page);
    await navLink(page, message("en", "nav_import")).click();
    await expect(page).toHaveURL(/\/Aconiq\/import$/);
    await expect(
      page.getByRole("heading", {
        name: message("en", "heading_import_geojson"),
      }),
    ).toBeVisible();
  });
});

test.describe("Keyboard navigation", () => {
  test("the skip link is the first Tab stop and moves focus to the content", async ({
    page,
  }) => {
    await useLocale(page, "en");
    await page.goto(appPath("/model"));
    await waitForPage(page);
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", {
      name: message("en", "action_skip_to_content"),
    });
    await expect(skip).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.locator("#main-content")).toBeFocused();
  });

  test("sidebar links are reachable by Tab", async ({ page }) => {
    await page.goto(appPath("/model"));
    await waitForPage(page);
    // Tab through the first few focusable elements until a sidebar link has focus.
    const focusedLink = page.locator('[data-sidebar="sidebar"] a[href]:focus');
    for (let i = 0; i < 5 && (await focusedLink.count()) === 0; i++) {
      await page.keyboard.press("Tab");
    }
    await expect(focusedLink).toBeVisible();
  });
});

test.describe("Locale", () => {
  test("PARAGLIDE_LOCALE=de renders the German navigation labels", async ({
    page,
  }) => {
    await useLocale(page, "de");
    await page.goto(appPath("/model"));
    await waitForPage(page);
    // `nav_import` reads "Import" in both locales, so on its own it proves
    // nothing; the whole rail includes labels that differ (Karte, Berechnung,
    // Ergebnisse), and only the German set is expected to be present.
    for (const key of WORKSPACE_NAV) {
      await expect(navLink(page, message("de", key))).toBeVisible();
    }
    await expect(navLink(page, message("en", "nav_model"))).toHaveCount(0);
  });
});
