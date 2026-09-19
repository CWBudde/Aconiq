/**
 * A ground zone is a polygon carrying a `ground_factor`, from which ISO 9613-2
 * resolves G for the source, middle and receiver regions of each path instead
 * of reading one global number three times.
 *
 * It is the first feature kind the frontend gained since the model reader was
 * unified, so the three things a kind has to survive are pinned here together:
 * normalization keeps it and its property, the validator applies its own rule
 * to it, and `to-geojson` puts it in a source of its own so the map can draw
 * it as ground rather than as an obstacle.
 */

import { describe, expect, it } from "vitest";
import { normalizeModelGeoJSON } from "./normalize";
import { featuresToSourceGroups } from "./to-geojson";
import { validateProjectModel } from "./validate";
import type { GeoJSONFeature, ModelFeature, Position } from "./types";

const ring: Position[] = [
  [0, 0],
  [10, 0],
  [10, 10],
  [0, 10],
  [0, 0],
];

function zoneFeature(properties: Record<string, unknown>): GeoJSONFeature {
  return {
    type: "Feature",
    properties: { id: "z1", kind: "ground-zone", ...properties },
    geometry: { type: "Polygon", coordinates: [ring] },
  } as GeoJSONFeature;
}

function zone(properties?: Record<string, unknown>): ModelFeature {
  return {
    id: "z1",
    kind: "ground-zone",
    ...(properties === undefined ? {} : { properties }),
    geometry: { type: "Polygon", coordinates: [ring] },
  };
}

describe("ground zones in the model reader", () => {
  it("keeps a ground zone and its factor", () => {
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [zoneFeature({ ground_factor: 1 })],
    });

    expect(result.skipped).toEqual([]);
    expect(result.features).toHaveLength(1);
    expect(result.features[0]?.kind).toBe("ground-zone");
    expect(result.features[0]?.properties?.["ground_factor"]).toBe(1);
  });

  it("draws a ground zone from its own source", () => {
    const groups = featuresToSourceGroups([zone({ ground_factor: 0.7 })]);

    expect(groups.groundZones.features).toHaveLength(1);
    expect(groups.buildings.features).toHaveLength(0);
    expect(groups.barriers.features).toHaveLength(0);
    expect(groups.sources.features).toHaveLength(0);
  });
});

describe("ground zone validation", () => {
  it("accepts a polygon with a factor in range", () => {
    const report = validateProjectModel([zone({ ground_factor: 0.5 })], []);

    expect(report.valid).toBe(true);
  });

  it("requires the factor rather than defaulting it", () => {
    const report = validateProjectModel([zone()], []);

    expect(report.errors.map((issue) => issue.code)).toContain(
      "groundzone.factor.required",
    );
  });

  it.each([1.5, -0.1, Number.NaN])("refuses the factor %s", (factor) => {
    const report = validateProjectModel([zone({ ground_factor: factor })], []);

    expect(report.errors.map((issue) => issue.code)).toContain(
      "groundzone.factor.invalid",
    );
  });

  it("refuses a geometry that is not a Polygon", () => {
    const line: ModelFeature = {
      id: "z1",
      kind: "ground-zone",
      properties: { ground_factor: 0.5 },
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      },
    };

    const report = validateProjectModel([line], []);

    expect(report.errors.map((issue) => issue.code)).toContain(
      "groundzone.geometry.invalid",
    );
  });
});
