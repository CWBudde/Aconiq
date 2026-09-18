import { test, expect } from "@playwright/test";
import type { Locator, Page } from "@playwright/test";
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
  test("serves the project page at / under the base path", async ({ page }) => {
    await page.goto(appPath("/"));
    // Asserting the URL did *not* move is the guard: `/` used to redirect to
    // `/welcome`, and without this the redirect could creep back unnoticed.
    await expect(page).toHaveURL(/\/Aconiq\/$/);
    await waitForPage(page);
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

  /**
   * Presses Tab until `focused` matches something, then asserts it does. The
   * counting pattern is the one the sidebar test below uses: a `:focus`
   * locator and a bounded loop, rather than `evaluate` against
   * `document.activeElement`, because the E2E tsconfig has no DOM lib.
   */
  async function tabUntil(page: Page, focused: Locator, limit = 60) {
    for (let i = 0; i < limit && (await focused.count()) === 0; i++) {
      await page.keyboard.press("Tab");
    }
    await expect(focused).toHaveCount(1);
  }

  test("places a feature by typing coordinates, and selects it from the list", async ({
    page,
  }) => {
    // The map is a canvas: its selection is a hit-tested click, which no
    // keyboard produces. This is the whole keyboard path end to end — reach
    // the coordinate-entry control by Tab, type a position, and then find the
    // feature again in the list and open its editor without a pointer.
    //
    // The coordinates are typed in the CRS the model is stored in and go into
    // the store untouched; nothing on this path is projected, which is why it
    // is not a second coordinate writer beside `use-draw-projection.ts`.
    await useLocale(page, "en");
    await page.goto(appPath("/model"));
    await waitForPage(page);

    const entryLabel = message("en", "action_enter_coordinates");
    await tabUntil(
      page,
      page.locator(`button[aria-label="${entryLabel}"]:focus`),
      40,
    );
    await page.keyboard.press("Enter");

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    // EPSG:4326 is the store's default, and the field labels say so.
    await dialog.getByLabel("X (EPSG:4326)").fill("10.5");
    await dialog.getByLabel("Y (EPSG:4326)").fill("51.5");

    const addLabel = message("en", "action_add_feature");
    await tabUntil(
      page,
      page.locator("button:focus").filter({ hasText: addLabel }),
    );
    await page.keyboard.press("Enter");
    await expect(dialog).toBeHidden();

    // The list counts what the dialog added, and Enter on its entry opens the
    // docked editor — the same thing a click on the canvas does.
    const listLabel = message("en", "label_feature_list");
    await tabUntil(
      page,
      page.locator("button:focus").filter({ hasText: listLabel }),
    );
    await page.keyboard.press("Enter");

    const list = page.getByRole("region", { name: listLabel });
    await expect(list.getByRole("button")).toHaveCount(1);
    await tabUntil(
      page,
      list.locator("button:focus").filter({
        hasText: message("en", "option_source"),
      }),
    );
    await page.keyboard.press("Enter");

    await expect(page.getByRole("dialog")).toBeVisible();
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

  test("puts the selected run in the URL, and Back returns to no selection", async ({
    page,
  }) => {
    // The history behaviour the parameterised routes exist for. Asserted from
    // an id that cannot exist, because the WASM dev server starts with no runs
    // and there is nothing in the list to click.
    await useLocale(page, "en");
    await page.goto(appPath("/results"));
    await waitForPage(page);

    await page.goto(appPath("/results/does-not-exist"));
    await waitForPage(page);
    await expect(page.getByRole("alert")).toContainText("does-not-exist");

    await page.goBack();
    await expect(page).toHaveURL(/\/Aconiq\/results$/);
  });
});

test.describe("The result raster", () => {
  test("a completed run paints a raster the map accepts", async ({ page }) => {
    // The one thing jsdom cannot answer. `raster-canvas.ts` is mocked in every
    // unit test because `HTMLCanvasElement.prototype.getContext` throws there,
    // so until here nothing had run a real `getContext("2d")`, a real
    // `toDataURL`, or handed MapLibre an `image` source to upload as a texture.
    //
    // Asserted through the console rather than through a map handle. Every
    // failure on that path is reported by `ResultLayers` or `useResultRaster`
    // with `console.error` — a refused source, an encoder that returned null, a
    // georeference the mirror would not place — so watching for those covers
    // the whole path without exposing the map on `window` for a test's
    // convenience. No pixels are asserted: a WebGL screenshot varies by GPU and
    // driver and would fail for reasons unrelated to this.
    const problems: string[] = [];
    page.on("console", (entry) => {
      if (entry.type() === "error") problems.push(entry.text());
    });
    page.on("pageerror", (error) => problems.push(error.message));

    await useLocale(page, "en");
    await page.goto(appPath("/import"));
    await waitForPage(page);

    // A road, imported rather than drawn. RLS-19 refuses a model with no line
    // source, and the map is a canvas whose clicks are hit-tested — which is
    // why the keyboard spec above types a *point*. A file is the deterministic
    // way to a completed run, and this test is about what happens after one.
    await page.setInputFiles('input[type="file"]', {
      name: "road.geojson",
      mimeType: "application/geo+json",
      buffer: Buffer.from(
        JSON.stringify({
          type: "FeatureCollection",
          features: [
            {
              type: "Feature",
              properties: { kind: "source", source_type: "line" },
              geometry: {
                type: "LineString",
                coordinates: [
                  [10.5, 51.5],
                  [10.503, 51.5],
                ],
              },
            },
          ],
        }),
      ),
    });

    // The label counts objects, not features, and is built from a message
    // with a `{count}` — matched by prefix so the count does not pin the test.
    await page.getByRole("button", { name: /^Import \d+ /i }).click();

    // In-app navigation from here on. A `page.goto` is a full load, and the
    // imported model lives in the store with an IndexedDB draft behind it —
    // the run dialog would open on an empty model before the draft had been
    // rehydrated.
    await navLink(page, message("en", "nav_run")).click();
    await waitForPage(page);
    await page
      .getByRole("button", { name: message("en", "action_new_run") })
      .click();
    // No Save step: an import through this page reaches the project itself, so
    // the dialog's unsaved-changes gate does not fire here. That is an
    // assertion and not an omission — if the gate ever did fire, this is where
    // the test would stop, on a Start Run that never enabled.
    const start = page.getByRole("button", {
      name: message("en", "action_start_run"),
    });
    await expect(start).toBeEnabled({ timeout: 15_000 });
    await start.click();

    // The dialog is modal, so the sidebar is unreachable until it closes.
    await expect(page.getByRole("dialog")).toBeHidden({ timeout: 30_000 });

    await navLink(page, message("en", "nav_model")).click();
    await waitForPage(page);

    // The panel is only rendered once a completed run has been resolved, so
    // waiting for the legend is waiting for the whole result path.
    await expect(
      page.getByText(message("en", "label_result_legend"), { exact: false }),
    ).toBeVisible({ timeout: 30_000 });

    // No refusal reached the panel either: those are the states that mean the
    // raster was not drawn, and this run is an ordinary auto-grid.
    await expect(
      page.getByText(message("en", "msg_result_raster_failed")),
    ).toHaveCount(0);
    await expect(
      page.getByText(message("en", "msg_result_raster_not_grid")),
    ).toHaveCount(0);

    expect(
      problems.filter((text) => /raster/i.test(text)),
      `console errors mentioning the raster:\n${problems.join("\n")}`,
    ).toEqual([]);
  });
});
