/**
 * The decibel gate, per channel.
 *
 * Two of these cases are regressions rather than features. `withLegacyUnits`
 * exists because runs persist — on disk and in IndexedDB — and nothing
 * rewrites them, so a container written before the unit was per channel still
 * arrives carrying a scalar `unit`. The frontend reads those artifacts as raw
 * JSON under a type assertion, which converts nothing, so without the
 * expansion every existing decibel run would lose its circles, its picker and
 * its raster the moment the app was upgraded — under a panel reporting that
 * the table holds no decibels, which is the opposite of true.
 */
import { describe, expect, it } from "vitest";
import {
  declaresLevels,
  levelIndicators,
  unitFor,
  withLegacyUnits,
} from "./result-units";

describe("declaresLevels", () => {
  it("accepts the spellings both targets write", () => {
    // The CLI writes "dB", browser mode "dB(A)"; the gate must not care.
    expect(declaresLevels("dB")).toBe(true);
    expect(declaresLevels("dB(A)")).toBe(true);
    expect(declaresLevels(" db ")).toBe(true);
  });

  it("refuses a count, and refuses saying nothing", () => {
    expect(declaresLevels("count")).toBe(false);
    expect(declaresLevels("")).toBe(false);
  });
});

describe("unitFor", () => {
  it("answers the empty string when the container names no unit", () => {
    // Not a thrown error and not a guessed "dB": a container that never said
    // is a real state, and the callers print the bare number or refuse the
    // ramp rather than labelling a count as decibels.
    expect(unitFor({ Lden: "dB" }, "Lden")).toBe("dB");
    expect(unitFor({ Lden: "dB" }, "Lnight")).toBe("");
    expect(unitFor(undefined, "Lden")).toBe("");
  });
});

describe("levelIndicators", () => {
  it("keeps the decibel indicators of a mixed table and drops the counts", () => {
    // `beb-exposure`'s shape. The whole run used to be refused over this.
    const order = ["Lden", "Lnight", "estimated_persons"];
    const units = {
      Lden: "dB",
      Lnight: "dB",
      estimated_persons: "count",
    };

    expect(levelIndicators(order, units)).toEqual(["Lden", "Lnight"]);
  });

  it("preserves the container's declared order, not the map's", () => {
    // The map's iteration order is not the container's, and the picker shows
    // this list.
    const order = ["Lnight", "Lden"];
    const units = { Lden: "dB", Lnight: "dB" };

    expect(levelIndicators(order, units)).toEqual(["Lnight", "Lden"]);
  });

  it("answers nothing when the units never arrived", () => {
    expect(levelIndicators(["Lden"], undefined)).toEqual([]);
  });
});

describe("withLegacyUnits", () => {
  it("expands a scalar unit across the channels the container declares", () => {
    // Typed as the container actually is on the wire — `units` absent but
    // declarable — so the assertion below reads the field rather than an
    // inferred shape that never had it.
    const legacy: {
      indicator_order: string[];
      unit: string;
      units?: Record<string, string>;
    } = {
      indicator_order: ["Lden", "Lnight"],
      unit: "dB(A)",
    };

    const expanded = withLegacyUnits(legacy, legacy.indicator_order);

    expect(expanded.units).toEqual({ Lden: "dB(A)", Lnight: "dB(A)" });
    // And the whole point: the gate now sees the indicators again.
    expect(levelIndicators(legacy.indicator_order, expanded.units)).toEqual([
      "Lden",
      "Lnight",
    ]);
  });

  it("returns a current container unchanged, by reference", () => {
    // Identity matters, not just equality: react-query hands back a stable
    // reference and callers key memos and effects on it. A fresh object per
    // render would re-run the raster's colourising every time.
    const current = {
      indicator_order: ["Lden"],
      units: { Lden: "dB" },
    };

    expect(withLegacyUnits(current, current.indicator_order)).toBe(current);
  });

  it("does not invent units when the container names no channels", () => {
    // A raster may declare no band names. There is nothing to hang a unit on,
    // and Go refuses units on such a sidecar — inventing a key here would
    // produce a container its own backend would reject.
    const legacy: { unit: string; units?: Record<string, string> } = {
      unit: "dB",
    };

    expect(withLegacyUnits(legacy, undefined).units).toBeUndefined();
  });

  it("leaves a container that says nothing alone", () => {
    const bare: {
      indicator_order: string[];
      units?: Record<string, string>;
    } = { indicator_order: ["Lden"] };

    expect(withLegacyUnits(bare, bare.indicator_order).units).toBeUndefined();
  });
});
