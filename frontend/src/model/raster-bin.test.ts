/**
 * Unit cases for the raster binary codec.
 *
 * The byte-level agreement with Go lives in `raster-bin.parity.test.ts`, which
 * reads fixtures the Go tree writes. What is left here is the part Go reaches
 * through `NewRaster`'s validation and the browser has no gate in front of:
 * a shape that does not describe a grid, a band count that does not match, and
 * a non-finite level — plus, for the reader, the two ways a headerless payload
 * is read against the wrong shape without complaining.
 */

import { describe, expect, it } from "vitest";

import { buildRasterBinary, readRasterBand } from "./raster-bin";

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

describe("readRasterBand", () => {
  it("round-trips what buildRasterBinary wrote", () => {
    const buffer = buildRasterBinary(SHAPE, [[1, 2, 3, 4]]);

    expect(Array.from(readRasterBand(buffer, SHAPE, 0))).toEqual([1, 2, 3, 4]);
  });

  /*
   * The fixture is 3x2 over two bands, as the parity fixture is: a band offset
   * that forgot the band stride reads band 0 back for every band, and only a
   * grid whose cells-per-band differs from its band count catches it.
   */
  it("reads band 1's own cells, not band 0's", () => {
    const shape = { width: 3, height: 2, bands: 2 };
    const buffer = buildRasterBinary(shape, [
      [10, 11, 12, 13, 14, 15],
      [20, 21, 22, 23, 24, 25],
    ]);

    expect(Array.from(readRasterBand(buffer, shape, 0))).toEqual([
      10, 11, 12, 13, 14, 15,
    ]);
    expect(Array.from(readRasterBand(buffer, shape, 1))).toEqual([
      20, 21, 22, 23, 24, 25,
    ]);
  });

  /*
   * The payload is headerless, so its length is the only thing that says it
   * matches the sidecar — the check `LoadRaster` makes before it reads a cell.
   */
  it("refuses a buffer whose length is not the declared cell count", () => {
    expect(() => readRasterBand(new ArrayBuffer(8 * 3), SHAPE, 0)).toThrow(
      "expected 32",
    );
    expect(() => readRasterBand(new ArrayBuffer(8 * 5), SHAPE, 0)).toThrow(
      "expected 32",
    );
  });

  it("refuses a band index outside the declared band count", () => {
    const buffer = buildRasterBinary(SHAPE, [[1, 2, 3, 4]]);

    expect(() => readRasterBand(buffer, SHAPE, 1)).toThrow("out of range");
    expect(() => readRasterBand(buffer, SHAPE, -1)).toThrow("out of range");
    expect(() => readRasterBand(buffer, SHAPE, 0.5)).toThrow("out of range");
  });

  it("refuses the same shapes the builder refuses", () => {
    const buffer = new ArrayBuffer(0);

    expect(() => readRasterBand(buffer, { ...SHAPE, width: 0 }, 0)).toThrow(
      "width",
    );
    expect(() => readRasterBand(buffer, { ...SHAPE, height: -1 }, 0)).toThrow(
      "height",
    );
    expect(() => readRasterBand(buffer, { ...SHAPE, bands: 1.5 }, 0)).toThrow(
      "bands",
    );
  });
});
