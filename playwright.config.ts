import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright config for E2E tests.
 * Run with: npx playwright test
 * Requires a running dev server: just fe-dev (or `cd frontend && bun run dev`)
 */
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
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
    command: "cd frontend && npx vite",
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
