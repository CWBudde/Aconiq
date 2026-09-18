/**
 * Unit cases for the raster binary builder.
 *
 * The byte-level agreement with Go lives in `raster-bin.parity.test.ts`, which
 * reads fixtures the Go tree writes. What is left here is the part Go reaches
 * through `NewRaster`'s validation and the browser has no gate in front of:
 * a shape that does not describe a grid, a band count that does not match, and
 * a non-finite level.
 */

import { describe, expect, it } from "vitest";

import { buildRasterBinary } from "./raster-bin";

const SHAPE = { width: 2, height: 2, bands: 1 };

function readCells(buffer: ArrayBuffer): number[] {
  const view = new DataView(buffer);
  const cells: number[] = [];
  for (let offset = 0; offset < buffer.byteLength; offset += 8) {
    cells.push(view.getFloat64(offset, true));
  }
  return cells;
}

describe("buildRasterBinary", () => {
  it("writes one little-endian float64 per cell per band", () => {
    const buffer = buildRasterBinary(SHAPE, [[1, 2, 3, 4]]);

    expect(buffer.byteLength).toBe(2 * 2 * 1 * 8);
    expect(readCells(buffer)).toEqual([1, 2, 3, 4]);
  });

  /*
   * Band-major: every cell of band 0, then every cell of band 1. A builder
   * that interleaves them produces a file of exactly the right length whose
   * every value is in the wrong place, which only an ordering assertion
   * catches.
   */
  it("lays bands out one after another, not interleaved", () => {
    const buffer = buildRasterBinary({ ...SHAPE, bands: 2 }, [
      [1, 2, 3, 4],
      [5, 6, 7, 8],
    ]);

    expect(readCells(buffer)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
  });

  it("refuses a shape that does not describe a grid", () => {
    expect(() => buildRasterBinary({ ...SHAPE, width: 0 }, [[]])).toThrow(
      "width",
    );
    expect(() => buildRasterBinary({ ...SHAPE, height: -1 }, [[]])).toThrow(
      "height",
    );
    expect(() => buildRasterBinary({ ...SHAPE, bands: 1.5 }, [[]])).toThrow(
      "bands",
    );
  });

  it("refuses a band count that does not match the values it was given", () => {
    expect(() =>
      buildRasterBinary({ ...SHAPE, bands: 2 }, [[1, 2, 3, 4]]),
    ).toThrow("expected 2 bands");
  });

  it("refuses a band whose length is not the cell count", () => {
    expect(() => buildRasterBinary(SHAPE, [[1, 2, 3]])).toThrow(
      "expected 4 for a 2x2 grid",
    );
  });

  /*
   * `SaveRaster` refuses these too. A browser run that wrote one would produce
   * a file the CLI could not have produced, and `LoadRaster` would read the
   * bit pattern back as a level.
   */
  it("refuses a non-finite level rather than writing its bit pattern", () => {
    expect(() => buildRasterBinary(SHAPE, [[1, 2, Number.NaN, 4]])).toThrow(
      "not finite",
    );
    expect(() =>
      buildRasterBinary(SHAPE, [[1, 2, Number.POSITIVE_INFINITY, 4]]),
    ).toThrow("not finite");
  });
});
