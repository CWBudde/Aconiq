import { describe, expect, it } from "vitest";
import { normalizeModelGeoJSON } from "./normalize";
import { DEFAULT_RECEIVER_HEIGHT_M } from "./types";
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

/**
 * The one reader of a v1 FeatureCollection. Receivers and the calculation area
 * survive it; the feature-only reader that folded them into `skipped` is gone,
 * along with the workspaces it would have emptied.
 */
describe("normalizeModelGeoJSON", () => {
  const receiverFeature = {
    type: "Feature" as const,
    properties: { id: "r1", kind: "receiver", height_m: 4 },
    geometry: { type: "Point", coordinates: [10, 51] },
  };

  const areaFeature = {
    type: "Feature" as const,
    properties: { id: "calc-area", kind: "calc-area" },
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [10, 51],
          [10.1, 51],
          [10.1, 51.1],
          [10, 51],
        ],
      ],
    },
  };

  it("normalizes a valid FeatureCollection", () => {
    const result = normalizeModelGeoJSON(validCollection);
    expect(result.features).toHaveLength(3);
    expect(result.features[0]?.kind).toBe("source");
    expect(result.features[0]?.sourceType).toBe("point");
    expect(result.features[0]?.properties?.["kind"]).toBe("source");
    expect(result.features[1]?.kind).toBe("building");
    expect(result.features[1]?.heightM).toBe(12);
    expect(result.features[2]?.kind).toBe("barrier");
  });

  it("assigns unique IDs to features without IDs", () => {
    const result = normalizeModelGeoJSON(validCollection);
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
    const result = normalizeModelGeoJSON(collection);
    expect(result.features[0]?.id).toBe("my-id");
  });

  it("returns empty array for empty collection", () => {
    const result = normalizeModelGeoJSON({
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
    const result = normalizeModelGeoJSON(collection);
    expect(result.features).toEqual([]);
    expect(result.skipped).toHaveLength(1);
  });

  it("preserves standard-specific source properties", () => {
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          id: "road-1",
          properties: {
            kind: "source",
            source_type: "line",
            surface_type: "SMA",
            speed_pkw_kph: 70,
            traffic_day_pkw: 900,
          },
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [10, 0],
            ],
          },
        },
      ],
    });

    expect(result.features[0]?.properties?.["surface_type"]).toBe("SMA");
    expect(result.features[0]?.properties?.["speed_pkw_kph"]).toBe(70);
    expect(result.features[0]?.properties?.["traffic_day_pkw"]).toBe(900);
  });

  it("keeps receivers and the calculation area", () => {
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [...validCollection.features, receiverFeature, areaFeature],
    });

    expect(result.features).toHaveLength(3);
    expect(result.receivers).toHaveLength(1);
    expect(result.receivers[0]?.id).toBe("r1");
    expect(result.receivers[0]?.heightM).toBe(4);
    expect(result.calcArea?.geometry.coordinates).toEqual(
      areaFeature.geometry.coordinates,
    );
    expect(result.skipped).toEqual([]);
  });

  it("keeps the stored id of the calculation area, and mints none", () => {
    // Kept, so a save does not rename an area the project already named; and
    // never minted, so an area the user drew stays anonymous and picks up the
    // derived id at emit time instead of a fresh UUID on every hydration.
    const named = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        { ...areaFeature, properties: { id: "extent", kind: "calc-area" } },
      ],
    });
    expect(named.calcArea?.id).toBe("extent");

    const anonymous = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [{ ...areaFeature, properties: { kind: "calc-area" } }],
    });
    expect(anonymous.calcArea).not.toBeNull();
    expect(anonymous.calcArea?.id).toBeUndefined();
  });

  it("reads ids from properties.id, the way the backend writes them", () => {
    // `Model.ToFeatureCollection` puts the id in `properties.id` and leaves
    // the GeoJSON `id` member unset, and Go's `featureID` reads it back from
    // there. Reading only the member minted a fresh UUID for every hydrated
    // feature, and the next save rewrote every id in the project.
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { id: "road-7", kind: "source", source_type: "line" },
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [1, 1],
            ],
          },
        },
      ],
    });

    expect(result.features[0]?.id).toBe("road-7");
  });

  it("prefers properties.id over the GeoJSON id member", () => {
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          id: "member-id",
          properties: { id: "property-id", kind: "building", height_m: 3 },
          geometry: {
            type: "Polygon",
            coordinates: [
              [
                [0, 0],
                [1, 0],
                [1, 1],
                [0, 0],
              ],
            ],
          },
        },
      ],
    });

    expect(result.features[0]?.id).toBe("property-id");
  });

  it("defaults a receiver with no usable height instead of dropping it", () => {
    // Dropping is the loss this reader exists to prevent: a receiver someone
    // placed on the map is not something to discard over a missing property.
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { id: "r2", kind: "receiver" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
    });

    expect(result.receivers).toHaveLength(1);
    expect(result.receivers[0]?.heightM).toBe(DEFAULT_RECEIVER_HEIGHT_M);
  });

  it("refuses a receiver that is not a Point", () => {
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { id: "r3", kind: "receiver" },
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [1, 1],
            ],
          },
        },
      ],
    });

    expect(result.receivers).toEqual([]);
    expect(result.skipped).toHaveLength(1);
  });

  it("refuses a MultiPolygon calculation area", () => {
    // The backend refuses one rather than reducing it to its envelope: a
    // disjoint area would become a single bbox over ground the user drew
    // around. Accepting one here would draw an area no run could honour.
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { kind: "calc-area" },
          geometry: {
            type: "MultiPolygon",
            coordinates: [areaFeature.geometry.coordinates],
          },
        },
      ],
    });

    expect(result.calcArea).toBeNull();
    expect(result.skipped).toHaveLength(1);
  });

  it("keeps only the first calculation area", () => {
    const second = {
      ...areaFeature,
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [20, 41],
            [20.1, 41],
            [20.1, 41.1],
            [20, 41],
          ],
        ],
      },
    };
    const result = normalizeModelGeoJSON({
      type: "FeatureCollection",
      features: [areaFeature, second],
    });

    expect(result.calcArea?.geometry.coordinates).toEqual(
      areaFeature.geometry.coordinates,
    );
    expect(result.skipped).toHaveLength(1);
  });
});
