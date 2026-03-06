import { describe, expect, it } from "vitest";
import { normalizeGeoJSON } from "./normalize";
import type { GeoJSONFeatureCollection } from "./types";

const validCollection: GeoJSONFeatureCollection = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { kind: "source", source_type: "point" },
      geometry: { type: "Point", coordinates: [10, 51] },
    },
    {
      type: "Feature",
      properties: { kind: "building", height_m: 12 },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    },
    {
      type: "Feature",
      properties: { kind: "barrier", height_m: 3.5 },
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      },
    },
  ],
};

describe("normalizeGeoJSON", () => {
  it("normalizes a valid FeatureCollection", () => {
    const result = normalizeGeoJSON(validCollection);
    expect(result.features).toHaveLength(3);
    expect(result.features[0]?.kind).toBe("source");
    expect(result.features[0]?.sourceType).toBe("point");
    expect(result.features[1]?.kind).toBe("building");
    expect(result.features[1]?.heightM).toBe(12);
    expect(result.features[2]?.kind).toBe("barrier");
  });

  it("assigns unique IDs to features without IDs", () => {
    const result = normalizeGeoJSON(validCollection);
    const ids = result.features.map((f) => f.id);
    expect(new Set(ids).size).toBe(3);
  });

  it("preserves existing feature IDs", () => {
    const collection: GeoJSONFeatureCollection = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          id: "my-id",
          properties: { kind: "source", source_type: "point" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
    };
    const result = normalizeGeoJSON(collection);
    expect(result.features[0]?.id).toBe("my-id");
  });

  it("returns empty array for empty collection", () => {
    const result = normalizeGeoJSON({
      type: "FeatureCollection",
      features: [],
    });
    expect(result.features).toEqual([]);
  });

  it("skips features with unknown kind", () => {
    const collection: GeoJSONFeatureCollection = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { kind: "unknown" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
    };
    const result = normalizeGeoJSON(collection);
    expect(result.features).toEqual([]);
    expect(result.skipped).toHaveLength(1);
  });
});
