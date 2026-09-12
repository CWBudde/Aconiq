import { describe, expect, it } from "vitest";
import {
  buildParkingSources,
  polygonArea,
  polygonCentroid,
} from "./rls19-parking";
import type { ModelFeature } from "./types";

function lot(properties: Record<string, unknown>): ModelFeature {
  return {
    id: "lot",
    kind: "source",
    sourceType: "area",
    properties,
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [0, 0],
          [50, 0],
          [50, 20],
          [0, 20],
          [0, 0],
        ],
      ],
    },
  };
}

const stated = {
  rls19_parking_num_spaces: 200,
  rls19_parking_type: "lkw-omnibus",
  rls19_parking_movements_per_space_day: 1.5,
  rls19_parking_movements_per_space_night: 0.8,
};

describe("polygon helpers", () => {
  it("computes the area of a unit square", () => {
    expect(
      polygonArea([
        [
          { x: 0, y: 0 },
          { x: 1, y: 0 },
          { x: 1, y: 1 },
          { x: 0, y: 1 },
          { x: 0, y: 0 },
        ],
      ]),
    ).toBeCloseTo(1, 12);
  });

  it("subtracts a hole and never returns a negative area", () => {
    const exterior = [
      { x: 0, y: 0 },
      { x: 10, y: 0 },
      { x: 10, y: 10 },
      { x: 0, y: 10 },
      { x: 0, y: 0 },
    ];
    const hole = [
      { x: 2, y: 2 },
      { x: 4, y: 2 },
      { x: 4, y: 4 },
      { x: 2, y: 4 },
      { x: 2, y: 2 },
    ];
    expect(polygonArea([exterior, hole])).toBeCloseTo(96, 12);
  });

  it("is insensitive to winding order", () => {
    const cw = [
      { x: 0, y: 0 },
      { x: 0, y: 1 },
      { x: 1, y: 1 },
      { x: 1, y: 0 },
      { x: 0, y: 0 },
    ];
    expect(polygonArea([cw])).toBeCloseTo(1, 12);
  });

  it("centres a rectangle on its middle", () => {
    expect(
      polygonCentroid([
        [
          { x: 0, y: 0 },
          { x: 4, y: 0 },
          { x: 4, y: 2 },
          { x: 0, y: 2 },
          { x: 0, y: 0 },
        ],
      ]),
    ).toEqual({ x: 2, y: 1 });
  });

  it("has no centroid for a ring enclosing no area", () => {
    expect(
      polygonCentroid([
        [
          { x: 0, y: 0 },
          { x: 1, y: 0 },
          { x: 2, y: 0 },
          { x: 0, y: 0 },
        ],
      ]),
    ).toBeNull();
  });
});

describe("buildParkingSources", () => {
  it("derives the centre and the area from the polygon", () => {
    const { sources, extent } = buildParkingSources([lot(stated)]);

    expect(sources).toHaveLength(1);
    expect(sources[0]?.center).toEqual({ x: 25, y: 10 });
    expect(sources[0]?.area_m2).toBeCloseTo(1000, 9);
    expect(sources[0]?.parking_type).toBe("lkw-omnibus");
    expect(sources[0]?.movements_per_space_day).toBe(1.5);

    // Every vertex, so the receiver grid can be padded around the footprint
    // rather than around a point inside it.
    expect(extent).toHaveLength(5);
  });

  it("seeds both rates from a Tabelle 7 Parkplatztyp", () => {
    const { sources } = buildParkingSources([
      lot({
        rls19_parking_num_spaces: 50,
        rls19_parking_type: "pkw",
        rls19_parking_facility_type: "park-and-ride",
      }),
    ]);

    expect(sources[0]?.movements_per_space_day).toBe(0.3);
    expect(sources[0]?.movements_per_space_night).toBe(0.06);
  });

  it("lets an explicit rate override its own period only", () => {
    const { sources } = buildParkingSources([
      lot({
        rls19_parking_num_spaces: 50,
        rls19_parking_type: "pkw",
        rls19_parking_facility_type: "tank-rastanlage",
        rls19_parking_movements_per_space_night: 0,
      }),
    ]);

    expect(sources[0]?.movements_per_space_day).toBe(1.5);
    expect(sources[0]?.movements_per_space_night).toBe(0);
  });

  it("keeps an explicit zero, which is a legal period with no movements", () => {
    const { sources } = buildParkingSources([
      lot({
        ...stated,
        rls19_parking_movements_per_space_night: 0,
      }),
    ]);

    expect(sources[0]?.movements_per_space_night).toBe(0);
  });

  it.each([
    ["rls19_parking_type", "rls19_parking_type"],
    ["rls19_parking_num_spaces", "rls19_parking_num_spaces"],
    [
      "rls19_parking_movements_per_space_day",
      "rls19_parking_movements_per_space_day",
    ],
  ])("refuses an omitted %s", (key, expected) => {
    const properties = Object.fromEntries(
      Object.entries(stated).filter(([name]) => name !== key),
    );

    expect(() => buildParkingSources([lot(properties)])).toThrow(expected);
  });

  it("refuses an unknown Parkplatztyp and lists the vocabulary", () => {
    expect(() =>
      buildParkingSources([lot({ ...stated, rls19_parking_type: "bus" })]),
    ).toThrow(/pkw, motorrad, lkw-omnibus/);
  });

  it("refuses a MultiPolygon, naming the Teilflächen rule", () => {
    const feature = lot(stated);
    feature.geometry = {
      type: "MultiPolygon",
      coordinates: [
        [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 0],
          ],
        ],
      ],
    };

    expect(() => buildParkingSources([feature])).toThrow(/single Polygon/);
  });

  it("falls back to a feature-index id and skips non-area features", () => {
    const feature = lot(stated);
    feature.id = "";

    const road: ModelFeature = {
      id: "road",
      kind: "source",
      sourceType: "line",
      properties: {},
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 0],
        ],
      },
    };

    const { sources } = buildParkingSources([road, feature]);

    expect(sources).toHaveLength(1);
    expect(sources[0]?.id).toBe("rls19-parking-001");
  });
});
