import { describe, expect, it } from "vitest";
import {
  CALC_AREA_FEATURE_ID,
  featuresToGeoJSON,
  featuresToSourceGroups,
  modelToGeoJSON,
} from "./to-geojson";
import type { CalcArea, ModelFeature, ModelReceiver } from "./types";

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

const area: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [10.1, 51],
        [10.1, 51.1],
        [10, 51.1],
        [10, 51],
      ],
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
    const fc = modelToGeoJSON({
      features: [src, bld],
      receivers: [rcv],
      calcArea: null,
    });
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features.map((f) => f.id)).toEqual(["s1", "b1", "r1"]);
    expect(fc.features[2]?.properties).toEqual({
      kind: "receiver",
      height_m: 4,
    });
  });

  it("produces an empty collection for an empty model", () => {
    expect(
      modelToGeoJSON({ features: [], receivers: [], calcArea: null }).features,
    ).toEqual([]);
  });

  it("keeps the feature properties the map payload carries", () => {
    const fc = modelToGeoJSON({
      features: [src],
      receivers: [],
      calcArea: null,
    });
    expect(fc.features[0]?.properties["source_type"]).toBe("point");
    expect(fc.features[0]?.properties["surface_type"]).toBe("SMA");
  });

  it("emits the calculation area last, after features and receivers", () => {
    // Last is load-bearing: it keeps the prefix of a payload written before
    // the area existed byte-identical, so adding one appends rather than
    // rewrites.
    const fc = modelToGeoJSON({
      features: [src, bld],
      receivers: [rcv],
      calcArea: area,
    });
    expect(fc.features.map((f) => f.id)).toEqual([
      "s1",
      "b1",
      "r1",
      CALC_AREA_FEATURE_ID,
    ]);
    const withoutArea = modelToGeoJSON({
      features: [src, bld],
      receivers: [rcv],
      calcArea: null,
    });
    expect(JSON.stringify(fc.features.slice(0, 3))).toBe(
      JSON.stringify(withoutArea.features),
    );
  });

  it("gives the area the fixed id, the calc-area kind and no height", () => {
    const fc = modelToGeoJSON({
      features: [],
      receivers: [],
      calcArea: area,
    });
    const emitted = fc.features[0];
    expect(emitted?.id).toBe(CALC_AREA_FEATURE_ID);
    // No `height_m`: the backend applies no height rule to this kind, so one
    // would be a property nothing reads.
    expect(emitted?.properties).toEqual({ kind: "calc-area" });
    expect(emitted?.geometry.type).toBe("Polygon");
    expect(emitted?.geometry.coordinates).toEqual(area.geometry.coordinates);
  });

  it("serialises one model to identical JSON twice", () => {
    // The saved file's bytes are the model's identity — the API answers a
    // save with their hash — so a generated id anywhere in the payload would
    // make unchanged content look changed.
    const payload = { features: [src, bld], receivers: [rcv], calcArea: area };
    expect(JSON.stringify(modelToGeoJSON(payload))).toBe(
      JSON.stringify(modelToGeoJSON(payload)),
    );
  });
});
