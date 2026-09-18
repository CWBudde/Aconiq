/**
 * The raster binary codec — the browser half of a byte contract.
 *
 * `results.SaveRaster` / `results.LoadRaster` in
 * `backend/internal/report/results/raster_io.go` are canonical. They write and
 * read a headerless little-endian `float64` array, tagged `float64-le-v1` in
 * the sidecar, and a browser-mode run must produce the same bytes for the same
 * grid — and read back what the CLI wrote. The contract is written out in
 * `docs/result-containers-v1.md`, section "Raster binary — byte contract", and
 * pinned from both sides by
 * `backend/internal/report/results/testdata/raster-parity/`.
 *
 * Both halves live here on purpose. The writer used to be the only one, and a
 * one-way mirror is where the two directions drift: a reader written beside
 * its caller picks up that caller's idea of the layout rather than this one's.
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
 * The shape checks both halves make, in one place so they cannot drift apart:
 * a buffer the reader accepts must be one the writer could have produced.
 *
 * Returns the cells per band, which both callers need next.
 */
function checkedCellsPerBand(shape: RasterBinaryShape): number {
  const { width, height, bands } = shape;

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

  if (!Number.isInteger(bands) || bands <= 0) {
    throw new Error(
      `raster bands must be a positive integer, got ${String(bands)}`,
    );
  }

  return width * height;
}

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
  const cellsPerBand = checkedCellsPerBand(shape);

  if (bands.length !== bandCount) {
    throw new Error(
      `expected ${String(bandCount)} bands of values, got ${String(bands.length)}`,
    );
  }

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

/**
 * One band of a raster binary, in raster order: `(band * height + y) * width + x`.
 *
 * The band comes back **unflipped** — row 0 is still the southernmost, as
 * `RowOrderSouthUp` declares and as every value in the file was written. The
 * flip into canvas order (row 0 northernmost) is a display concern and belongs
 * with whatever draws the raster, under `map/`. Doing it here would mean the
 * bytes this module reads and the bytes it writes no longer describe the same
 * grid, and the round trip through `buildRasterBinary` would mirror the map.
 */
export function readRasterBand(
  buffer: ArrayBuffer,
  shape: RasterBinaryShape,
  band: number,
): Float64Array {
  const { bands: bandCount } = shape;
  const cellsPerBand = checkedCellsPerBand(shape);

  const expectedBytes = cellsPerBand * bandCount * BYTES_PER_CELL;

  // The length check `LoadRaster` makes. The payload is headerless, so its
  // length is the only thing that says the file matches the sidecar: a
  // truncated or over-long buffer read against the declared shape silently
  // reports cells from the wrong band, or past the end.
  if (buffer.byteLength !== expectedBytes) {
    throw new Error(
      `raster binary is ${String(buffer.byteLength)} bytes, expected ${String(expectedBytes)} for a ${String(shape.width)}x${String(shape.height)} grid over ${String(bandCount)} band(s)`,
    );
  }

  if (!Number.isInteger(band) || band < 0 || band >= bandCount) {
    throw new Error(
      `band index ${String(band)} is out of range for ${String(bandCount)} band(s)`,
    );
  }

  const view = new DataView(buffer);
  const values = new Float64Array(cellsPerBand);
  const start = band * cellsPerBand;

  // Read cell by cell through the DataView rather than wrapping the buffer in
  // a `Float64Array`: a typed-array view decodes in *host* byte order, which
  // would byte-swap every level on a big-endian host while the writer above
  // stays explicitly little-endian. That asymmetry is exactly what the
  // LITTLE_ENDIAN constant exists to prevent, and it cannot be tested here.
  for (let index = 0; index < cellsPerBand; index++) {
    values[index] = view.getFloat64(
      (start + index) * BYTES_PER_CELL,
      LITTLE_ENDIAN,
    );
  }

  return values;
}
