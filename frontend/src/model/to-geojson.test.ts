import { describe, expect, it } from "vitest";
import { featuresToGeoJSON, featuresToSourceGroups } from "./to-geojson";
import type { ModelFeature } from "./types";

const src: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
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

describe("featuresToGeoJSON", () => {
  it("produces a valid FeatureCollection", () => {
    const fc = featuresToGeoJSON([src, bld]);
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features).toHaveLength(2);
    expect(fc.features[0]?.properties["kind"]).toBe("source");
    expect(fc.features[0]?.properties["source_type"]).toBe("point");
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
