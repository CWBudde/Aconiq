import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { Locator, Page } from "@playwright/test";

/**
 * Shared knowledge about how the app is served, for every spec in e2e/.
 */

/**
 * The path the app is mounted under. Must match `base` in vite.config.ts (and
 * therefore `import.meta.env.BASE_URL`, which the router uses as `basename`).
 * Playwright's `baseURL` carries it too, but `page.goto("/model")` resolves an
 * absolute path against the origin and would drop it, so every navigation in
 * the specs goes through `appPath()` instead.
 */
export const BASE_PATH = "/Aconiq";

/**
 * The routes the accessibility baseline visits. Every route `src/routes.tsx`
 * registers, plus one bogus run id per parameterised section: the unknown-run
 * warning is a `role="alert"` on a tinted surface, and neither its contrast
 * nor its announcement is something jsdom can check.
 */
export const ROUTES = [
  "/welcome",
  "/model",
  "/import",
  "/run",
  "/results",
  "/results/does-not-exist",
  "/export",
  "/export/does-not-exist",
  "/status",
  "/settings",
] as const;

export type Route = (typeof ROUTES)[number];

/** The locales project.inlang declares; `en` is the base locale. */
export const LOCALES = ["en", "de"] as const;

export type Locale = (typeof LOCALES)[number];

/** Returns the base-prefixed path for an app route, e.g. `/model` → `/Aconiq/model`. */
export function appPath(route: string): string {
  return `${BASE_PATH}${route}`;
}

/**
 * Pins the UI locale for every page load in this context. Paraglide's strategy
 * (see vite.config.ts) reads localStorage first, under the key the generated
 * runtime exports as `localStorageKey` (src/i18n/runtime.js), so seeding it
 * before the app script runs wins over the browser's preferred language. The
 * key is duplicated here as a literal; the "renders the German navigation
 * labels" smoke test is the guard that catches a Paraglide rename of it.
 */
export async function useLocale(page: Page, locale: Locale): Promise<void> {
  // A source string rather than a function: the e2e tsconfig has no DOM lib,
  // so `window` does not exist at type level here, while the script itself
  // runs in the browser.
  await page.addInitScript(
    `window.localStorage.setItem("PARAGLIDE_LOCALE", ${JSON.stringify(locale)});`,
  );
}

/**
 * The UI strings of a locale, straight from messages/<locale>.json. Specs
 * assert on these rather than on hardcoded English so the same test runs in
 * every locale.
 */
export function messages(locale: Locale): Record<string, string> {
  const file = fileURLToPath(
    new URL(`../messages/${locale}.json`, import.meta.url),
  );
  return JSON.parse(readFileSync(file, "utf8")) as Record<string, string>;
}

/** One UI string; throws when the key is missing so a typo cannot pass as a match. */
export function message(locale: Locale, key: string): string {
  const value = messages(locale)[key];
  if (value === undefined) {
    throw new Error(`messages/${locale}.json has no key "${key}"`);
  }
  return value;
}

/**
 * Resolves once the route's own content has rendered. The header `<h1>` is
 * part of the shell and shows up before the lazily loaded page chunk, while a
 * skeleton fills the content area; every route then renders at least one
 * `<h2>` or `<h3>` inside `main`, so that is the signal the page is up.
 */
export async function waitForPage(page: Page): Promise<void> {
  await page.locator("header h1").waitFor();
  await page.locator("main :is(h2, h3)").first().waitFor();
}

/**
 * A link in the sidebar rail by its accessible name. Scoped to the sidebar
 * because pages repeat rail targets in their content (the map route's
 * workspace-start panel has its own "Import" link), and an unscoped role query
 * would then be ambiguous.
 */
export function navLink(page: Page, name: string): Locator {
  return page
    .locator('[data-sidebar="sidebar"]')
    .getByRole("link", { name, exact: true });
}
