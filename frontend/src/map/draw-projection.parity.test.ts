// The draw path's projection against the same PROJ reference vectors the Go
// tree checks its own transverse-Mercator series with.
//
// `display-projection.parity.test.ts` does this for the direction the map reads
// in — store CRS → WGS 84. This file does it for the one direction the map
// *writes* in, WGS 84 → store CRS, which is the single deliberate exception to
// "a projection of the model, never a source for it". A fake kernel cannot
// catch a swapped `source_crs`/`target_crs` — it answers whatever it was told
// to answer, in whichever direction — and that swap is exactly the defect an
// inverse use of a forward-only transform invites. So this one runs the real
// WASM kernel.
//
// The round trip is the assertion the invariant actually needs: a vertex drawn
// on the map, carried back into a metric store and projected out again for
// display, must land on the pixel it was drawn at.
//
// Skipped, not failed, when the kernel is not built — `kernelSkipReason()`
// turns that into a failure under ACONIQ_REQUIRE_WASM, which CI sets.

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "@/wasm/kernel-node";
import { projectGeometry, projectWorkspace } from "@/model/compute-crs";
import type { Geometry, ModelFeature, Position } from "@/model/types";
import { DISPLAY_CRS } from "./display-model";

const REFERENCE_VECTORS = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/geo/testdata/proj-reference-vectors.json",
);

const STORE_CRS = "EPSG:25832";

interface ProjectionCase {
  name: string;
  epsg: number;
  lon: number;
  lat: number;
  easting: number;
  northing: number;
}

function metricCases(): ProjectionCase[] {
  const parsed = JSON.parse(readFileSync(REFERENCE_VECTORS, "utf8")) as {
    projection_cases: ProjectionCase[];
  };
  return parsed.projection_cases.filter((entry) => entry.epsg === 25832);
}

const skipReason = kernelSkipReason();

describe.skipIf(skipReason !== null)("draw projection vs. PROJ", () => {
  it("turns a shape drawn in lon/lat into EPSG:25832 eastings and northings", async () => {
    const kernel = await getNodeKernel();
    const cases = metricCases();
    expect(cases.length).toBeGreaterThan(0);

    // One LineString through every reference point, which is what a drawn
    // barrier is, and which exercises the nested coordinate tree rather than a
    // bare pair.
    const drawn: Geometry = {
      type: "LineString",
      coordinates: cases.map((entry): Position => [entry.lon, entry.lat]),
    };

    const projected = await projectGeometry(
      (req) => kernel.transform(req),
      drawn,
      DISPLAY_CRS,
      STORE_CRS,
    );

    expect(projected.projection.computeCRS).toBe(STORE_CRS);
    expect(projected.projection.applied).toBe(true);
    // The argument is never mutated: the store keeps whatever it already had,
    // and terra-draw's own feature is left as it was handed over.
    expect(projected.geometry).not.toBe(drawn);
    expect(drawn.coordinates).toEqual(
      cases.map((entry) => [entry.lon, entry.lat]),
    );

    const coordinates = projected.geometry.coordinates as [number, number][];
    for (const [index, entry] of cases.entries()) {
      const position = coordinates[index];
      if (!position) throw new Error(`case ${entry.name} was not projected`);
      // Sub-millimetre, in metres. Tighter than that measures the fixture's own
      // rounding rather than the series: PROJ's northing is quoted to 1e-9 m.
      expect(position[0]).toBeCloseTo(entry.easting, 3);
      expect(position[1]).toBeCloseTo(entry.northing, 3);
    }
  });

  it("would fail if source and target were swapped", async () => {
    // The assertion above passes for *some* transform in either direction only
    // if the kernel ignores the CRS pair. It does not: asking for the inverse
    // over the same input lands in degrees, nowhere near a metre.
    const kernel = await getNodeKernel();
    const first = metricCases()[0];
    if (!first) throw new Error("no EPSG:25832 reference case");

    const projected = await projectGeometry(
      (req) => kernel.transform(req),
      {
        type: "Point",
        coordinates: [first.easting, first.northing],
      } as Geometry,
      STORE_CRS,
      DISPLAY_CRS,
    );

    const position = projected.geometry.coordinates as [number, number];
    expect(position[0]).toBeCloseTo(first.lon, 7);
    expect(position[1]).toBeCloseTo(first.lat, 7);
  });

  it("does not move a vertex on a round trip through the map", async () => {
    // The invariant in one test. The map draws a metric model by projecting it
    // into WGS 84 (`useDisplayModel`), the user draws on top of that picture,
    // and the shape travels back through the inverse. If the two directions
    // disagree by more than the series' own precision, every edit drags the
    // model a little further from where it was drawn.
    const kernel = await getNodeKernel();
    const cases = metricCases();

    const features: ModelFeature[] = cases.map((entry, index) => ({
      id: `case-${String(index)}`,
      kind: "source",
      sourceType: "point",
      geometry: { type: "Point", coordinates: [entry.easting, entry.northing] },
    }));

    const displayed = await projectWorkspace(
      (req) => kernel.transform(req),
      { features, receivers: [], calcArea: null, crs: STORE_CRS },
      DISPLAY_CRS,
    );

    for (const [index, entry] of cases.entries()) {
      const onScreen = displayed.features[index]?.geometry;
      if (!onScreen) throw new Error(`case ${entry.name} was not displayed`);

      const back = await projectGeometry(
        (req) => kernel.transform(req),
        onScreen,
        DISPLAY_CRS,
        STORE_CRS,
      );

      const position = back.geometry.coordinates as [number, number];
      // A tenth of a millimetre, which is finer than any coordinate this
      // project stores and far finer than a pixel at any usable map scale.
      expect(position[0]).toBeCloseTo(entry.easting, 4);
      expect(position[1]).toBeCloseTo(entry.northing, 4);
    }
  });
});

if (skipReason !== null) {
  describe("draw projection vs. PROJ", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Nothing to run.
    });
  });
}
