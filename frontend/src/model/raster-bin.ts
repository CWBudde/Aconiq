/**
 * The raster binary builder — the browser half of a byte contract.
 *
 * `results.SaveRaster` in `backend/internal/report/results/raster_io.go` is
 * canonical. It writes a headerless little-endian `float64` array, tagged
 * `float64-le-v1` in the sidecar, and a browser-mode run must produce the same
 * bytes for the same grid. The contract is written out in
 * `docs/result-containers-v1.md`, section "Raster binary — byte contract", and
 * pinned from both sides by
 * `backend/internal/report/results/testdata/raster-parity/`.
 *
 * The part a mirror gets wrong is the **index order**, not the encoding. A
 * raster is band-major and row-major within a band, while a run produces one
 * value per receiver in receiver order. Writing receivers in arrival order
 * yields a file of exactly the right length, full of finite values, with every
 * cell in the wrong place — which is why the parity fixture is 3x2 over two
 * bands rather than a square single-band grid.
 *
 * Deliberately import-free, like `receiver-csv.ts`: `model/` must not grow a
 * dependency on `api/client` for the sake of three field names.
 */

/** Structural mirror of the part of `RasterMetadata` the layout depends on. */
export interface RasterBinaryShape {
  width: number;
  height: number;
  bands: number;
}

/** The byte width of one cell. Go writes `math.Float64bits` per value. */
const BYTES_PER_CELL = 8;

/**
 * Go's `binary.LittleEndian`. Every platform this runs on is little-endian, but
 * `DataView` is explicit about it and `Float64Array` is not, so the file is the
 * same on a big-endian host rather than silently byte-swapped.
 */
const LITTLE_ENDIAN = true;

/**
 * Builds the `.bin` payload from per-band values in **receiver** order.
 *
 * `bands[b][i]` is band `b`'s value at receiver index `i`, where the receiver
 * index runs row-major from the south-west corner — the order both
 * `geo.GridReceiverSet.Generate` and `buildReceiverGrid` emit. The output is in
 * raster order, `(band * height + y) * width + x`.
 *
 * Non-finite values are refused rather than written. `SaveRaster` refuses them
 * too (`raster_io.go`), so a browser run that wrote one would produce a file
 * the CLI could not have produced and `LoadRaster` would read back as a level.
 */
export function buildRasterBinary(
  shape: RasterBinaryShape,
  bands: readonly (readonly number[])[],
): ArrayBuffer {
  const { width, height, bands: bandCount } = shape;

  if (!Number.isInteger(width) || width <= 0) {
    throw new Error(
      `raster width must be a positive integer, got ${String(width)}`,
    );
  }

  if (!Number.isInteger(height) || height <= 0) {
    throw new Error(
      `raster height must be a positive integer, got ${String(height)}`,
    );
  }

  if (!Number.isInteger(bandCount) || bandCount <= 0) {
    throw new Error(
      `raster bands must be a positive integer, got ${String(bandCount)}`,
    );
  }

  if (bands.length !== bandCount) {
    throw new Error(
      `expected ${String(bandCount)} bands of values, got ${String(bands.length)}`,
    );
  }

  const cellsPerBand = width * height;
  const buffer = new ArrayBuffer(cellsPerBand * bandCount * BYTES_PER_CELL);
  const view = new DataView(buffer);

  for (let band = 0; band < bandCount; band++) {
    const values = bands[band];

    if (values === undefined || values.length !== cellsPerBand) {
      throw new Error(
        `band ${String(band)} carries ${String(values?.length ?? 0)} values, expected ${String(cellsPerBand)} for a ${String(width)}x${String(height)} grid`,
      );
    }

    for (let index = 0; index < cellsPerBand; index++) {
      const value = values[index] as number;

      if (!Number.isFinite(value)) {
        throw new Error(
          `raster value at band ${String(band)} index ${String(index)} is not finite`,
        );
      }

      // The receiver index is already row-major from the south-west corner,
      // and so is the raster's within-band order, so this is a straight copy
      // into the band's slice. Spelling it out rather than concatenating the
      // bands keeps the one thing that can go wrong visible.
      const x = index % width;
      const y = Math.floor(index / width);
      const cell = (band * height + y) * width + x;

      view.setFloat64(cell * BYTES_PER_CELL, value, LITTLE_ENDIAN);
    }
  }

  return buffer;
}
