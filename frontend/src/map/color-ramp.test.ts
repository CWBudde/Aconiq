import { describe, expect, it } from "vitest";
import {
  NOISE_LEVEL_RAMP,
  rampToExpression,
  type ColorStop,
} from "./color-ramp";

/**
 * The ramp has no importers yet — "Results on the map" is what will read it —
 * so nothing else would notice it drifting out of shape before the day it is
 * wired up. What is pinned here is what MapLibre and the reader each require
 * of it, not the particular hex codes, which are a design choice.
 */

describe("NOISE_LEVEL_RAMP", () => {
  it("rises strictly, because an interpolate expression rejects anything else", () => {
    // MapLibre's `interpolate` requires its stop inputs in ascending order and
    // throws on the style, not on the offending stop. A stop pasted into the
    // wrong row would surface as an unstyled map with a console error.
    const values = NOISE_LEVEL_RAMP.map((stop) => stop.value);
    const ascending = [...values].sort((a, b) => a - b);
    expect(values).toEqual(ascending);
    expect(new Set(values).size).toBe(values.length);
  });

  it("steps in 5 dB, which is the bucket width the labels promise", () => {
    // The interior labels are bare numbers ("55"), so they only read correctly
    // if each one stands for the 5 dB above it. A 2 dB or 10 dB step would
    // leave the legend saying something the colors do not.
    const steps = NOISE_LEVEL_RAMP.slice(1).map(
      (stop, i) => stop.value - (NOISE_LEVEL_RAMP[i]?.value ?? Number.NaN),
    );
    expect(steps).toEqual(steps.map(() => 5));
  });

  it("labels every interior stop with its own value", () => {
    // A stop added without its label, or renumbered without it, is otherwise
    // invisible: the map still draws, and only the legend lies.
    for (const stop of NOISE_LEVEL_RAMP.slice(1, -1)) {
      expect(stop.label).toBe(String(stop.value));
    }
  });

  it("marks only the two end buckets as open-ended", () => {
    // Below the first stop and above the last, the color no longer stands for
    // a 5 dB band but for everything beyond it, and the legend has to say so.
    const first = NOISE_LEVEL_RAMP.at(0);
    const last = NOISE_LEVEL_RAMP.at(-1);
    expect(first?.label).toBe("< 40");
    expect(last?.label).toBe("> 75");
  });

  it("gives every stop a full six-digit hex color", () => {
    // These go straight into a paint property. MapLibre parses a bad color to
    // transparent black rather than failing, so a typo reads as a hole in the
    // map.
    for (const stop of NOISE_LEVEL_RAMP) {
      expect(stop.color).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  it("spans the range environmental noise mapping is read over", () => {
    // Legend endpoints, not arbitrary: below 35 dB(A) nothing is assessed and
    // above 80 the top bucket absorbs it.
    expect(NOISE_LEVEL_RAMP.at(0)?.value).toBe(35);
    expect(NOISE_LEVEL_RAMP.at(-1)?.value).toBe(80);
  });
});

describe("rampToExpression", () => {
  it("flattens the ramp into value/color pairs after the expression head", () => {
    const ramp: ColorStop[] = [
      { value: 10, color: "#000000", label: "10" },
      { value: 20, color: "#ffffff", label: "20" },
    ];

    expect(rampToExpression(ramp, "level")).toEqual([
      "interpolate",
      ["linear"],
      ["get", "level"],
      10,
      "#000000",
      20,
      "#ffffff",
    ]);
  });

  it("reads the `value` property when none is named", () => {
    // The default is load-bearing: a caller that omits the argument is styling
    // a feature collection whose levels live under `value`.
    const [, , accessor] = rampToExpression([]);
    expect(accessor).toEqual(["get", "value"]);
  });

  it("invents no stops for an empty ramp", () => {
    // MapLibre rejects a stopless interpolate, which is the right failure: a
    // fabricated default would paint levels that were never computed.
    expect(rampToExpression([])).toEqual([
      "interpolate",
      ["linear"],
      ["get", "value"],
    ]);
  });

  it("keeps the standard ramp's stops paired and in order", () => {
    const expression = rampToExpression(NOISE_LEVEL_RAMP);
    const stops = expression.slice(3);

    expect(stops).toHaveLength(NOISE_LEVEL_RAMP.length * 2);
    for (const [i, stop] of NOISE_LEVEL_RAMP.entries()) {
      expect(stops[i * 2]).toBe(stop.value);
      expect(stops[i * 2 + 1]).toBe(stop.color);
    }
  });
});
