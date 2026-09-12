import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright config for E2E tests.
 * Run with: just fe-e2e
 *
 * The app is served under `base: "/Aconiq/"` (vite.config.ts), so the dev
 * server answers at http://localhost:5174/Aconiq/ and the bare origin is a 404.
 * `baseURL` therefore carries the base path. Note that `page.goto("/map")`
 * would still resolve against the origin and drop it; the specs go through
 * `appPath()` in e2e/app.ts, which prefixes the base explicitly.
 */
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // `workers` is optional and the tsconfig sets `exactOptionalPropertyTypes`, so it
  // has to be omitted rather than set to undefined. Absent, Playwright uses its
  // default of half the logical cores.
  ...(process.env.CI ? { workers: 1 } : {}),
  reporter: "html",

  use: {
    baseURL: "http://localhost:5174/Aconiq/",
    trace: "on-first-retry",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    // The accessibility baseline runs a second time under the dark colour
    // scheme. The ThemeProvider defaults to "system", so the emulated scheme
    // selects the `.dark` token set and axe sees the dark palette. The other
    // specs are not colour-dependent and run once.
    {
      name: "chromium-dark",
      use: { ...devices["Desktop Chrome"], colorScheme: "dark" },
      testMatch: /a11y\.spec\.ts/,
    },
  ],

  // Start the Vite dev server automatically. It runs in WASM mode on purpose:
  // in HTTP mode the app needs `aconiq serve` on :8080 behind the Vite proxy,
  // and without it no project loads and the sidebar collapses to Import plus
  // Status/Settings, so most routes never render their real content. The
  // browser backend always reports a project, so every route is exercised, the
  // suite is self-contained (no Go server to start), and it mirrors the GitHub
  // Pages demo build (`just fe-build-wasm`). It needs public/aconiq.wasm and
  // public/wasm_exec.js, which `just wasm-build` produces; `just fe-e2e`
  // depends on that recipe.
  //
  // Port 5174 rather than Vite's configured 5173: `just dev` and `just fe-dev`
  // put an HTTP-mode server on 5173, and with `reuseExistingServer` Playwright
  // would silently test that backend instead. The CLI flag overrides
  // `server.port` in vite.config.ts (bun forwards the arguments verbatim), and
  // --strictPort makes a taken port a startup failure instead of a silent hop
  // to the next free one. Only an E2E server can be on 5174, so reusing one is
  // safe.
  webServer: {
    command: "VITE_WASM_MODE=true bun run dev -- --port 5174 --strictPort",
    url: "http://localhost:5174/Aconiq/",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
