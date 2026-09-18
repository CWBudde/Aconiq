/**
 * The compiler script and the Vite plugin must be told the same thing.
 *
 * `compile:i18n` generates `src/i18n/` before `tsc` runs, because the directory
 * is gitignored and on a fresh clone — every CI run — nothing has produced it
 * yet. The Vite plugin generates the same directory when Vite runs. Both
 * comments say "keep these in sync"; nothing checked it, and a divergence would
 * produce two different runtimes from the same tree, whichever ran last.
 *
 * The three options are read out of the two files as text rather than imported.
 * `vite.config.ts` cannot be imported here — it pulls the plugin graph into the
 * test environment — and `compile-i18n.mjs` spawns a compiler on import. The
 * regexes are therefore the price of the check, and a rename that defeats one
 * fails the test rather than passing it: a missing match is an assertion
 * failure, not an empty comparison that happens to agree.
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

function read(file: string): string {
  return readFileSync(join(process.cwd(), file), "utf8");
}

/** The single-quoted or double-quoted string a named constant is set to. */
function stringAfter(source: string, pattern: RegExp): string | null {
  return pattern.exec(source)?.[1] ?? null;
}

/** The members of a `[...]` array literal that follows a label. */
function arrayAfter(source: string, pattern: RegExp): string[] | null {
  const body = pattern.exec(source)?.[1];
  if (body === undefined) {
    return null;
  }

  return [...body.matchAll(/"([^"]+)"/g)].map((match) => match[1] ?? "");
}

describe("paraglide options", () => {
  const script = read("scripts/compile-i18n.mjs");
  const vite = read("vite.config.ts");

  it("name the same inlang project", () => {
    const fromScript = stringAfter(script, /const PROJECT = "([^"]+)"/);
    const fromVite = stringAfter(vite, /project: "([^"]+)"/);

    expect(fromScript).not.toBeNull();
    expect(fromVite).toBe(fromScript);
  });

  it("write to the same outdir", () => {
    // A mismatch here is the quiet one: `tsc` would resolve `@/i18n` against
    // whatever the script wrote while the dev server served the plugin's copy.
    const fromScript = stringAfter(script, /const OUTDIR = "([^"]+)"/);
    const fromVite = stringAfter(vite, /outdir: "([^"]+)"/);

    expect(fromScript).not.toBeNull();
    expect(fromVite).toBe(fromScript);
  });

  it("resolve the locale by the same strategy", () => {
    // The strategy is compiled *into* the runtime, so the two copies would
    // disagree about where the locale comes from — and `src/locale.ts` assumes
    // the localStorage strategy, under which a switch changes no URL.
    const fromScript = arrayAfter(script, /const STRATEGY = \[([^\]]*)\]/);
    const fromVite = arrayAfter(vite, /strategy: \[([^\]]*)\]/);

    expect(fromScript).not.toBeNull();
    expect(fromVite).toEqual(fromScript);
  });
});
