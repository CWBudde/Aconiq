import js from "@eslint/js";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";
import globals from "globals";

export default tseslint.config(
  {
    ignores: [
      "dist/",
      "eslint.config.ts",
      "vite.config.ts",
      "vitest.config.ts",
      "src/i18n/**",
      // Build outputs, not sources: `just wasm-build` copies Go's wasm_exec.js
      // into public/ and it is gitignored. It only became visible to eslint
      // when the frontend gate started building the kernel, and it is not ours
      // to lint or to add to tsconfig.
      "public/",
      // Same: the HTML coverage report ships its own bundled scripts
      // (block-navigation.js, prettify.js, sorter.js), which eslint's project
      // service cannot resolve because they are in no tsconfig. Gitignored, and
      // only present after `just fe-test-coverage`.
      "coverage/",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  {
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
    },
  },
  // Build scripts run under Node and sit outside every tsconfig `include`, so
  // the type-aware rules have no program to work from. Lint them with the
  // syntax-only rules rather than pulling them into the app's tsconfig.
  {
    files: ["scripts/**/*.{js,mjs,cjs}"],
    extends: [tseslint.configs.disableTypeChecked],
    languageOptions: {
      globals: globals.node,
    },
  },
  // No user-visible prose outside the message catalogue.
  //
  // Scoped to the three directories that render the product: `src/pages`,
  // `src/map` and `src/ui`. Everything the rule reports here is either a
  // sentence that belongs in `messages/*.json` or one of the punctuation and
  // separator nodes below, which are presentation and translate to nothing.
  //
  // `noStrings` stays off on purpose: with it on the rule also polices every
  // string-valued prop, and the props that legitimately carry a literal
  // (`variant`, `side`, `data-*`, a Tailwind `className`) outnumber the prose
  // by two orders of magnitude. Bare JSX text is where English has actually
  // been leaking in — `map-view.tsx` and `sidebar.tsx` both leaked exactly
  // that way — so that is what this guards.
  //
  // Tests are excluded: they assert on rendered copy and `m.key()` is how they
  // do it, but the fixtures and labels they build are not shipped strings.
  {
    files: ["src/pages/**", "src/map/**", "src/ui/**"],
    ignores: ["**/*.test.ts", "**/*.test.tsx"],
    plugins: { react },
    rules: {
      "react/jsx-no-literals": [
        "error",
        {
          allowedStrings: [
            // The wordmark. A brand name is the same in every locale.
            "AconiQ",
            "AQ",
            // Punctuation and separators the JSX adds around a message,
            // because the catalogue holds the bare term — see the
            // `label_*` colon rule in `src/locale-parity.test.ts`.
            ":",
            "·",
            "×",
            "/",
            "(",
            ")",
            ",",
            "—",
            "–",
            "-",
            "…",
          ],
        },
      ],
    },
  },
  // Relax rules for shadcn/ui generated components (vendor-like code)
  {
    files: ["src/ui/components/**", "src/ui/hooks/**"],
    rules: {
      "@typescript-eslint/no-deprecated": "off",
      "@typescript-eslint/restrict-template-expressions": "off",
      "@typescript-eslint/no-confusing-void-expression": "off",
      "react-refresh/only-export-components": "off",
    },
  },
);
