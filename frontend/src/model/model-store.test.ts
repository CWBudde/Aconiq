import { beforeEach, describe, expect, it } from "vitest";
import { useModelStore } from "./model-store";
import type { ModelFeature, ModelReceiver } from "./types";

const pointSource: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const building: ModelFeature = {
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

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [12, 53] },
};

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("model store", () => {
  it("starts empty", () => {
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("addFeature adds a feature", () => {
    useModelStore.getState().addFeature(pointSource);
    expect(useModelStore.getState().features).toEqual([pointSource]);
  });

  it("addFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("updateFeature replaces a feature by id", () => {
    useModelStore.getState().addFeature(pointSource);
    const updated = {
      ...pointSource,
      geometry: {
        type: "Point" as const,
        coordinates: [11, 52] as [number, number],
      },
    };
    useModelStore.getState().updateFeature(updated);
    expect(useModelStore.getState().features[0]?.geometry.coordinates).toEqual([
      11, 52,
    ]);
  });

  it("updateFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    const updated = {
      ...pointSource,
      geometry: {
        type: "Point" as const,
        coordinates: [11, 52] as [number, number],
      },
    };
    useModelStore.getState().updateFeature(updated);
    useModelStore.getState().undo();
    expect(useModelStore.getState().features[0]?.geometry.coordinates).toEqual([
      10, 51,
    ]);
  });

  it("removeFeature removes a feature by id", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().removeFeature("src-1");
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("removeFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().removeFeature("src-1");
    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toEqual([pointSource]);
  });

  it("loadFeatures replaces all features (not undoable)", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().loadFeatures([building]);
    expect(useModelStore.getState().features).toEqual([building]);
  });

  it("getFeatureById returns the correct feature", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().addFeature(building);
    expect(useModelStore.getState().getFeatureById("bld-1")).toEqual(building);
  });

  it("getFeatureById returns undefined for missing id", () => {
    expect(useModelStore.getState().getFeatureById("nope")).toBeUndefined();
  });

  it("featuresByKind filters correctly", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().addFeature(building);
    expect(useModelStore.getState().featuresByKind("source")).toEqual([
      pointSource,
    ]);
    expect(useModelStore.getState().featuresByKind("building")).toEqual([
      building,
    ]);
  });

  it("dirty flag is false initially and true after edits", () => {
    expect(useModelStore.getState().dirty).toBe(false);
    useModelStore.getState().addFeature(pointSource);
    expect(useModelStore.getState().dirty).toBe(true);
  });

  it("markClean resets dirty flag", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().markClean();
    expect(useModelStore.getState().dirty).toBe(false);
  });

  it("addReceiver adds a receiver", () => {
    useModelStore.getState().addReceiver(receiver);
    expect(useModelStore.getState().receivers).toEqual([receiver]);
  });

  it("removeReceiver is undoable", () => {
    useModelStore.getState().addReceiver(receiver);
    useModelStore.getState().removeReceiver(receiver.id);
    useModelStore.getState().undo();
    expect(useModelStore.getState().receivers).toEqual([receiver]);
  });
});
