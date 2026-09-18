// The contour entry point across the real WASM boundary.
//
// Go pins everything below `wasmkernel.Contours`, and a host test greps
// cmd/wasm/main.go to prove the name is registered — but nothing on the Go side
// can call the JavaScript export, because that file is `//go:build js && wasm`.
// What is left unpinned is exactly the part this file drives: the raster values
// crossing as a Uint8Array *beside* the request JSON rather than inside it, in
// that argument order. Getting it wrong is a runtime TypeError in the browser
// and nothing at all in CI.

import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "./kernel-node";
import type { ContourRequest, RasterMetadata } from "./types";

const skipReason = kernelSkipReason();

const CRS = "EPSG:25832";

const metadata: RasterMetadata = {
  width: 8,
  height: 8,
  bands: 2,
  nodata: -999,
  unit: "dB(A)",
  band_names: ["lr_day", "lr_night"],
  crs: CRS,
  georeference: {
    origin_x: 681_000,
    origin_y: 5_646_000,
    pixel_size_m: 10,
    row_order: "south-up",
  },
};

/**
 * The fixture's values in the layout a run writes: band-major, row-major within
 * a band, little-endian float64. This is the one place in the frontend that
 * encodes that contract, and it is a test fixture on purpose — production code
 * hands the bytes straight from IndexedDB to the kernel and lets Go decode
 * them, which is why a second reading of the layout can only ever appear here.
 *
 * The values slope across both axes so that every 5 dB level in range crosses
 * the grid, and the second band sits 10 dB lower so the two are told apart.
 */
function payload(): Uint8Array {
  const cells = metadata.width * metadata.height * metadata.bands;
  const buffer = new ArrayBuffer(cells * 8);
  const view = new DataView(buffer);

  let index = 0;
  for (let band = 0; band < metadata.bands; band += 1) {
    for (let y = 0; y < metadata.height; y += 1) {
      for (let x = 0; x < metadata.width; x += 1) {
        view.setFloat64(index * 8, 40 + x + y - 10 * band, true);
        index += 1;
      }
    }
  }

  return new Uint8Array(buffer);
}

function request(overrides: Partial<ContourRequest> = {}): ContourRequest {
  return { raster: metadata, target_crs: CRS, ...overrides };
}

describe.skipIf(skipReason !== null)("kernel.contours", () => {
  it("traces every band and echoes the CRS and interval", async () => {
    const kernel = await getNodeKernel();
    const result = await kernel.contours(payload(), request());

    expect(result.crs).toBe(CRS);
    // Omitted in the request, so the EU END default comes back stated.
    expect(result.interval).toBe(5);

    const bands = new Set(result.lines.map((line) => line.band_name));
    expect([...bands].sort()).toEqual(["lr_day", "lr_night"]);

    // Target equal to the raster's CRS means no reprojection, so the vertices
    // are still the metres the georeference places the grid on. Degrees here
    // would mean the reprojection ran when it should not have.
    for (const line of result.lines) {
      for (const [x, y] of line.points) {
        expect(x).toBeGreaterThan(680_000);
        expect(y).toBeGreaterThan(5_645_000);
      }
    }
  });

  it("reprojects into the requested CRS", async () => {
    const kernel = await getNodeKernel();
    const result = await kernel.contours(
      payload(),
      request({ target_crs: "EPSG:4326", interval: 10 }),
    );

    expect(result.crs).toBe("EPSG:4326");
    expect(result.interval).toBe(10);
    expect(result.lines.length).toBeGreaterThan(0);

    for (const line of result.lines) {
      for (const [lon, lat] of line.points) {
        expect(lon).toBeGreaterThan(11);
        expect(lon).toBeLessThan(12);
        expect(lat).toBeGreaterThan(50);
        expect(lat).toBeLessThan(52);
      }
    }
  });

  // The payload carries no header, so its length is the only thing that says it
  // matches the sidecar it arrived with. The refusal must reach the browser as
  // a rejected promise carrying Go's own words, not as a resolved empty result.
  it("rejects a payload that does not match the declared shape", async () => {
    const kernel = await getNodeKernel();
    const truncated = payload().slice(0, -8);

    await expect(kernel.contours(truncated, request())).rejects.toThrow(
      /raster binary size mismatch/,
    );
  });
});
