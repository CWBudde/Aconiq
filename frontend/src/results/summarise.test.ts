import { describe, expect, it } from "vitest";
import { summariseIndicators } from "./summarise";
import type { SummarisableRecord } from "./summarise";

function record(values: Record<string, number>): SummarisableRecord {
  return { values };
}

describe("summariseIndicators", () => {
  it("counts min, max and mean per indicator", () => {
    const records = [
      record({ Lden: 62.4, Lnight: 55.1 }),
      record({ Lden: 58.2, Lnight: 51.9 }),
      record({ Lden: 71.8, Lnight: 60.4 }),
    ];

    expect(summariseIndicators(records, ["Lden", "Lnight"])).toEqual([
      { ind: "Lden", min: 58.2, max: 71.8, mean: (62.4 + 58.2 + 71.8) / 3 },
      { ind: "Lnight", min: 51.9, max: 60.4, mean: (55.1 + 51.9 + 60.4) / 3 },
    ]);
  });

  it("keeps the indicators in the order they were given", () => {
    const records = [record({ a: 1, b: 2 })];

    expect(summariseIndicators(records, ["b", "a"]).map((s) => s.ind)).toEqual([
      "b",
      "a",
    ]);
  });

  it("reads a missing indicator as 0 rather than skipping the record", () => {
    // The same `?? 0` the table cells and the sort comparator apply. A record
    // with no `Lnight` pulls the minimum to 0 and the mean down with it — the
    // gap is visible in the card, not averaged away.
    const records = [
      record({ Lden: 62.4, Lnight: 55.1 }),
      record({ Lden: 71.8 }),
    ];

    expect(summariseIndicators(records, ["Lnight"])).toEqual([
      { ind: "Lnight", min: 0, max: 55.1, mean: 55.1 / 2 },
    ]);
  });

  it("gives an empty table a zeroed card per indicator", () => {
    expect(summariseIndicators([], ["Lden", "Lnight"])).toEqual([
      { ind: "Lden", min: 0, max: 0, mean: 0 },
      { ind: "Lnight", min: 0, max: 0, mean: 0 },
    ]);
  });

  it("summarises no indicators as no cards", () => {
    expect(summariseIndicators([record({ Lden: 1 })], [])).toEqual([]);
  });

  it("handles negative values without the accumulators leaking their seeds", () => {
    // The seeds are ±Infinity; a table whose values are all negative is where
    // a seed of 0 would silently become the maximum.
    const records = [record({ d: -12 }), record({ d: -3 })];

    expect(summariseIndicators(records, ["d"])).toEqual([
      { ind: "d", min: -12, max: -3, mean: -7.5 },
    ]);
  });

  it("summarises 200 000 records", () => {
    /*
     * The size is the point. `Math.min(...vals)`, which this function replaced,
     * throws `RangeError: Maximum call stack size exceeded` at this length —
     * verified both in a bare node process and under this runner. The exact
     * ceiling is not a constant: a spread call takes one stack slot per
     * argument, so it depends on how much stack the caller has already used.
     * A bare node process survives 125 000; inside vitest that already throws.
     * 200 000 throws everywhere it was tried.
     *
     * The auto receiver grid is capped at 250 000 receivers, so a table this
     * size is a table the app can produce, and before this function existed it
     * took the whole receivers tab down with it.
     */
    const count = 200_000;
    const records: SummarisableRecord[] = Array.from(
      { length: count },
      (_, i) => record({ Lden: i, Lnight: count - i }),
    );

    expect(summariseIndicators(records, ["Lden", "Lnight"])).toEqual([
      { ind: "Lden", min: 0, max: count - 1, mean: (count - 1) / 2 },
      { ind: "Lnight", min: 1, max: count, mean: (count + 1) / 2 },
    ]);
  });
});
