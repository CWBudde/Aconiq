import { describe, expect, it } from "vitest";
import { validateProjectModel } from "./validate";
import type { ModelFeature, ModelReceiver } from "./types";
import { SELF_INTERSECTION_POINT_LIMIT } from "./self-intersection";

const validSource: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const validBuilding: ModelFeature = {
  id: "bld-1",
  kind: "building",
  heightM: 12,
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

const validBarrier: ModelFeature = {
  id: "bar-1",
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

const validReceiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [2, 3] },
};

describe("validateProjectModel", () => {
  it("valid model returns valid=true with no errors", () => {
    const report = validateProjectModel(
      [validSource, validBuilding, validBarrier],
      [],
    );
    expect(report.valid).toBe(true);
    expect(report.errors).toHaveLength(0);
  });

  it("empty model produces an error", () => {
    const report = validateProjectModel([], []);
    expect(report.valid).toBe(false);
    expect(report.errors[0]?.code).toBe("model.empty");
  });

  it("source without source_type produces error", () => {
    const bad: ModelFeature = {
      id: "s1",
      kind: "source",
      geometry: { type: "Point", coordinates: [0, 0] },
    };
    const report = validateProjectModel([bad], []);
    expect(report.errors.some((e) => e.code === "source.type.required")).toBe(
      true,
    );
  });

  it("source with wrong geometry produces error", () => {
    const bad: ModelFeature = {
      id: "s1",
      kind: "source",
      sourceType: "point",
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
    const report = validateProjectModel([bad], []);
    expect(
      report.errors.some((e) => e.code === "source.geometry.mismatch"),
    ).toBe(true);
  });

  it("building without height produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
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
    const report = validateProjectModel([bad], []);
    expect(
      report.errors.some((e) => e.code === "building.height.required"),
    ).toBe(true);
  });

  it("building with negative height produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
      heightM: -5,
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
    const report = validateProjectModel([bad], []);
    expect(
      report.errors.some((e) => e.code === "building.height.invalid"),
    ).toBe(true);
  });

  it("barrier without height produces error", () => {
    const bad: ModelFeature = {
      id: "br1",
      kind: "barrier",
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      },
    };
    const report = validateProjectModel([bad], []);
    expect(
      report.errors.some((e) => e.code === "barrier.height.required"),
    ).toBe(true);
  });

  it("building with non-polygon geometry produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
      heightM: 10,
      geometry: { type: "Point", coordinates: [0, 0] },
    };
    const report = validateProjectModel([bad], []);
    expect(
      report.errors.some((e) => e.code === "building.geometry.invalid"),
    ).toBe(true);
  });

  it("duplicate IDs produce error", () => {
    const a = { ...validSource };
    const b = { ...validBuilding, id: "src-1" };
    const report = validateProjectModel([a, b], []);
    expect(
      report.errors.some(
        (e) =>
          e.code === "feature.id.duplicate" ||
          e.code === "receiver.id.duplicate",
      ),
    ).toBe(true);
  });

  it("receiver validation accepts finite coordinates and positive height", () => {
    const report = validateProjectModel([validSource], [validReceiver]);
    expect(report.valid).toBe(true);
  });

  it("receiver validation rejects invalid height", () => {
    const report = validateProjectModel(
      [validSource],
      [{ ...validReceiver, heightM: 0 }],
    );
    expect(
      report.errors.some((e) => e.code === "receiver.height.invalid"),
    ).toBe(true);
  });

  it("receiver validation rejects invalid coordinates", () => {
    const report = validateProjectModel(
      [validSource],
      [
        {
          ...validReceiver,
          geometry: { type: "Point", coordinates: [Number.NaN, 3] },
        },
      ],
    );
    expect(
      report.errors.some((e) => e.code === "receiver.coordinates.invalid"),
    ).toBe(true);
  });

  it("receiver ids must not collide with feature ids", () => {
    const report = validateProjectModel(
      [validSource],
      [{ ...validReceiver, id: validSource.id }],
    );
    expect(report.errors.some((e) => e.featureId === validSource.id)).toBe(
      true,
    );
  });

  it("warns when imported source acoustics require review", () => {
    const report = validateProjectModel(
      [
        {
          ...validSource,
          sourceType: "line",
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [1, 0],
            ],
          },
          properties: { source_acoustics_review_required: true },
        },
      ],
      [],
    );

    expect(
      report.warnings.some(
        (issue) => issue.code === "source.rls19.review_required",
      ),
    ).toBe(true);
  });

  it("stops warning once the reader has signed the acoustics off", () => {
    // The import's own flag stays: it records what OSM gave us, and a run
    // stamps it into provenance. What retires the finding is the second
    // property, the reader's sign-off — otherwise this is a warning no action
    // in the app can clear, and an OSM district import raises 608 of them.
    const report = validateProjectModel(
      [
        {
          ...validSource,
          sourceType: "line",
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [1, 0],
            ],
          },
          properties: {
            source_acoustics_review_required: true,
            source_acoustics_reviewed: true,
          },
        },
      ],
      [],
    );

    expect(
      report.warnings.some(
        (issue) => issue.code === "source.rls19.review_required",
      ),
    ).toBe(false);
  });

  it("rejects invalid RLS-19 source override values", () => {
    const report = validateProjectModel(
      [
        {
          ...validSource,
          sourceType: "line",
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [1, 0],
            ],
          },
          properties: {
            speed_pkw_kph: 0,
            traffic_day_pkw: -1,
            surface_type: "bogus",
          },
        },
      ],
      [],
    );

    expect(
      report.errors.some(
        (issue) => issue.code === "source.rls19.speed.invalid",
      ),
    ).toBe(true);
    expect(
      report.errors.some(
        (issue) => issue.code === "source.rls19.traffic.invalid",
      ),
    ).toBe(true);
    expect(
      report.errors.some(
        (issue) => issue.code === "source.rls19.surface_type.invalid",
      ),
    ).toBe(true);
  });
});

describe("RLS-19 Parkplatz validation", () => {
  const areaFeature = (properties: Record<string, unknown>): ModelFeature => ({
    id: "lot",
    kind: "source",
    sourceType: "area",
    properties,
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [0, 0],
          [10, 0],
          [10, 10],
          [0, 10],
          [0, 0],
        ],
      ],
    },
  });

  // This validator runs with no standard selected, and an area source is a
  // legitimate input to cnossos-industry and bub-industry too. Claiming every
  // one of them is a Parkplatz would bury those models in unrelated errors.
  it("leaves an area source carrying no parking property alone", () => {
    const report = validateProjectModel([areaFeature({})], []);

    expect(
      report.errors.filter((issue) =>
        issue.code.startsWith("source.rls19.parking."),
      ),
    ).toEqual([]);
  });

  it("refuses a half-filled Parkplatz, which is the realistic mistake", () => {
    const report = validateProjectModel(
      [areaFeature({ rls19_parking_num_spaces: 200 })],
      [],
    );

    const codes = report.errors.map((issue) => issue.code);
    expect(codes).toContain("source.rls19.parking.parking_type.missing");
    expect(codes).toContain("source.rls19.parking.movements.missing");
  });

  it("accepts a Tabelle 7 facility type in place of explicit rates", () => {
    const report = validateProjectModel(
      [
        areaFeature({
          rls19_parking_num_spaces: 200,
          rls19_parking_type: "lkw-omnibus",
          rls19_parking_facility_type: "tank-rastanlage",
        }),
      ],
      [],
    );

    expect(
      report.errors.filter((issue) =>
        issue.code.startsWith("source.rls19.parking."),
      ),
    ).toEqual([]);
  });

  it("keeps an explicitly stated zero movement rate legal", () => {
    const report = validateProjectModel(
      [
        areaFeature({
          rls19_parking_num_spaces: 200,
          rls19_parking_type: "pkw",
          rls19_parking_movements_per_space_day: 0.3,
          rls19_parking_movements_per_space_night: 0,
        }),
      ],
      [],
    );

    expect(
      report.errors.filter((issue) =>
        issue.code.startsWith("source.rls19.parking."),
      ),
    ).toEqual([]);
  });

  const square = (offset: number): [number, number][][] => [
    [
      [offset, 0],
      [offset + 10, 0],
      [offset + 10, 10],
      [offset, 10],
      [offset, 0],
    ],
  ];

  const complete = {
    rls19_parking_num_spaces: 200,
    rls19_parking_type: "pkw",
    rls19_parking_movements_per_space_day: 0.3,
    rls19_parking_movements_per_space_night: 0,
  };

  // Every property is filled in, so nothing else here has anything to say — and
  // until this rule existed that was the whole finding: a lot a user could
  // complete on the map, save, and have refused only when a run read it.
  it("refuses a multi-part Parkplatz the extractors cannot read", () => {
    const feature = areaFeature(complete);
    feature.geometry = {
      type: "MultiPolygon",
      coordinates: [square(0), square(50)],
    };

    const report = validateProjectModel([feature], []);

    const issue = report.errors.find(
      (candidate) =>
        candidate.code === "source.rls19.parking.geometry.multipart",
    );
    // The part count is the finding's payload and is asserted here; that it
    // reaches a sentence naming the Teilfläche is `validation-message.test.ts`'s
    // business, because the sentence is now per locale and this file is not.
    expect(issue?.params).toEqual({
      parts: 2,
      field: "rls19_parking_num_spaces",
    });
  });

  it("accepts a MultiPolygon carrying exactly one part, as the extractors do", () => {
    const feature = areaFeature(complete);
    feature.geometry = { type: "MultiPolygon", coordinates: [square(0)] };

    const report = validateProjectModel([feature], []);

    expect(
      report.errors.filter((issue) =>
        issue.code.startsWith("source.rls19.parking."),
      ),
    ).toEqual([]);
  });
});

/**
 * The gap that let an OSM import be shown as clean and then refused.
 *
 * `validate.ts` had no geometry checks at all, so every
 * `geometry.*.self_intersection` the backend produces was invisible here — 45
 * of them across the 2397 ways of a Berlin extract, every one of which the
 * preview called clean and the save then had an opinion about.
 *
 * The severity split is the backend's, not a second opinion — a ring's winding
 * is read by point-in-polygon and the screening crossing counts, a line's is
 * read by nothing.
 */
describe("self-intersection", () => {
  const bowTie: [number, number][] = [
    [0, 0],
    [10, 10],
    [10, 0],
    [0, 10],
    [0, 0],
  ];

  it("refuses a footprint whose ring crosses itself", () => {
    const report = validateProjectModel(
      [
        {
          id: "bld-x",
          kind: "building",
          heightM: 10,
          geometry: { type: "Polygon", coordinates: [bowTie] },
        },
      ],
      [],
    );

    expect(report.valid).toBe(false);
    expect(report.errors.map((issue) => issue.code)).toContain(
      "geometry.polygon.self_intersection",
    );
  });

  it("keeps a line that crosses itself, and says so", () => {
    const report = validateProjectModel(
      [
        {
          id: "src-x",
          kind: "source",
          sourceType: "line",
          geometry: {
            type: "LineString",
            coordinates: [
              [0, 0],
              [10, 10],
              [0, 10],
              [10, 0],
            ],
          },
        },
      ],
      [],
    );

    // A road drawn as one way that touches itself is a roundabout. Refusing it
    // made an OSM extract unsaveable over geometry it holds correctly.
    expect(report.valid).toBe(true);
    expect(report.warnings.map((issue) => issue.code)).toContain(
      "geometry.linestring.self_intersection",
    );
  });

  it("answers the same for one footprint in degrees and in metres", () => {
    // The check used to compare a cross product — an area, so the coordinate
    // unit squared — against a fixed tolerance, so the same building was
    // self-intersecting in EPSG:4326 and simple in EPSG:25832.
    const shape: [number, number][] = [
      [0, 0],
      [3, 0],
      [3, 1],
      [1, 1],
      [1, 3],
      [0, 3],
      [0, 0],
    ];
    const rotation = (31 * Math.PI) / 180;

    const ring = (
      unit: number,
      originX: number,
      originY: number,
    ): [number, number][] =>
      shape.map(([px, py]) => [
        originX + (px * Math.cos(rotation) - py * Math.sin(rotation)) * unit,
        originY + (px * Math.sin(rotation) + py * Math.cos(rotation)) * unit,
      ]);

    const inDegrees = ring(1e-5, 13.38, 52.51);
    const inMetres = ring(1e-5 * 111320, 390000, 5819000);

    for (const [name, coordinates] of [
      ["degrees", inDegrees],
      ["metres", inMetres],
    ] as const) {
      const report = validateProjectModel(
        [
          {
            id: `bld-${name}`,
            kind: "building",
            heightM: 10,
            geometry: { type: "Polygon", coordinates: [coordinates] },
          },
        ],
        [],
      );

      expect(report.errors, name).toEqual([]);
    }
  });

  it("reports the cost bound rather than walking a huge geometry", () => {
    // Above the limit the quadratic walk is not attempted. In a browser a
    // frozen tab is worse than an unchecked geometry, so it is reported.
    const coordinates: [number, number][] = [];
    for (let i = 0; i <= SELF_INTERSECTION_POINT_LIMIT; i++) {
      coordinates.push([i % 2, i]);
    }

    const report = validateProjectModel(
      [
        {
          id: "src-long",
          kind: "source",
          sourceType: "line",
          geometry: { type: "LineString", coordinates },
        },
      ],
      [],
    );

    expect(report.valid).toBe(true);
    expect(report.warnings.map((issue) => issue.code)).toContain(
      "geometry.linestring.self_intersection.skipped",
    );
  });
});
