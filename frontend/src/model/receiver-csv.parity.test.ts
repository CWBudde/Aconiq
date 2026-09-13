/**
 * Browser-CLI parity for the receiver-table CSV, at the byte level.
 *
 * Go's `encoding/csv` is canonical. The fixtures below belong to the Go tree —
 * `backend/internal/report/results/receiver_table_csv_test.go` writes them, and
 * `just update-golden` regenerates them — so nothing here restates what the
 * bytes ought to be. It reads what the CLI actually produced and asserts the
 * browser builder produces the same thing.
 *
 * Two fixtures, for the two ways the spellings can diverge:
 *
 *   - receiver_table.golden.json / .csv pin the quoting rules over a table
 *     built to hit every clause of `csv.Writer.fieldNeedsQuotes`;
 *   - float_spelling.golden.json pins `strconv.FormatFloat(v, 'f', -1, 64)`,
 *     keyed on IEEE-754 bit patterns so each double is reconstructed exactly
 *     rather than routed through a JS decimal parser first.
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { buildReceiverTableCSV, formatCSVFloat } from "./receiver-csv";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses.
const FIXTURE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/report/results/testdata/csv-parity",
);

function readFixture(name: string): string {
  return readFileSync(resolve(FIXTURE_DIR, name), "utf8");
}

describe("receiver CSV parity with the Go writer", () => {
  it("reproduces the CLI's bytes for the adversarial table", () => {
    const table: unknown = JSON.parse(
      readFixture("receiver_table.golden.json"),
    );
    const expected = readFixture("receiver_table.golden.csv");

    const got = buildReceiverTableCSV(
      table as Parameters<typeof buildReceiverTableCSV>[0],
    );

    expect(got).toBe(expected);
  });

  it("spells every curated double the way strconv.FormatFloat does", () => {
    const parsed: unknown = JSON.parse(
      readFixture("float_spelling.golden.json"),
    );
    const spellings = parsed as { bits: string; want: string }[];

    expect(spellings.length).toBeGreaterThan(0);

    const view = new DataView(new ArrayBuffer(8));
    for (const { bits, want } of spellings) {
      view.setBigUint64(0, BigInt(`0x${bits}`));
      const value = view.getFloat64(0);

      expect(formatCSVFloat(value), `bits ${bits}`).toBe(want);
    }
  });
});
