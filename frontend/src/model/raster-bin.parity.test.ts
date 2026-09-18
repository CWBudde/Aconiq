/**
 * Browser-CLI parity for the raster binary, at the byte level.
 *
 * `results.SaveRaster` is canonical. The fixtures below belong to the Go tree —
 * `backend/internal/report/results/raster_parity_test.go` writes them, and
 * `just update-golden` regenerates them — so nothing here restates what the
 * bytes ought to be. It reads what the CLI actually produced and asserts the
 * browser builder produces the same thing.
 *
 * The fixture is 3 wide by 2 high over two bands, so a width/height swap and a
 * band/row swap both change the bytes. A square single-band raster would pass
 * either way.
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { buildRasterBinary } from "./raster-bin";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses.
const FIXTURE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/report/results/testdata/raster-parity",
);

interface ParityFixture {
  metadata: {
    width: number;
    height: number;
    bands: number;
    nodata: number;
    unit: string;
    band_names: string[];
    crs: string;
    georeference: {
      origin_x: number;
      origin_y: number;
      pixel_size_m: number;
      row_order: string;
    };
  };
  bands: number[][];
}

function readFixture(): ParityFixture {
  return JSON.parse(
    readFileSync(resolve(FIXTURE_DIR, "raster.golden.json"), "utf8"),
  ) as ParityFixture;
}

describe("raster binary parity with the Go writer", () => {
  it("reproduces the CLI's bytes for the parity grid", () => {
    const fixture = readFixture();
    const expected = readFileSync(resolve(FIXTURE_DIR, "raster.golden.bin"));

    const got = new Uint8Array(
      buildRasterBinary(fixture.metadata, fixture.bands),
    );

    expect(got.byteLength).toBe(expected.byteLength);
    expect(Array.from(got)).toEqual(Array.from(expected));
  });

  /*
   * The sidecar the browser writes is a metadata object rather than a file, so
   * the file's bookkeeping keys are not its business — but the keys that say
   * what the raster *is* are, and they are what a consumer reads on either
   * target. This pins the spelling of the ones browser mode writes.
   */
  it("agrees with the sidecar the CLI writes about the georeference", () => {
    const sidecar = JSON.parse(
      readFileSync(resolve(FIXTURE_DIR, "raster_sidecar.golden.json"), "utf8"),
    ) as Record<string, unknown>;
    const fixture = readFixture();

    expect(sidecar["width"]).toBe(fixture.metadata.width);
    expect(sidecar["height"]).toBe(fixture.metadata.height);
    expect(sidecar["bands"]).toBe(fixture.metadata.bands);
    expect(sidecar["crs"]).toBe(fixture.metadata.crs);
    expect(sidecar["georeference"]).toEqual(fixture.metadata.georeference);
    // The encoding tag is the contract's name; a change to it is a change to
    // what buildRasterBinary must produce.
    expect(sidecar["encoding"]).toBe("float64-le-v1");
    expect(sidecar["data_bytes"]).toBe(
      fixture.metadata.width *
        fixture.metadata.height *
        fixture.metadata.bands *
        8,
    );
  });
});
