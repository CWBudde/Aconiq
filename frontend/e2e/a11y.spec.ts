import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { Result } from "axe-core";
import {
  LOCALES,
  ROUTES,
  appPath,
  message,
  navLink,
  useLocale,
  waitForPage,
} from "./app";
import type { Route } from "./app";

/**
 * Accessibility baseline: axe-core on every route in every locale.
 * Run with: just fe-e2e
 *
 * Two bars, checked in one axe pass:
 *
 * - WCAG 2.0 A/AA (`wcag2a`, `wcag2aa`) must be clean. There is no allow-list
 *   for it; nothing fires today and nothing may start to.
 * - axe's `best-practice` rules are gated by KNOWN_VIOLATIONS below. That set
 *   is where the structural defects scheduled for PLAN.md Priority 8 Phase B
 *   ("Structural a11y in one PR") show up — WCAG alone does not see a nested
 *   `<main>` or a rail without a `<nav>` landmark.
 *
 * KNOWN_VIOLATIONS is checked in both directions: a rule outside the list
 * fails the route, and a listed rule that no longer fires fails it too, so the
 * list has to be pruned in the same change that fixes the defect. An allow-list
 * that outlives its defect is a lie.
 *
 * Beyond axe, every route asserts `<html lang>` matches the locale under
 * test: index.html hardcodes `lang="en"` and src/main.tsx corrects it at
 * startup, which no axe rule can see (`html-has-lang` only wants a value).
 *
 * The map route with an empty model shows the workspace-start panel rather
 * than the map canvas; the baseline covers what renders.
 */

const WCAG_TAGS = ["wcag2a", "wcag2aa"] as const;
const BEST_PRACTICE_TAG = "best-practice";

/**
 * Best-practice rules that fire today, per route. Each entry names the Phase B
 * item that removes it. All observed with `just fe-e2e` on this tree; none is
 * speculative.
 */
const SHELL_VIOLATIONS: readonly string[] = [
  // Phase B "nested <main>": SidebarInset (ui/components/sidebar.tsx) renders a
  // <main> and app-shell.tsx wraps the page in a second one inside it. PLAN.md
  // names settings.tsx, which adds a third, but the pair is in the shell and
  // fires on every route.
  "landmark-no-duplicate-main",
  // Phase B "nested <main>": the inner <main> is not a top-level landmark.
  "landmark-main-is-top-level",
  // Phase B "nested <main>": two main landmarks without distinguishing labels.
  "landmark-unique",
  // Phase B "<nav aria-label> in app-shell.tsx": the sidebar header and the
  // rail links sit outside every landmark until the rail becomes a <nav>.
  "region",
];

const KNOWN_VIOLATIONS: Record<Route, readonly string[]> = {
  "/welcome": SHELL_VIOLATIONS,
  "/map": SHELL_VIOLATIONS,
  "/import": SHELL_VIOLATIONS,
  "/run": SHELL_VIOLATIONS,
  "/results": SHELL_VIOLATIONS,
  "/export": SHELL_VIOLATIONS,
  "/status": SHELL_VIOLATIONS,
  "/settings": [
    ...SHELL_VIOLATIONS,
    // Phase B "heading-level skip": settings.tsx goes from the shell <h1>
    // straight to <h3> section headings. (PLAN.md cites run.tsx, whose
    // h2 -> h4 skip only renders once runs exist; the empty model shows none.)
    "heading-order",
  ],
};

function summarize(route: Route, violations: Result[]): string {
  return violations
    .map(
      (v) =>
        `${v.impact ?? "unknown"} ${v.id} on ${route} (${String(v.nodes.length)} node(s)): ${v.help}`,
    )
    .join("\n");
}

for (const locale of LOCALES) {
  test.describe(`Accessibility baseline (${locale})`, () => {
    for (const route of ROUTES) {
      test(`${route} matches the baseline`, async ({ page }) => {
        await useLocale(page, locale);
        await page.goto(appPath(route));
        await waitForPage(page);
        // The workspace rail only renders once useProjectStatus has resolved;
        // waiting for its first link pins the baseline to the same DOM every
        // run rather than to whichever state axe happened to catch.
        await navLink(page, message(locale, "nav_map")).waitFor();

        await expect(page.locator("html")).toHaveAttribute("lang", locale);

        const results = await new AxeBuilder({ page })
          .withTags([...WCAG_TAGS, BEST_PRACTICE_TAG])
          .analyze();

        const isWcag = (v: Result) =>
          v.tags.some((t) => (WCAG_TAGS as readonly string[]).includes(t));
        const wcag = results.violations.filter(isWcag);
        const bestPractice = results.violations.filter((v) => !isWcag(v));

        expect
          .soft(wcag, `WCAG 2.0 A/AA violations:\n${summarize(route, wcag)}`)
          .toHaveLength(0);

        const known = KNOWN_VIOLATIONS[route];
        const unexpected = bestPractice.filter((v) => !known.includes(v.id));
        expect
          .soft(
            unexpected,
            `best-practice violations not in KNOWN_VIOLATIONS:\n${summarize(route, unexpected)}`,
          )
          .toHaveLength(0);

        const fired = new Set(bestPractice.map((v) => v.id));
        const stale = known.filter((id) => !fired.has(id));
        expect
          .soft(
            stale,
            `KNOWN_VIOLATIONS lists rules that no longer fire on ${route}; prune them: ${stale.join(", ")}`,
          )
          .toHaveLength(0);
      });
    }
  });
}
