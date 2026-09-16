// The map's display projection against the same PROJ reference vectors the Go
// tree checks its own transverse-Mercator series with.
//
// A fake kernel cannot catch a swapped `source_crs`/`target_crs` — it answers
// whatever it was told to answer, in whichever direction — and that swap is the
// classic defect in a change that adds a second, inverse use of a transform
// that until now only ever ran forwards. So this one runs the real WASM kernel,
// pushes the EPSG:25832 eastings and northings through `projectWorkspace` the
// way `useDisplayModel` does, and compares against PROJ's lon/lat.
//
// Skipped, not failed, when the kernel is not built — `kernelSkipReason()`
// turns that into a failure under ACONIQ_REQUIRE_WASM, which CI sets.

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "@/wasm/kernel-node";
import { projectWorkspace } from "@/model/compute-crs";
import type { ModelFeature } from "@/model/types";
import { DISPLAY_CRS } from "./display-model";

const REFERENCE_VECTORS = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/geo/testdata/proj-reference-vectors.json",
);

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

describe.skipIf(skipReason !== null)("display projection vs. PROJ", () => {
  it("turns EPSG:25832 eastings and northings back into lon/lat", async () => {
    const kernel = await getNodeKernel();
    const cases = metricCases();
    expect(cases.length).toBeGreaterThan(0);

    const features: ModelFeature[] = cases.map((entry, index) => ({
      id: `case-${String(index)}`,
      kind: "source",
      sourceType: "point",
      geometry: { type: "Point", coordinates: [entry.easting, entry.northing] },
    }));

    const projected = await projectWorkspace(
      (req) => kernel.transform(req),
      { features, receivers: [], calcArea: null, crs: "EPSG:25832" },
      DISPLAY_CRS,
    );

    expect(projected.projection.computeCRS).toBe(DISPLAY_CRS);
    expect(projected.projection.applied).toBe(true);

    for (const [index, entry] of cases.entries()) {
      const coordinates = projected.features[index]?.geometry.coordinates as
        | [number, number]
        | undefined;
      if (!coordinates) throw new Error(`case ${entry.name} was not projected`);

      // ~1e-7° is about a centimetre of latitude — tight enough that a series
      // truncated one term early shows up, loose enough that PROJ's own
      // rounding in the fixture does not.
      expect(coordinates[0]).toBeCloseTo(entry.lon, 7);
      expect(coordinates[1]).toBeCloseTo(entry.lat, 7);
    }
  });

  it("would fail if source and target were swapped", async () => {
    // The assertion above passes for *some* transform in either direction only
    // if the kernel ignores the CRS pair. It does not: asking for the forward
    // direction over the same input lands in metres, nowhere near a degree.
    const kernel = await getNodeKernel();
    const first = metricCases()[0];
    if (!first) throw new Error("no EPSG:25832 reference case");

    const projected = await projectWorkspace(
      (req) => kernel.transform(req),
      {
        features: [
          {
            id: "swapped",
            kind: "source",
            sourceType: "point",
            geometry: { type: "Point", coordinates: [first.lon, first.lat] },
          },
        ],
        receivers: [],
        calcArea: null,
        crs: DISPLAY_CRS,
      },
      "EPSG:25832",
    );

    const coordinates = projected.features[0]?.geometry.coordinates as
      | [number, number]
      | undefined;
    if (!coordinates) throw new Error("the forward case was not projected");
    // Sub-millimetre, in metres. Tighter than that measures the fixture's own
    // rounding rather than the series: PROJ's northing is quoted to 1e-9 m.
    expect(coordinates[0]).toBeCloseTo(first.easting, 3);
    expect(coordinates[1]).toBeCloseTo(first.northing, 3);
  });
});

if (skipReason !== null) {
  describe("display projection vs. PROJ", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Nothing to run.
    });
  });
}
