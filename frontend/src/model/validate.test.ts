import { describe, expect, it } from "vitest";
import { validateModel, validateProjectModel } from "./validate";
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

describe("validateModel", () => {
  it("valid model returns valid=true with no errors", () => {
    const report = validateModel([validSource, validBuilding, validBarrier]);
    expect(report.valid).toBe(true);
    expect(report.errors).toHaveLength(0);
  });

  it("empty model produces an error", () => {
    const report = validateModel([]);
    expect(report.valid).toBe(false);
    expect(report.errors[0]?.code).toBe("model.empty");
  });

  it("source without source_type produces error", () => {
    const bad: ModelFeature = {
      id: "s1",
      kind: "source",
      geometry: { type: "Point", coordinates: [0, 0] },
    };
    const report = validateModel([bad]);
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
    const report = validateModel([bad]);
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
    const report = validateModel([bad]);
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
    const report = validateModel([bad]);
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
    const report = validateModel([bad]);
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
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "building.geometry.invalid"),
    ).toBe(true);
  });

  it("duplicate IDs produce error", () => {
    const a = { ...validSource };
    const b = { ...validBuilding, id: "src-1" };
    const report = validateModel([a, b]);
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
});
