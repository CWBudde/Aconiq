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

/**
 * A complex message: the array-wrapped form `@inlang/plugin-message-format`
 * takes for anything selected by a plural category. The array wrapper is what
 * distinguishes it from a nested object holding more messages.
 */
interface VariantMessage {
  declarations?: string[];
  selectors?: string[];
  match: Record<string, string>;
}

type Message = string | VariantMessage[];

// Resolved against the Vitest root (frontend/) rather than `import.meta.url`:
// the jsdom environment does not give this module a file: URL.
function catalogue(locale: Locale): Record<string, Message> {
  const file = join(process.cwd(), "messages", `${locale}.json`);
  return JSON.parse(readFileSync(file, "utf8")) as Record<string, Message>;
}

/**
 * Every string a message can render.
 *
 * One for a simple message, one per variant for a complex one. Every check
 * below runs over this rather than over the raw value, because a variant's
 * value is an array and `typeof value !== "string"` would either fail the whole
 * file or, for the placeholder check, quietly return nothing and pass.
 */
function patterns(message: Message): string[] {
  if (typeof message === "string") {
    return [message];
  }

  return message.flatMap((variant) => Object.values(variant.match));
}

/** The variants of a complex message; empty for a simple one. */
function variants(message: Message): VariantMessage[] {
  return typeof message === "string" ? [] : message;
}

function messageKeys(locale: Locale): string[] {
  return Object.keys(catalogue(locale)).filter(isMessageKey);
}

/** `{name}` placeholders paraglide turns into function parameters. */
function placeholders(pattern: string): string[] {
  return [...pattern.matchAll(/\{([A-Za-z0-9_]+)\}/g)].map(
    (match) => match[1] ?? "",
  );
}

/**
 * The placeholders a message uses, across every variant it holds.
 *
 * A union rather than a per-variant comparison: a locale may legitimately drop
 * the number from one arm — English "one item" against German "{count} Eintrag"
 * — while the *function* paraglide compiles still takes the same parameters.
 * What must not differ is the set the two catalogues ask the call sites for.
 */
function placeholdersOf(message: Message): string[] {
  return [...new Set(patterns(message).flatMap(placeholders))].sort();
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
      .filter(([, value]) => {
        const rendered = patterns(value);
        return (
          rendered.length === 0 ||
          rendered.some(
            (pattern) => typeof pattern !== "string" || pattern.trim() === "",
          )
        );
      })
      .map(([key]) => key);

    expect(blank).toEqual([]);
  });

  it.each(LOCALES)("give every variant message a catch-all in %s", (locale) => {
    // `Intl.PluralRules` answers with a category this catalogue may not list —
    // `many` for a language added later, or simply a count no branch matched.
    // Without an `other` (or `*`) arm paraglide has nothing to return and the
    // sentence renders empty, which is a blank the check above cannot see
    // because every arm it *does* declare is fine.
    const uncovered = Object.entries(catalogue(locale))
      .filter(([key]) => isMessageKey(key))
      .filter(([, value]) =>
        variants(value).some(
          (variant) =>
            !Object.keys(variant.match).some((arm) =>
              arm
                .split(",")
                .every((clause) =>
                  ["other", "*"].includes(clause.split("=")[1]?.trim() ?? ""),
                ),
            ),
        ),
      )
      .map(([key]) => key);

    expect(uncovered).toEqual([]);
  });

  it.each(LOCALES)("punctuate no label_* message in %s", (locale) => {
    // A label's colon is presentation, and the same term often labels a form
    // field as well as a value, where a colon would be wrong. So the message
    // is the bare term and the JSX punctuates it; a colon typed into the
    // catalogue reappears next to the one the page already adds.
    const punctuated = Object.entries(catalogue(locale))
      .filter(([key]) => isMessageKey(key) && key.startsWith("label_"))
      .filter(([, value]) =>
        patterns(value).some((pattern) => pattern.trimEnd().endsWith(":")),
      )
      .map(([key]) => key);

    expect(punctuated).toEqual([]);
  });

  it.each(LOCALES)("spell no plural in parentheses in %s", (locale) => {
    // "1 export bundle(s)" is a count that was never given to the message:
    // the sentence was assembled in JSX and the number stayed outside, so no
    // plural rule could fire and the author wrote both endings at once. The
    // fix is a variant message, and this is the check that the shortcut does
    // not come back — including in German, where the endings differ
    // ("Warnung(en)", "Export-Paket(e)", "Immissionsort(e)").
    const parenthesised = Object.entries(catalogue(locale))
      .filter(([key]) => isMessageKey(key))
      .filter(([, value]) =>
        patterns(value).some((pattern) =>
          /\((?:s|e|en|n|er|innen)\)/i.test(pattern),
        ),
      )
      .map(([key]) => key);

    expect(parenthesised).toEqual([]);
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
        en: placeholdersOf(en[key] ?? ""),
        de: placeholdersOf(de[key] ?? ""),
      }))
      .filter(({ en: a, de: b }) => a.join(",") !== b.join(","));

    expect(mismatched).toEqual([]);
  });
});
