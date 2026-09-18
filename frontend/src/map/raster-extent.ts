/**
 * Where a result raster's cells sit on the ground — the centre→corner
 * conversion, mirrored from Go.
 *
 * `results.Georeference` records the **centre** of cell (0,0), because that is
 * what the engine produces: a grid receiver is a point in the middle of the
 * cell it stands for. Every GIS format, and MapLibre's `image` source with it,
 * wants the **corner** of the top-left pixel. Go's comment on `Georeference`
 * says that conversion happens "once, in `report/export`, and nowhere else".
 *
 * This is therefore a second implementation by necessity, as `raster-bin.ts`
 * and `receiver-csv.ts` are: browser mode has no Go to call for it, and the
 * alternative is each call site doing the half-pixel arithmetic inline, which
 * is how the two conventions get mixed in the first place. Because it is a
 * mirror rather than a caller, it is pinned by a Go golden —
 * `backend/internal/report/export/testdata/raster-parity/geotransform.golden.json`,
 * written by `formats_parity_test.go` and read back by
 * `raster-extent.parity.test.ts`.
 *
 * It lives under `map/` and not under `model/` for two reasons:
 *
 *   - it is about placing a raster on a map, which is what `display-model.ts`
 *     and `extent.ts` beside it are about, not about the model's own contents;
 *   - `raster-bin.ts` is deliberately import-free, and this wants
 *     `RasterGeoreference` from `@/api/client` rather than a fourth structural
 *     copy of four field names.
 */

import type { RasterGeoreference } from "@/api/client";

/**
 * The only row order this project writes. Go's `results.RowOrderSouthUp`:
 * row 0 holds the southernmost cells.
 */
const ROW_ORDER_SOUTH_UP = "south-up";

/** Go's `exportfmt.GeoTransform`: the top-left corner of the top-left pixel. */
export interface RasterGeoTransform {
  originX: number;
  originY: number;
  /** Positive, east. */
  pixelSizeX: number;
  /** Negative, south. */
  pixelSizeY: number;
}

/**
 * Mirrors `exportfmt.GeoTransformFromGeoreference`. Throws what Go refuses.
 *
 * Half a pixel west, and — because row 0 is the southernmost — the full height
 * plus half a pixel north.
 *
 * It throws rather than returning a union on purpose. This is a programming
 * contract, not user input: a georeference reaches it straight off a run's
 * sidecar, and every value it refuses is a bug upstream rather than something
 * a caller can recover from. A guessed row order in particular would place the
 * grid upside down in silence — a vertically mirrored noise map that looks
 * entirely plausible — which is the one failure of a mirror nobody notices.
 */
export function geoTransformFromGeoreference(
  georef: RasterGeoreference,
  gridHeight: number,
): RasterGeoTransform {
  // The same four checks, in the same order, as `Georeference.Validate`.
  if (!Number.isFinite(georef.origin_x)) {
    throw new Error("georeference origin_x must be finite");
  }

  if (!Number.isFinite(georef.origin_y)) {
    throw new Error("georeference origin_y must be finite");
  }

  if (!Number.isFinite(georef.pixel_size_m) || georef.pixel_size_m <= 0) {
    throw new Error("georeference pixel_size_m must be finite and > 0");
  }

  if (georef.row_order !== ROW_ORDER_SOUTH_UP) {
    throw new Error(
      `georeference row_order "${georef.row_order}" is not supported`,
    );
  }

  if (!Number.isInteger(gridHeight) || gridHeight <= 0) {
    throw new Error("grid height must be positive");
  }

  return {
    originX: georef.origin_x - georef.pixel_size_m / 2,
    originY:
      georef.origin_y +
      (gridHeight - 1) * georef.pixel_size_m +
      georef.pixel_size_m / 2,
    pixelSizeX: georef.pixel_size_m,
    pixelSizeY: -georef.pixel_size_m,
  };
}

/** The raster's outer edges, in the raster's own CRS. */
export interface RasterCornerExtent {
  west: number;
  east: number;
  south: number;
  north: number;
}

/**
 * The raster's outer edges — the corners of the outermost cells, not the
 * centres of the outermost receivers. A grid of `width`x`height` cells covers
 * `width * pixel` by `height * pixel`, which is one whole pixel more than the
 * span between the receivers at its edges.
 */
export function rasterCornerExtent(
  georef: RasterGeoreference,
  width: number,
  height: number,
): RasterCornerExtent {
  if (!Number.isInteger(width) || width <= 0) {
    throw new Error("grid width must be positive");
  }

  // Validates the georeference and the height, and puts north and west in
  // corner terms. Deriving the other two from it rather than from the
  // georeference keeps one centre→corner conversion in this file, not two.
  const transform = geoTransformFromGeoreference(georef, height);

  return {
    west: transform.originX,
    east: transform.originX + width * transform.pixelSizeX,
    // pixelSizeY is negative, so this walks south.
    south: transform.originY + height * transform.pixelSizeY,
    north: transform.originY,
  };
}

/**
 * The corner order a MapLibre `image` source takes: top-left, top-right,
 * bottom-right, bottom-left.
 *
 * MapLibre reads `coordinates` in that order and does not check it; a
 * different winding drapes the image mirrored or rotated over the right
 * extent, so the order is the contract and is asserted rather than described.
 */
/**
 * Exactly four corners. A tuple and not an array, because MapLibre's
 * `Coordinates` is a 4-tuple and a caller that assembled three would otherwise
 * find out at runtime, from an image draped over the wrong quad.
 */
export type RasterCorners = [
  [number, number],
  [number, number],
  [number, number],
  [number, number],
];

export function cornerCoordinates(extent: RasterCornerExtent): RasterCorners {
  return [
    [extent.west, extent.north],
    [extent.east, extent.north],
    [extent.east, extent.south],
    [extent.west, extent.south],
  ];
}
