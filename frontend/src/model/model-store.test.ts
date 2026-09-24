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

  it("replaces one of two features sharing an id, not both", () => {
    // `feature.id.duplicate` is a finding the validator raises, not a state the
    // store refuses, so a bulk sign-off can run over a model that holds one.
    // Addressing by id would overwrite both entries with the same object and
    // throw the other's geometry away — irreversibly, since the undo map would
    // be built the same wrong way.
    const first: ModelFeature = { ...pointSource, heightM: 1 };
    const second: ModelFeature = { ...pointSource, heightM: 2 };
    const store = useModelStore.getState();
    store.addFeature(first);
    store.addFeature(second);

    store.updateFeatures([{ ...first, properties: { reviewed: true } }]);

    const after = useModelStore.getState().features;
    expect(after).toHaveLength(2);
    expect(after[0]?.heightM).toBe(1);
    expect(after[0]?.properties).toEqual({ reviewed: true });
    // The second is untouched, height and all.
    expect(after[1]).toEqual(second);

    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toEqual([first, second]);
  });

  it("gives two incoming features sharing an id a position each", () => {
    const first: ModelFeature = { ...pointSource, heightM: 1 };
    const second: ModelFeature = { ...pointSource, heightM: 2 };
    const store = useModelStore.getState();
    store.addFeature(first);
    store.addFeature(second);

    store.updateFeatures([
      { ...first, heightM: 10 },
      { ...second, heightM: 20 },
    ]);

    expect(useModelStore.getState().features.map((f) => f.heightM)).toEqual([
      10, 20,
    ]);
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

describe("replaceBuildingsInBBox", () => {
  // A ~10 m square footprint centred on (x, y), in degrees.
  function footprint(x: number, y: number): ModelFeature["geometry"] {
    const d = 0.00005;
    return {
      type: "Polygon",
      coordinates: [
        [
          [x - d, y - d],
          [x + d, y - d],
          [x + d, y + d],
          [x - d, y + d],
          [x - d, y - d],
        ],
      ],
    };
  }

  const bbox = { south: 52.37, west: 9.73, north: 52.38, east: 9.74 };

  const osmInside: ModelFeature = {
    id: "osm-way-1",
    kind: "building",
    heightM: 9,
    properties: { osm_id: "1" },
    geometry: footprint(9.735, 52.375),
  };
  const osmOutside: ModelFeature = {
    id: "osm-way-2",
    kind: "building",
    heightM: 9,
    properties: { osm_id: "2" },
    geometry: footprint(9.75, 52.375),
  };
  const drawnInside: ModelFeature = {
    id: "bld-drawn",
    kind: "building",
    heightM: 6,
    geometry: footprint(9.736, 52.376),
  };
  const osmRoad: ModelFeature = {
    id: "osm-way-3",
    kind: "source",
    sourceType: "line",
    properties: { osm_id: "3", highway: "residential" },
    geometry: {
      type: "LineString",
      coordinates: [
        [9.731, 52.371],
        [9.739, 52.379],
      ],
    },
  };
  const insideReceiver: ModelReceiver = {
    id: "rcv-in",
    heightM: 4,
    geometry: { type: "Point", coordinates: [9.735, 52.374] },
  };
  const lgln: ModelFeature = {
    id: "DENILD0100000001",
    kind: "building",
    heightM: 11.4,
    properties: { import_format: "lgln-lod2" },
    geometry: footprint(9.735, 52.375),
  };
  const lglnModel = { features: [lgln], receivers: [], calcArea: null };

  function seed() {
    useModelStore.getState().loadModel({
      features: [osmInside, osmOutside, drawnInside, osmRoad],
      receivers: [insideReceiver],
      calcArea: null,
      crs: "EPSG:4326",
    });
  }

  it("removes only the OSM buildings inside the box and adds the LGLN ones", () => {
    seed();

    const result = useModelStore
      .getState()
      .replaceBuildingsInBBox(lglnModel, bbox);

    expect(result.removed).toBe(1);
    expect(result.skipped.features).toBe(0);
    const state = useModelStore.getState();
    expect(state.features.map((f) => f.id)).toEqual([
      "osm-way-2",
      "bld-drawn",
      "osm-way-3",
      "DENILD0100000001",
    ]);
    expect(state.receivers).toEqual([insideReceiver]);
  });

  it("recognises an OSM building by its osm_id tag when the id was renamed", () => {
    useModelStore.getState().loadModel({
      features: [{ ...osmInside, id: "renamed" }],
      receivers: [],
      calcArea: null,
    });

    const result = useModelStore
      .getState()
      .replaceBuildingsInBBox(lglnModel, bbox);

    expect(result.removed).toBe(1);
  });

  it("undoes the whole replacement in one step", () => {
    seed();
    const before = useModelStore.getState();

    useModelStore.getState().replaceBuildingsInBBox(lglnModel, bbox);
    useModelStore.getState().undo();

    const after = useModelStore.getState();
    expect(after.features).toEqual(before.features);
    expect(after.receivers).toEqual(before.receivers);
    expect(after.canUndo).toBe(false);

    useModelStore.getState().redo();
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual([
      "osm-way-2",
      "bld-drawn",
      "osm-way-3",
      "DENILD0100000001",
    ]);
  });

  it("does not duplicate on a repeated load, and pushes nothing for it", () => {
    seed();
    useModelStore.getState().replaceBuildingsInBBox(lglnModel, bbox);
    const once = useModelStore.getState().features;

    const result = useModelStore
      .getState()
      .replaceBuildingsInBBox(lglnModel, bbox);

    expect(result).toEqual({
      removed: 0,
      skipped: { features: 1, receivers: 0, calcArea: false },
    });
    expect(useModelStore.getState().features).toBe(once);
    // One undo takes back the first load; there is no second step.
    useModelStore.getState().undo();
    expect(useModelStore.getState().canUndo).toBe(false);
  });

  it("keeps a previous LGLN building even though it lies inside the box", () => {
    useModelStore.getState().loadModel({
      features: [{ ...lgln, id: "DENILD0100000009" }],
      receivers: [],
      calcArea: null,
    });

    const result = useModelStore
      .getState()
      .replaceBuildingsInBBox(lglnModel, bbox);

    expect(result.removed).toBe(0);
    expect(useModelStore.getState().features).toHaveLength(2);
  });

  it("removes nothing from a projected workspace", () => {
    useModelStore.getState().loadModel({
      features: [osmInside],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });

    const result = useModelStore
      .getState()
      .replaceBuildingsInBBox(lglnModel, bbox);

    expect(result.removed).toBe(0);
    expect(useModelStore.getState().features.map((f) => f.id)).toContain(
      "osm-way-1",
    );
  });
});
