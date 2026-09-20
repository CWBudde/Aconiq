import { fileURLToPath, URL } from "url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { paraglideVitePlugin } from "@inlang/paraglide-js";
import { defineConfig } from "vite";

export default defineConfig({
  base: "/Aconiq/",
  plugins: [
    react(),
    tailwindcss(),
    // Keep these three options in sync with scripts/compile-i18n.mjs, which
    // `compile:i18n` runs. The plugin only generates src/i18n/ when Vite runs,
    // but `tsc` needs those modules to exist beforehand — and the directory is
    // gitignored, so on a fresh clone (every CI run) nothing has produced it
    // yet. `typecheck` and `build` therefore invoke that script first, and it
    // has to tell the compiler the same project, outdir and strategy this
    // plugin uses or the two would generate different runtimes.
    //
    // This plugin has no guard of its own: a failed plugin import would leave
    // a Vite build serving an empty catalogue. It does not need one, because
    // every path that reaches a build runs `compile:i18n` first.
    paraglideVitePlugin({
      project: "./project.inlang",
      outdir: "./src/i18n",
      strategy: ["localStorage", "preferredLanguage", "baseLocale"],
    }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  // The compute kernel runs in a module worker (`src/wasm/kernel.worker.ts`),
  // which is what lets it load `public/wasm_exec.js` with a dynamic `import()`
  // instead of `importScripts`. Vite's default worker format is "iife", and an
  // IIFE bundle cannot carry that import — so this has to match the
  // `{ type: "module" }` in `src/wasm/spawn-worker.ts` or the worker fails at
  // build time rather than at run time.
  worker: {
    format: "es",
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
