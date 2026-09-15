/**
 * The two message catalogues must stay in lockstep.
 *
 * Nothing in `src/` reads `messages/de.json` — the pages import the compiled
 * paraglide functions, which fall back to the base locale when a key is
 * missing from a translation. So a key added to `en.json` and forgotten in
 * `de.json` renders English text to a German user and no test, type or lint
 * rule notices. `e2e/app.ts`'s `message()` throws on a missing key, but only
 * for the one locale a spec happens to ask about.
 *
 * These assertions are the guard. They run over the raw JSON rather than the
 * generated runtime because the generated runtime is exactly what papers over
 * the difference.
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/** The locales `project.inlang/settings.json` declares; `en` is the base. */
const LOCALES = ["en", "de"] as const;

type Locale = (typeof LOCALES)[number];

/**
 * Inlang's own keys start with `$` (currently just `$schema`). They are file
 * metadata, not messages, so every check below ignores them.
 */
function isMessageKey(key: string): boolean {
  return !key.startsWith("$");
}

// Resolved against the Vitest root (frontend/) rather than `import.meta.url`:
// the jsdom environment does not give this module a file: URL.
function catalogue(locale: Locale): Record<string, string> {
  const file = join(process.cwd(), "messages", `${locale}.json`);
  return JSON.parse(readFileSync(file, "utf8")) as Record<string, string>;
}

function messageKeys(locale: Locale): string[] {
  return Object.keys(catalogue(locale)).filter(isMessageKey);
}

/** `{name}` placeholders paraglide turns into function parameters. */
function placeholders(message: string): string[] {
  return [...message.matchAll(/\{([A-Za-z0-9_]+)\}/g)]
    .map((match) => match[1] ?? "")
    .sort();
}

describe("message catalogues", () => {
  it("declare the same keys in every locale", () => {
    const en = new Set(messageKeys("en"));
    const de = new Set(messageKeys("de"));

    // Reported as sorted arrays rather than a bare boolean so a failure names
    // the keys to add instead of only saying the sets differ.
    expect({
      missingFromDe: [...en].filter((key) => !de.has(key)).sort(),
      missingFromEn: [...de].filter((key) => !en.has(key)).sort(),
    }).toEqual({ missingFromDe: [], missingFromEn: [] });
  });

  it("list their keys in the same order", () => {
    // Not cosmetic: the two files are edited together in every change that
    // touches UI copy, and matching order is what keeps those diffs readable
    // side by side.
    expect(messageKeys("de")).toEqual(messageKeys("en"));
  });

  it.each(LOCALES)("have no blank message in %s", (locale) => {
    const blank = Object.entries(catalogue(locale))
      .filter(([key]) => isMessageKey(key))
      .filter(([, value]) => typeof value !== "string" || value.trim() === "")
      .map(([key]) => key);

    expect(blank).toEqual([]);
  });

  it.each(LOCALES)("punctuate no label_* message in %s", (locale) => {
    // A label's colon is presentation, and the same term often labels a form
    // field as well as a value, where a colon would be wrong. So the message
    // is the bare term and the JSX punctuates it; a colon typed into the
    // catalogue reappears next to the one the page already adds.
    const punctuated = Object.entries(catalogue(locale))
      .filter(([key]) => isMessageKey(key) && key.startsWith("label_"))
      .filter(([, value]) => value.trimEnd().endsWith(":"))
      .map(([key]) => key);

    expect(punctuated).toEqual([]);
  });

  it("use the same placeholders in every locale", () => {
    // A translation that drops a `{count}` compiles and then renders a
    // sentence with a hole in it; one that invents a placeholder paraglide
    // never passes renders the braces literally.
    const en = catalogue("en");
    const de = catalogue("de");

    const mismatched = messageKeys("en")
      .map((key) => ({
        key,
        en: placeholders(en[key] ?? ""),
        de: placeholders(de[key] ?? ""),
      }))
      .filter(({ en: a, de: b }) => a.join(",") !== b.join(","));

    expect(mismatched).toEqual([]);
  });
});
