import { test, expect } from "@playwright/test";
import type { Locator, Page } from "@playwright/test";
import { appPath, message, navLink, useLocale, waitForPage } from "./app";

/**
 * What the run dialog shows while the kernel is working, and what is left
 * behind when the user stops it.
 *
 * Only a real browser can answer either question. The kernel runs in a Web
 * Worker, and jsdom has none — every unit test mocks the module — so the
 * progress reports asserted here are the first ones in the suite that crossed
 * a `postMessage` from a Go computation, and the cancel is the first that
 * really terminated one.
 */

/**
 * A short road inside a large calculation area.
 *
 * The area is what makes the run long enough to watch: it replaces the source
 * extent for the auto receiver grid, so the number of receivers is set by the
 * polygon rather than by the road. A road long enough to produce the same grid
 * would also produce thousands of segments and take minutes. The kernel sizes
 * its chunks to report about ten times a second whatever a receiver costs, so
 * a grid of this size reports many times over — and a run that reported once,
 * at the end, would let a bar that never moved pass this file.
 *
 * Roughly 1.6 km by 0.8 km at 51.5°N; at the default 10 m grid that is about
 * 13,000 receivers.
 */
const SCENE = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { kind: "source", source_type: "line" },
      geometry: {
        type: "LineString",
        coordinates: [
          [10.5, 51.5],
          [10.504, 51.5],
        ],
      },
    },
    {
      type: "Feature",
      properties: { kind: "calc-area" },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [10.49, 51.4965],
            [10.513, 51.4965],
            [10.513, 51.5035],
            [10.49, 51.5035],
            [10.49, 51.4965],
          ],
        ],
      },
    },
  ],
};

/**
 * Imports {@link SCENE}, opens the run dialog and starts the run.
 *
 * Imported rather than drawn, for the reason the raster spec gives: the map is
 * a canvas whose clicks are hit-tested, and a file is the deterministic way to
 * a scene. Navigation after the import is in-app — a `page.goto` is a full
 * load, and the dialog would open on a model whose IndexedDB draft had not
 * been rehydrated yet.
 *
 * The locale is pinned by the caller, not here: `useLocale` reads as a React
 * hook to `react-hooks/rules-of-hooks`, and a named helper that calls one is
 * a lint error however little React there is in an E2E spec.
 */
async function startRun(page: Page): Promise<void> {
  await page.goto(appPath("/import"));
  await waitForPage(page);

  await page.setInputFiles('input[type="file"]', {
    name: "scene.geojson",
    mimeType: "application/geo+json",
    buffer: Buffer.from(JSON.stringify(SCENE)),
  });
  await page.getByRole("button", { name: /^Import \d+ /i }).click();

  await navLink(page, message("en", "nav_run")).click();
  await waitForPage(page);
  await page
    .getByRole("button", { name: message("en", "action_new_run") })
    .click();

  const start = page.getByRole("button", {
    name: message("en", "action_start_run"),
  });
  await expect(start).toBeEnabled({ timeout: 15_000 });
  await start.click();
}

/** The bar's `aria-valuenow`, or `null` once the bar is gone. */
async function receiversDone(bar: Locator): Promise<number | null> {
  try {
    const value = await bar.getAttribute("aria-valuenow", { timeout: 1_000 });
    return value === null ? null : Number(value);
  } catch {
    // The run finished and the dialog closed under the poll. That is an
    // outcome, not a failure — the caller decides whether it saw enough
    // first.
    return null;
  }
}

test.describe("Run progress", () => {
  test("shows a determinate bar that advances as receivers are computed", async ({
    page,
  }) => {
    await useLocale(page, "en");
    await startRun(page);

    const bar = page.getByRole("progressbar", {
      name: message("en", "label_run_progress"),
    });
    await expect(bar).toBeVisible({ timeout: 30_000 });

    // Determinate, and about the right thing: the maximum is the receiver
    // count, not a percentage, so a screen reader reads "512 of 13041" rather
    // than a number with no unit.
    const total = Number(await bar.getAttribute("aria-valuemax"));
    expect(total).toBeGreaterThan(256);

    // Sampled rather than asserted once: the worker throttles to about ten
    // reports a second, so two distinct values are evidence the bar is being
    // driven by the computation and not painted once at the end.
    const seen: number[] = [];
    const deadline = Date.now() + 60_000;
    while (Date.now() < deadline) {
      const done = await receiversDone(bar);
      if (done === null) break;
      if (seen.at(-1) !== done) seen.push(done);
      if (seen.length >= 3) break;
    }

    expect(
      seen.length,
      `receiver counts observed on the bar: ${seen.join(", ")}`,
    ).toBeGreaterThanOrEqual(2);
    // Monotonic: a bar that went backwards would mean a stale report won a
    // race with a newer one.
    expect([...seen].sort((a, b) => a - b)).toEqual(seen);
    expect(seen.at(-1)).toBeLessThanOrEqual(total);

    // And the run still finishes: the reporting must not be what ends it.
    await expect(page.getByRole("dialog")).toBeHidden({ timeout: 120_000 });
  });

  test("cancelling stops the run and leaves the project without one", async ({
    page,
  }) => {
    await useLocale(page, "en");
    await startRun(page);

    const bar = page.getByRole("progressbar", {
      name: message("en", "label_run_progress"),
    });
    await expect(bar).toBeVisible({ timeout: 30_000 });

    await page
      .getByRole("button", { name: message("en", "action_cancel_run") })
      .click();

    // Neutral, not an error: nothing went wrong, and nothing was written.
    const notice = page.getByTestId("run-cancelled");
    await expect(notice).toBeVisible({ timeout: 30_000 });
    await expect(notice).toHaveText(message("en", "msg_run_cancelled"));
    await expect(notice).toHaveAttribute("data-variant", "warning");
    // The bar goes with the run it was reporting on.
    await expect(bar).toBeHidden();

    // The dialog is still open and can start another run — the form was never
    // replaced by a failure the user has to dismiss first.
    await expect(
      page.getByRole("button", { name: message("en", "action_start_run") }),
    ).toBeEnabled();

    // And the project gained nothing. The run id was minted inside the store
    // lock and never used, so the list is as empty as it was before.
    await page
      .getByRole("button", { name: message("en", "action_cancel") })
      .click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(
      page.getByText(message("en", "msg_no_runs_empty_state")),
    ).toBeVisible();
  });
});
