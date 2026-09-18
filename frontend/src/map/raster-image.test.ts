import { describe, expect, it } from "vitest";
import { NOISE_LEVEL_RAMP, rampToExpression } from "./color-ramp";
import { rampColorRGB, rasterToRGBA } from "./raster-image";

/** The sidecar's nodata in both targets. */
const NODATA = -9999;

/** `#rrggbb` for a parsed triple, so a failure reads as a colour. */
function hex({
  r,
  g,
  b,
}: {
  r: number | undefined;
  g: number | undefined;
  b: number | undefined;
}): string {
  return `#${[r, g, b].map((c) => (c ?? 0).toString(16).padStart(2, "0")).join("")}`;
}

/** The RGBA quadruple at (x, y) of a `width`-wide buffer. */
function pixel(
  rgba: Uint8ClampedArray,
  width: number,
  x: number,
  y: number,
): number[] {
  const at = (y * width + x) * 4;
  return [...rgba.slice(at, at + 4)];
}

describe("rampColorRGB", () => {
  it("returns each stop's own colour at its own value", () => {
    for (const stop of NOISE_LEVEL_RAMP) {
      expect(hex(rampColorRGB(NOISE_LEVEL_RAMP, stop.value))).toBe(stop.color);
    }
  });

  it("agrees with the expression MapLibre paints the receiver circles with", () => {
    // The circles and the raster sit on the same map under one legend. If these
    // two ever disagree, a cell and the receiver standing in it are different
    // colours at the same level, and nothing on screen says which is meant.
    const expression = rampToExpression(NOISE_LEVEL_RAMP) as [
      string,
      unknown,
      unknown,
      ...(number | string)[],
    ];
    const stops = expression.slice(3) as (number | string)[];

    for (let i = 0; i < stops.length; i += 2) {
      const value = stops[i] as number;
      const color = stops[i + 1] as string;
      expect(hex(rampColorRGB(NOISE_LEVEL_RAMP, value))).toBe(color);
    }
  });

  it("interpolates between two stops component by component", () => {
    // MapLibre's `interpolate` resolves a colour through `Color.interpolate`
    // with its default `rgb` space, which is a lerp of the unpremultiplied sRGB
    // channels. Halfway between #1a9641 and #69b764 is the channel mean.
    const mid = rampColorRGB(NOISE_LEVEL_RAMP, 37.5);
    expect(mid).toEqual({
      r: Math.round((0x1a + 0x69) / 2),
      g: Math.round((0x96 + 0xb7) / 2),
      b: Math.round((0x41 + 0x64) / 2),
    });
  });

  it("clamps to the end stops rather than extrapolating", () => {
    expect(hex(rampColorRGB(NOISE_LEVEL_RAMP, -20))).toBe(
      NOISE_LEVEL_RAMP[0]?.color,
    );
    expect(hex(rampColorRGB(NOISE_LEVEL_RAMP, 500))).toBe(
      NOISE_LEVEL_RAMP[NOISE_LEVEL_RAMP.length - 1]?.color,
    );
  });

  it("refuses an empty ramp and a colour it cannot parse", () => {
    expect(() => rampColorRGB([], 50)).toThrow(/at least one stop/);
    expect(() =>
      rampColorRGB([{ value: 0, color: "red", label: "red" }], 0),
    ).toThrow(/not a #rrggbb value/);
  });
});

describe("rasterToRGBA", () => {
  it("puts the southernmost row last, where the image's south edge is", () => {
    // Deliberately asymmetric in both axes: a square or vertically symmetric
    // fixture passes whether the flip happens or not, and a mirrored noise map
    // looks entirely plausible.
    const width = 3;
    const height = 2;
    // Raster order, row 0 south: south row is 40/45/50, north row is 65/70/75.
    const values = Float64Array.from([40, 45, 50, 65, 70, 75]);

    const rgba = rasterToRGBA({
      values,
      width,
      height,
      nodata: NODATA,
      ramp: NOISE_LEVEL_RAMP,
    });

    // Image row 0 is north, so it must hold the 65/70/75 row.
    const [nr, ng, nb] = pixel(rgba, width, 0, 0);
    expect(hex({ r: nr, g: ng, b: nb })).toBe(
      NOISE_LEVEL_RAMP.find((s) => s.value === 65)?.color,
    );
    // Image row 1 is south, so it must hold the 40/45/50 row.
    const [r, g, b] = pixel(rgba, width, 0, 1);
    expect(hex({ r, g, b })).toBe(
      NOISE_LEVEL_RAMP.find((s) => s.value === 40)?.color,
    );
    // The x axis is not flipped with it.
    const [r2, g2, b2] = pixel(rgba, width, 2, 1);
    expect(hex({ r: r2, g: g2, b: b2 })).toBe(
      NOISE_LEVEL_RAMP.find((s) => s.value === 50)?.color,
    );
  });

  it("makes an uncomputed cell transparent rather than the ramp's floor", () => {
    const rgba = rasterToRGBA({
      values: Float64Array.from([NODATA, 55]),
      width: 2,
      height: 1,
      nodata: NODATA,
      ramp: NOISE_LEVEL_RAMP,
    });

    expect(pixel(rgba, 2, 0, 0)).toEqual([0, 0, 0, 0]);
    expect(pixel(rgba, 2, 1, 0)[3]).toBe(255);
  });

  it("treats NaN and an infinity as nodata", () => {
    const rgba = rasterToRGBA({
      values: Float64Array.from([Number.NaN, Number.POSITIVE_INFINITY]),
      width: 2,
      height: 1,
      nodata: NODATA,
      ramp: NOISE_LEVEL_RAMP,
    });

    expect(pixel(rgba, 2, 0, 0)).toEqual([0, 0, 0, 0]);
    expect(pixel(rgba, 2, 1, 0)).toEqual([0, 0, 0, 0]);
  });

  it("emits four channels per cell", () => {
    const rgba = rasterToRGBA({
      values: Float64Array.from([50, 55, 60, 65, 70, 75]),
      width: 3,
      height: 2,
      nodata: NODATA,
      ramp: NOISE_LEVEL_RAMP,
    });

    expect(rgba.length).toBe(3 * 2 * 4);
  });

  it("refuses dimensions the band does not match", () => {
    const band = Float64Array.from([50, 55]);
    const base = { values: band, nodata: NODATA, ramp: NOISE_LEVEL_RAMP };

    expect(() => rasterToRGBA({ ...base, width: 3, height: 1 })).toThrow(
      /holds 2 values, expected 3/,
    );
    expect(() => rasterToRGBA({ ...base, width: 0, height: 1 })).toThrow(
      /width must be a positive integer/,
    );
    expect(() => rasterToRGBA({ ...base, width: 2, height: 1.5 })).toThrow(
      /height must be a positive integer/,
    );
  });
});
