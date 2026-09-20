import { beforeEach, describe, expect, it } from "vitest";
import { useModelStore } from "./model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "./types";

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

const calcArea: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [0, 0],
        [2, 0],
        [2, 2],
        [0, 0],
      ],
    ],
  },
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

  it("loadModel replaces all features (not undoable)", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore
      .getState()
      .loadModel({ features: [building], receivers: [], calcArea: null });
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

  // `dirty` means "differs from the project". A load brings content from
  // outside the project (a file, OSM, a recovered draft), so the model is
  // dirty until it is saved there — the draft write must not clear it.
  it("loadModel marks the model dirty", () => {
    useModelStore
      .getState()
      .loadModel({ features: [pointSource], receivers: [], calcArea: null });
    expect(useModelStore.getState().dirty).toBe(true);
  });

  it("reset leaves the model clean", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().reset();
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

describe("mergeModel", () => {
  const imported = {
    features: [pointSource, building],
    receivers: [receiver],
    calcArea,
  };

  it("adds features, receivers and the calculation area", () => {
    const skipped = useModelStore.getState().mergeModel(imported);

    expect(useModelStore.getState().features).toEqual([pointSource, building]);
    expect(useModelStore.getState().receivers).toEqual([receiver]);
    expect(useModelStore.getState().calcArea).toEqual(calcArea);
    expect(skipped).toEqual({ features: 0, receivers: 0, calcArea: false });
  });

  it("keeps what the workspace already holds under the same id", () => {
    // Skipped, not re-minted: a re-minted id is a second copy of the same
    // feature, and the project keys on ids.
    useModelStore.getState().addFeature(pointSource);
    const edited: ModelFeature = { ...pointSource, heightM: 99 };

    const skipped = useModelStore.getState().mergeModel({
      features: [edited, building],
      receivers: [],
      calcArea: null,
    });

    expect(useModelStore.getState().features).toEqual([pointSource, building]);
    expect(skipped.features).toBe(1);
  });

  it("skips a receiver whose id a feature already took", () => {
    // One namespace, because `validateProjectModel` checks ids as one.
    useModelStore.getState().addFeature(pointSource);
    const collision: ModelReceiver = { ...receiver, id: pointSource.id };

    const skipped = useModelStore
      .getState()
      .mergeModel({ features: [], receivers: [collision], calcArea: null });

    expect(useModelStore.getState().receivers).toEqual([]);
    expect(skipped.receivers).toBe(1);
  });

  it("is idempotent when the same model arrives twice", () => {
    useModelStore.getState().mergeModel(imported);
    const before = useModelStore.getState();
    const snapshot = {
      features: before.features,
      receivers: before.receivers,
      calcArea: before.calcArea,
      canUndo: before.canUndo,
      canRedo: before.canRedo,
    };

    const skipped = useModelStore.getState().mergeModel(imported);

    expect(useModelStore.getState().features).toEqual(snapshot.features);
    expect(useModelStore.getState().receivers).toEqual(snapshot.receivers);
    expect(useModelStore.getState().calcArea).toEqual(snapshot.calcArea);
    expect(skipped).toEqual({ features: 2, receivers: 1, calcArea: true });
  });

  it("pushes nothing onto the undo stack when it changes nothing", () => {
    // A no-op command would still clear the redo stack, so a second import of
    // the same file would be idempotent in the model and not in the history.
    useModelStore.getState().mergeModel(imported);
    useModelStore.getState().undo();
    useModelStore.getState().mergeModel({
      features: [],
      receivers: [],
      calcArea: null,
    });

    expect(useModelStore.getState().canRedo).toBe(true);
  });

  it("keeps an existing calculation area over an imported one", () => {
    useModelStore.getState().setCalcArea(calcArea);
    const otherArea: CalcArea = {
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [5, 5],
            [6, 5],
            [6, 6],
            [5, 5],
          ],
        ],
      },
    };

    const skipped = useModelStore
      .getState()
      .mergeModel({ features: [], receivers: [], calcArea: otherArea });

    expect(useModelStore.getState().calcArea).toEqual(calcArea);
    expect(skipped.calcArea).toBe(true);
  });

  it("is one undo, restoring the exact prior state", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().mergeModel({
      features: [building],
      receivers: [receiver],
      calcArea,
    });

    useModelStore.getState().undo();

    expect(useModelStore.getState().features).toEqual([pointSource]);
    expect(useModelStore.getState().receivers).toEqual([]);
    expect(useModelStore.getState().calcArea).toBeNull();
    // One command, not one per imported item: the first feature is still
    // there and only a second undo removes it.
    expect(useModelStore.getState().canUndo).toBe(true);
  });

  it("marks the model dirty, like every other load", () => {
    useModelStore.getState().mergeModel(imported);
    expect(useModelStore.getState().dirty).toBe(true);
  });
});

describe("updateFeatures", () => {
  /** Three sources, so "one step" is distinguishable from "one per feature". */
  function threeSources(): ModelFeature[] {
    return ["a", "b", "c"].map((suffix) => ({
      ...pointSource,
      id: `src-${suffix}`,
    }));
  }

  it("replaces many features as a single undo step", () => {
    // The reason it exists. A loop over `updateFeature` would push one command
    // each, so taking back a sign-off over 608 imported sources would be 608
    // presses of Ctrl+Z — a bulk action nobody could safely try.
    const store = useModelStore.getState();
    for (const feature of threeSources()) store.addFeature(feature);

    store.updateFeatures(
      threeSources().map((f) => ({ ...f, properties: { reviewed: true } })),
    );
    expect(
      useModelStore
        .getState()
        .features.every((f) => f.properties !== undefined),
    ).toBe(true);

    useModelStore.getState().undo();

    expect(
      useModelStore
        .getState()
        .features.every((f) => f.properties === undefined),
    ).toBe(true);
  });

  it("keeps the order and the untouched features", () => {
    const store = useModelStore.getState();
    store.addFeature(pointSource);
    store.addFeature(building);

    store.updateFeatures([{ ...pointSource, heightM: 3 }]);

    const features = useModelStore.getState().features;
    expect(features.map((f) => f.id)).toEqual(["src-1", "bld-1"]);
    expect(features[1]).toEqual(building);
  });

  it("skips ids the store does not hold rather than adding them", () => {
    // It replaces features; it does not create them. A caller holding a stale
    // list must not be able to resurrect a deleted source through it.
    const store = useModelStore.getState();
    store.addFeature(pointSource);

    store.updateFeatures([
      { ...pointSource, heightM: 3 },
      { ...pointSource, id: "ghost" },
    ]);

    const features = useModelStore.getState().features;
    expect(features).toHaveLength(1);
    expect(features[0]?.heightM).toBe(3);
  });

  it("pushes nothing when it would change nothing", () => {
    // An empty command on the stack is a Ctrl+Z that appears to do nothing,
    // which reads as broken undo.
    const store = useModelStore.getState();
    store.addFeature(pointSource);
    const before = useModelStore.getState().features;

    useModelStore.getState().updateFeatures([]);
    useModelStore.getState().undo();

    expect(useModelStore.getState().features).not.toBe(before);
    expect(useModelStore.getState().features).toEqual([]);
  });
});
