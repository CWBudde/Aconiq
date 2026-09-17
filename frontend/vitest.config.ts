import { fileURLToPath, URL } from "url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    // Unit tests live under `src/`. Without this, vitest's default include also
    // picks up `e2e/*.spec.ts`, which is Playwright's and needs a real browser.
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    environment: "jsdom",
    setupFiles: ["./src/vitest.setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text-summary", "json-summary", "html"],
      reportsDirectory: "coverage",
      // Not optional, and the single most important line here. Left unset, the
      // denominator becomes "files some test happened to import" — every
      // untested module drops out of it instead of reading 0%, and the
      // headline describes the tested subset rather than the app. That default
      // only ever errs in the flattering direction, which is why it has to be
      // pinned rather than inherited.
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        // Paraglide's generated runtime: `bun run compile:i18n` writes it, it
        // is gitignored, and nobody reviews or tests it.
        "src/i18n/**",
        "**/*.test.{ts,tsx}",
        "**/*.d.ts",
        // The Vite entry point — `createRoot(...).render(...)` against a node
        // that only exists in index.html. Loading it under jsdom would mount
        // the whole app to measure four statements.
        "src/main.tsx",
        // src/vitest.setup.ts is deliberately absent: the runner appends every
        // `setupFiles` entry to this list itself, in resolveConfig. Repeating
        // it here would read like the line doing the work.
      ],
      // Report even when a test failed. The default is the opposite
      // (`reportOnFailure: false`), which skips the report — and with it the
      // thresholds — on exactly the runs whose numbers are most worth having,
      // and leaves the CI job with nothing to publish.
      reportOnFailure: true,
      // Two points under the measured baseline. docs/testing/coverage.md holds
      // the numbers, the date and the commit they were taken at; ratchet there,
      // never quietly here.
      //
      // Advisory: the `frontend-coverage` job that applies them is not a
      // required status check, so a breach is visible without blocking a merge.
      thresholds: {
        lines: 84,
        statements: 84,
        functions: 84,
        branches: 85,
      },
    },
  },
});
