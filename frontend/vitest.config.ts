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
  },
});
