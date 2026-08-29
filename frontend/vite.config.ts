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
    // Keep these three options in sync with the `compile:i18n` script in
    // package.json. The plugin only generates src/i18n/ when Vite runs, but
    // `tsc` needs those modules to exist beforehand — and the directory is
    // gitignored, so on a fresh clone (every CI run) nothing has produced it
    // yet. `typecheck` and `build` therefore invoke the paraglide CLI first,
    // and it has to be told the same project, outdir and strategy this plugin
    // uses or the two would generate different runtimes.
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
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
