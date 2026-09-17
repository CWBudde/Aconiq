import { describe, expect, it } from "vitest";
import { validateProjectModel } from "./validate";
import type { ModelFeature, ModelReceiver } from "./types";

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
    expect(issue?.message).toContain("2 parts");
    expect(issue?.message).toContain("Teilfläche");
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
