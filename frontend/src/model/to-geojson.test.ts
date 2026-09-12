import { describe, expect, it } from "vitest";
import {
  featuresToGeoJSON,
  featuresToSourceGroups,
  modelToGeoJSON,
} from "./to-geojson";
import type { ModelFeature, ModelReceiver } from "./types";

const src: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  properties: { surface_type: "SMA", speed_pkw_kph: 60 },
  geometry: { type: "Point", coordinates: [10, 51] },
};
const bld: ModelFeature = {
  id: "b1",
  kind: "building",
  heightM: 10,
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
};
const bar: ModelFeature = {
  id: "br1",
  kind: "barrier",
  heightM: 3,
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [1, 1],
    ],
  },
};

const rcv: ModelReceiver = {
  id: "r1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10.5, 51.5] },
};

describe("featuresToGeoJSON", () => {
  it("produces a valid FeatureCollection", () => {
    const fc = featuresToGeoJSON([src, bld]);
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features).toHaveLength(2);
    expect(fc.features[0]?.properties["kind"]).toBe("source");
    expect(fc.features[0]?.properties["source_type"]).toBe("point");
    expect(fc.features[0]?.properties["surface_type"]).toBe("SMA");
    expect(fc.features[1]?.properties["kind"]).toBe("building");
    expect(fc.features[1]?.properties["height_m"]).toBe(10);
  });

  it("omits source_type for non-source features", () => {
    const fc = featuresToGeoJSON([bld]);
    expect(fc.features[0]?.properties["source_type"]).toBeUndefined();
  });

  it("omits height_m for source features", () => {
    const fc = featuresToGeoJSON([src]);
    expect(fc.features[0]?.properties["height_m"]).toBeUndefined();
  });
});

describe("featuresToSourceGroups", () => {
  it("groups features by kind for map sources", () => {
    const groups = featuresToSourceGroups([src, bld, bar]);
    expect(groups.sources.features).toHaveLength(1);
    expect(groups.buildings.features).toHaveLength(1);
    expect(groups.barriers.features).toHaveLength(1);
  });

  it("returns empty collections for missing kinds", () => {
    const groups = featuresToSourceGroups([src]);
    expect(groups.buildings.features).toHaveLength(0);
    expect(groups.barriers.features).toHaveLength(0);
  });
});

describe("modelToGeoJSON", () => {
  it("puts features first and receivers after them in one collection", () => {
    const fc = modelToGeoJSON({ features: [src, bld], receivers: [rcv] });
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features.map((f) => f.id)).toEqual(["s1", "b1", "r1"]);
    expect(fc.features[2]?.properties).toEqual({
      kind: "receiver",
      height_m: 4,
    });
  });

  it("produces an empty collection for an empty model", () => {
    expect(modelToGeoJSON({ features: [], receivers: [] }).features).toEqual(
      [],
    );
  });

  it("keeps the feature properties the map payload carries", () => {
    const fc = modelToGeoJSON({ features: [src], receivers: [] });
    expect(fc.features[0]?.properties["source_type"]).toBe("point");
    expect(fc.features[0]?.properties["surface_type"]).toBe("SMA");
  });
});
