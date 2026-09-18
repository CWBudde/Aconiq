/**
 * Browser-CLI parity for the raster centre→corner conversion.
 *
 * `exportfmt.GeoTransformFromGeoreference` is canonical. The fixture below
 * belongs to the Go tree — `backend/internal/report/export/formats_parity_test.go`
 * writes it, and `just update-golden` regenerates it — so nothing here restates
 * what the numbers ought to be. It feeds the recorded inputs to the browser
 * mirror and asserts the recorded outputs come back.
 *
 * It sits in `export/` rather than beside the raster binary fixture in
 * `results/` because `export` imports `results`, and the Go test that writes
 * the binary fixture is in package `results` itself — generating this there
 * would close an import cycle.
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import type { RasterGeoreference } from "@/api/client";
import { geoTransformFromGeoreference } from "./raster-extent";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses.
const FIXTURE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/report/export/testdata/raster-parity",
);

interface GeoTransformParityCase {
  name: string;
  why: string;
  georeference: RasterGeoreference;
  grid_height: number;
  // Go spells the transform in snake_case in the golden, like every other JSON
  // in that tree; `RasterGeoTransform` is camelCase. The mapping is this file's
  // job and nothing else's.
  geo_transform: {
    origin_x: number;
    origin_y: number;
    pixel_size_x: number;
    pixel_size_y: number;
  };
}

function readCases(): GeoTransformParityCase[] {
  return JSON.parse(
    readFileSync(resolve(FIXTURE_DIR, "geotransform.golden.json"), "utf8"),
  ) as GeoTransformParityCase[];
}

describe("geo transform parity with the Go converter", () => {
  const cases = readCases();

  // A golden that silently shrank to nothing would pass every assertion below.
  it("reads a fixture with cases in it", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases)("reproduces the CLI's transform for $name", (testCase) => {
    const got = geoTransformFromGeoreference(
      testCase.georeference,
      testCase.grid_height,
    );

    // Exact equality, not toBeCloseTo: both sides apply the same operations in
    // the same order to the same float64 bits, and a tolerance would hide a
    // mirror that multiplies by 0.5 where Go divides by 2, or that reorders the
    // sum — which is the drift this file exists to catch.
    expect(got.originX).toBe(testCase.geo_transform.origin_x);
    expect(got.originY).toBe(testCase.geo_transform.origin_y);
    expect(got.pixelSizeX).toBe(testCase.geo_transform.pixel_size_x);
    expect(got.pixelSizeY).toBe(testCase.geo_transform.pixel_size_y);
  });
});
