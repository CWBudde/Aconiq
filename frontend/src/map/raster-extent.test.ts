/**
 * Unit cases for the centre→corner conversion.
 *
 * The agreement with Go lives in `raster-extent.parity.test.ts`, which reads a
 * golden the Go tree writes. What is left here is the shape of the conversion
 * — which way each half-pixel step goes — and the refusals, which the golden
 * cannot carry because Go returns an error for them rather than a transform.
 */

import { describe, expect, it } from "vitest";

import type { RasterGeoreference } from "@/api/client";
import {
  cornerCoordinates,
  geoTransformFromGeoreference,
  rasterCornerExtent,
} from "./raster-extent";

const GEOREF: RasterGeoreference = {
  origin_x: 1000,
  origin_y: 2000,
  pixel_size_m: 10,
  row_order: "south-up",
};

describe("geoTransformFromGeoreference", () => {
  /*
   * The georeference names the centre of cell (0,0) and the transform names
   * the corner of the top-left pixel, so the two steps are in opposite
   * directions: half a pixel west, and the whole grid plus half a pixel north
   * because row 0 is the southernmost.
   */
  it("steps half a pixel west of the cell centre", () => {
    expect(geoTransformFromGeoreference(GEOREF, 3).originX).toBe(995);
  });

  it("steps (height-1) pixels plus half a pixel north", () => {
    // 2000 + 2*10 + 5.
    expect(geoTransformFromGeoreference(GEOREF, 3).originY).toBe(2025);
  });

  it("reports a pixel size that grows east and shrinks north", () => {
    const transform = geoTransformFromGeoreference(GEOREF, 3);

    expect(transform.pixelSizeX).toBe(10);
    expect(transform.pixelSizeY).toBe(-10);
  });

  /*
   * The case `TestGeoTransformFromGeoreferenceHandlesASingleRow` pins on the
   * Go side, value for value. Receiver-coordinate inference has no answer for
   * a one-row grid — it divides by n-1 — which is the case a declared
   * georeference exists to cover, and the case where an off-by-one in the
   * (height-1) term disappears.
   */
  it("places a single-row grid where Go's single-row test says", () => {
    const transform = geoTransformFromGeoreference(
      { origin_x: 100, origin_y: 200, pixel_size_m: 25, row_order: "south-up" },
      1,
    );

    expect(transform).toEqual({
      originX: 87.5,
      originY: 212.5,
      pixelSizeX: 25,
      pixelSizeY: -25,
    });
  });

  /*
   * A north-up raster read as south-up is a vertically mirrored noise map that
   * looks entirely plausible, so an unknown row order is refused rather than
   * guessed — the one failure of a mirror nobody notices.
   */
  it("refuses a row order it cannot read rather than guessing", () => {
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, row_order: "north-up" }, 3),
    ).toThrow("row_order");
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, row_order: "" }, 3),
    ).toThrow("row_order");
  });

  it("refuses a pixel size that cannot describe a cell", () => {
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, pixel_size_m: 0 }, 3),
    ).toThrow("pixel_size_m");
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, pixel_size_m: -10 }, 3),
    ).toThrow("pixel_size_m");
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, pixel_size_m: Number.NaN }, 3),
    ).toThrow("pixel_size_m");
    expect(() =>
      geoTransformFromGeoreference(
        { ...GEOREF, pixel_size_m: Number.POSITIVE_INFINITY },
        3,
      ),
    ).toThrow("pixel_size_m");
  });

  it("refuses a non-finite origin", () => {
    expect(() =>
      geoTransformFromGeoreference({ ...GEOREF, origin_x: Number.NaN }, 3),
    ).toThrow("origin_x");
    expect(() =>
      geoTransformFromGeoreference(
        { ...GEOREF, origin_y: Number.NEGATIVE_INFINITY },
        3,
      ),
    ).toThrow("origin_y");
  });

  it("refuses a grid height that is not a positive row count", () => {
    expect(() => geoTransformFromGeoreference(GEOREF, 0)).toThrow(
      "grid height",
    );
    expect(() => geoTransformFromGeoreference(GEOREF, -1)).toThrow(
      "grid height",
    );
    expect(() => geoTransformFromGeoreference(GEOREF, 2.5)).toThrow(
      "grid height",
    );
  });
});

describe("rasterCornerExtent", () => {
  /*
   * The extent is one whole pixel wider and taller than the span between the
   * outermost receivers: each edge cell reaches half a pixel beyond its own
   * centre.
   */
  it("reaches half a pixel beyond the outermost cell centres", () => {
    expect(rasterCornerExtent(GEOREF, 4, 3)).toEqual({
      west: 995,
      east: 1035,
      south: 1995,
      north: 2025,
    });
  });

  it("spans width*pixel by height*pixel", () => {
    const extent = rasterCornerExtent(GEOREF, 4, 3);

    expect(extent.east - extent.west).toBe(40);
    expect(extent.north - extent.south).toBe(30);
  });

  it("refuses the same georeference the transform refuses", () => {
    expect(() =>
      rasterCornerExtent({ ...GEOREF, row_order: "north-up" }, 4, 3),
    ).toThrow("row_order");
    expect(() => rasterCornerExtent(GEOREF, 4, 0)).toThrow("grid height");
    expect(() => rasterCornerExtent(GEOREF, 0, 3)).toThrow("grid width");
  });
});

describe("cornerCoordinates", () => {
  /*
   * MapLibre reads `coordinates` as TL/TR/BR/BL and does not check it; a
   * different winding drapes the image mirrored or rotated over the right
   * extent, so the order is asserted rather than described.
   */
  it("emits exactly four pairs, top-left, top-right, bottom-right, bottom-left", () => {
    const corners = cornerCoordinates(rasterCornerExtent(GEOREF, 4, 3));

    expect(corners).toEqual([
      [995, 2025],
      [1035, 2025],
      [1035, 1995],
      [995, 1995],
    ]);
  });
});
