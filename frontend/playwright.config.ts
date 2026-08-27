import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright config for E2E tests.
 * Run with: just fe-e2e
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
    baseURL: "http://localhost:5173",
    trace: "on-first-retry",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],

  // Start Vite dev server automatically when running E2E locally.
  webServer: {
    command: "bun run dev",
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
