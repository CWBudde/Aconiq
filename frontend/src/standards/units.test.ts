import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import * as units from "./units";

/**
 * The symbols in `units.ts` must be symbols the Go side publishes.
 *
 * The frontend names units only for model properties, which no descriptor
 * covers — but the reader does not know which half of a panel a field came
 * from, so both halves have to use one vocabulary. This reads
 * `framework.go`'s constants and holds the copy to them.
 *
 * Only a subset is required: the frontend names the units its own fields use,
 * not all nine. What is forbidden is a *sixth* spelling.
 */
function goUnitSymbols(): string[] {
  // Resolved against the test runner's root (frontend/) rather than
  // `import.meta.url`, for the reason `locale-parity.test.ts` gives: the jsdom
  // environment does not give this module a file: URL.
  const source = readFileSync(
    join(
      process.cwd(),
      "..",
      "backend/internal/standards/framework/framework.go",
    ),
    "utf8",
  );
  const block = /const \(\n(\s*Unit\w+\s*=[\s\S]*?)\n\)/.exec(source);
  expect(
    block,
    "the framework.Unit* const block was not found — has it been renamed?",
  ).not.toBeNull();
  return [...(block?.[1] ?? "").matchAll(/Unit\w+\s*=\s*"([^"]*)"/g)].map(
    (match) => match[1] ?? "",
  );
}

describe("frontend unit symbols", () => {
  it("finds the Go constants", () => {
    // Guards the regex: a changed declaration style must fail here rather than
    // turn the check below into a vacuous truth.
    const symbols = goUnitSymbols();
    expect(symbols).toContain("km/h");
    expect(symbols).toContain("dB/km");
    expect(symbols.length).toBeGreaterThanOrEqual(9);
  });

  it("spells every symbol the way the Go constants do", () => {
    const allowed = new Set(goUnitSymbols());
    for (const [name, symbol] of Object.entries(units)) {
      expect(
        allowed.has(symbol),
        `${name} = ${JSON.stringify(symbol)} is not a framework.Unit* symbol`,
      ).toBe(true);
    }
  });
});
