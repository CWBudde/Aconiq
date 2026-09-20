import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { MapContext } from "./use-map";
import { useDisplayModel } from "./display-model";
import { ModelLayers } from "./model-layers";
import { useMapStore } from "./map-store";
import { LAYER_IDS, SOURCE_IDS } from "./layers";

/**
 * What the map does with a model it cannot draw in the CRS the model is in.
 *
 * The file did not exist before this change, which is part of why nothing
 * noticed that `fitBounds` sat outside both `try` blocks: a 5 644 000 northing
 * threw `Invalid LngLat latitude value` straight out of the sync effect. The
 * fake below models that throw rather than merely asserting the guard, because
 * modelling it is what makes the guard testable at all.
 */
const state = vi.hoisted(() => {
  const value: {
    canReprojectForDisplay: boolean;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    requests: [],
    respond: (req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.target_crs,
        applied: true,
        coordinates: req.coordinates.map((value) => value / 100000),
      }),
  };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.canReprojectForDisplay ? "browser" : "http",
        canExport: false,
        runsAgainstSavedModel: false,
        runsChangeExternally: false,
        exportsOutliveRunDelete: false,
        canReprojectForDisplay: state.canReprojectForDisplay,
      };
    },
    transformCoordinates: (req: TransformRequest) => {
      state.requests.push(req);
      return state.respond(req);
    },
  },
}));

/**
 * The MapLibre surface `ModelLayers` touches, and only that.
 *
 * `fitBounds` throws the way MapLibre throws: a latitude outside ±90 is not a
 * silently clamped view, it is an exception out of the caller's effect.
 */
class FakeMap {
  readonly sources = new Map<string, { data: unknown }>();
  readonly layers = new Set<string>();
  /** Add order, which a Set does not keep — the z-order is the paint order. */
  readonly layerOrder: string[] = [];
  readonly fitBoundsCalls: [[number, number], [number, number]][] = [];
  /** Feature states by `${source}/${id}`, the way MapLibre keys them. */
  readonly featureStates = new Map<string, Record<string, unknown>>();
  /** The last `visibility` written per layer id. */
  readonly layout = new Map<string, string>();

  getStyle() {
    return { sources: {} };
  }

  getSource(id: string) {
    const source = this.sources.get(id);
    if (!source) return undefined;
    return {
      setData: (data: unknown) => {
        source.data = data;
      },
    };
  }

  addSource(id: string, source: { data: unknown }) {
    this.sources.set(id, { data: source.data });
  }

  getLayer(id: string) {
    return this.layers.has(id) ? { id } : undefined;
  }

  addLayer(layer: { id: string }) {
    this.layers.add(layer.id);
    this.layerOrder.push(layer.id);
  }

  setLayoutProperty(id: string, property: string, value: string) {
    // MapLibre throws for a layer that is not on the style; the caller relies
    // on that being harmless.
    if (!this.layers.has(id)) throw new Error(`no layer ${id}`);
    this.layout.set(`${id}/${property}`, value);
  }

  setFeatureState(
    target: { source: string; id: string },
    state: Record<string, unknown>,
  ) {
    const key = `${target.source}/${target.id}`;
    this.featureStates.set(key, {
      ...(this.featureStates.get(key) ?? {}),
      ...state,
    });
  }

  removeFeatureState(target: { source: string; id: string }, key: string) {
    const stored = this.featureStates.get(`${target.source}/${target.id}`);
    if (!stored) return;
    Reflect.deleteProperty(stored, key);
  }

  fitBounds(bounds: [[number, number], [number, number]]) {
    const [[, south], [, north]] = bounds;
    if (Math.abs(south) > 90 || Math.abs(north) > 90) {
      throw new Error("Invalid LngLat latitude value");
    }
    this.fitBoundsCalls.push(bounds);
  }
}

function drawn(map: FakeMap, id: string): GeoJSON.FeatureCollection {
  const source = map.sources.get(id);
  if (!source) throw new Error(`source ${id} was never added`);
  return source.data as GeoJSON.FeatureCollection;
}

const METRIC_FEATURE: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [667000, 5644000],
      [667100, 5644100],
    ],
  },
};

const METRIC_RECEIVER: ModelReceiver = {
  id: "R1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [667200, 5644200] },
};

const WGS84_FEATURE: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [10, 51],
      [10.01, 51.01],
    ],
  },
};

const AREA: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [10.1, 51],
        [10.1, 51.1],
        [10, 51],
      ],
    ],
  },
};

// `ModelLayers` takes the display projection as a prop so that `pages/map.tsx`
// can hold the one `useDisplayModel` instance and gate drawing on it. The tests
// still drive the store, so the harness supplies the same hook the page does.
function Harness({ selectedFeatureId }: { selectedFeatureId: string | null }) {
  return (
    <ModelLayers
      display={useDisplayModel()}
      selectedFeatureId={selectedFeatureId}
    />
  );
}

function renderLayers(map: FakeMap, selectedFeatureId: string | null = null) {
  return render(
    <MapContext value={map as unknown as MapLibreMap}>
      <Harness selectedFeatureId={selectedFeatureId} />
    </MapContext>,
  );
}

function selectionState(map: FakeMap, source: string, id: string): unknown {
  return map.featureStates.get(`${source}/${id}`)?.["selected"];
}

beforeEach(() => {
  state.canReprojectForDisplay = true;
  state.requests = [];
  useModelStore.getState().reset();
  useMapStore.setState({ layerVisibility: {} });
});

describe("ModelLayers", () => {
  it("draws a WGS84 model as the store holds it, fitting the whole workspace", () => {
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [10.5, 51.5] },
        },
      ],
      calcArea: AREA,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();

    renderLayers(map);

    expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(1);
    // The receiver at 10.5/51.5 and the calculation area are both inside the
    // fitted extent. `computeFeatureBounds`, the copy this file used to keep,
    // visited features alone and framed a workspace to a fraction of itself.
    expect(map.fitBoundsCalls).toEqual([
      [
        [10, 51],
        [10.5, 51.5],
      ],
    ]);
    expect(state.requests).toEqual([]);
  });

  it("draws nothing while a metric model is being projected, then draws it in WGS84", async () => {
    useModelStore.getState().loadModel({
      features: [METRIC_FEATURE],
      receivers: [METRIC_RECEIVER],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const map = new FakeMap();

    renderLayers(map);

    // A stale model left drawn in the wrong place is the bug; empty is the
    // honest state while the answer is on its way.
    expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(0);
    expect(map.fitBoundsCalls).toEqual([]);

    await waitFor(() => {
      expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(1);
    });
    const line = drawn(map, SOURCE_IDS.sources).features[0]?.geometry;
    expect(line).toEqual({
      type: "LineString",
      coordinates: [
        [6.67, 56.44],
        [6.671, 56.441],
      ],
    });

    // The one-shot fit survived the `projecting` render: had the counter
    // advanced there, the reprojected model would never have come into view.
    expect(map.fitBoundsCalls).toEqual([
      [
        [6.67, 56.44],
        [6.672, 56.442],
      ],
    ]);

    // A reprojected model draws, and — since the inverse transform landed —
    // drawing into it is allowed, so the notice says where the model is stored
    // and stops there. A refusal here would be the stale premise it used to
    // carry: the projector that drew this model is the same one a finished
    // shape travels back through.
    const notice = screen.getByRole("status");
    expect(notice).toHaveTextContent(
      m.msg_map_crs_reprojected({ crs: "EPSG:25832" }),
    );
    expect(notice).not.toHaveTextContent(
      m.msg_draw_disabled_no_projection({ crs: "EPSG:25832" }),
    );
  });

  it("frames a receiver-only workspace once it is ready", async () => {
    // The bounds helper deliberately visits receivers and the calculation
    // area, and the one-shot fit was still gated on `features.length` — which
    // cancelled the fix for the commonest case there is, an import of
    // receivers alone.
    useModelStore.getState().loadModel({
      features: [],
      receivers: [METRIC_RECEIVER],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const map = new FakeMap();

    renderLayers(map);

    // Not while it is projecting: framing an empty map would consume the fit.
    expect(map.fitBoundsCalls).toEqual([]);

    await waitFor(() => {
      expect(drawn(map, SOURCE_IDS.receivers).features).toHaveLength(1);
    });

    // Exactly once, on the `ready` render that first had something to frame.
    expect(map.fitBoundsCalls).toEqual([
      [
        [6.672, 56.442],
        [6.672, 56.442],
      ],
    ]);
  });

  it("does not write the display projection back into the store", async () => {
    useModelStore.getState().loadModel({
      features: [METRIC_FEATURE],
      receivers: [METRIC_RECEIVER],
      calcArea: null,
      crs: "EPSG:25832",
    });
    useModelStore.getState().markClean();
    const fixtureFeatures = useModelStore.getState().features;
    const map = new FakeMap();

    renderLayers(map);
    await waitFor(() => {
      expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(1);
    });

    // Reference identity, not deep equality: identity fails on a write-back
    // that happens to round-trip to the same numbers, and deep equality does
    // not. *The map is a projection of the model, never a source for it.*
    expect(useModelStore.getState().features).toBe(fixtureFeatures);
    expect(useModelStore.getState().features[0]).toBe(fixtureFeatures[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");
    expect(useModelStore.getState().dirty).toBe(false);
  });

  it("empties the sources and says so when no projector is reachable", async () => {
    state.canReprojectForDisplay = false;
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE],
      receivers: [],
      calcArea: AREA,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();

    renderLayers(map);
    await waitFor(() => {
      expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(1);
    });

    act(() => {
      useModelStore.getState().loadModel({
        features: [METRIC_FEATURE],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      });
    });

    expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(0);
    expect(state.requests).toEqual([]);
    expect(screen.getByRole("status")).toHaveTextContent("EPSG:25832");
    expect(screen.getByRole("status")).toHaveTextContent(
      m.msg_draw_disabled_no_projection({ crs: "EPSG:25832" }),
    );
  });

  it("survives an extent that is not lon/lat instead of throwing out of the effect", () => {
    // The case reprojection cannot fix: a store labelled EPSG:4326 that holds
    // metres. `fitBounds` was outside both `try` blocks, so MapLibre's
    // `Invalid LngLat latitude value` took the whole sync down with it.
    useModelStore.getState().loadModel({
      features: [METRIC_FEATURE],
      receivers: [],
      calcArea: null,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    expect(() => {
      renderLayers(map);
    }).not.toThrow();

    expect(map.fitBoundsCalls).toEqual([]);
    expect(drawn(map, SOURCE_IDS.sources).features).toHaveLength(1);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});

/**
 * Which feature the editor is open on, drawn on the map.
 *
 * It is a `feature-state` and never a model property: the model is what a run
 * reads, and a highlight is not an input to anything. The pairing this has to
 * get right is (source, id) — the features are split into three sources by
 * kind, and a state written against the wrong one paints nothing and reports
 * no error.
 */
describe("ModelLayers layer visibility", () => {
  it("re-applies what the layer control hid when the layers are added again", () => {
    // A basemap switch and a tile-failure fallback both rebuild the map, which
    // is how the model layers come back at all. They come back at the
    // visibility their specification declares, so the store is the only record
    // of the user's choice — without this the group reappears on the canvas
    // while `LayerControl` still labels it as hidden.
    useMapStore.getState().setLayerVisible("buildings", false);
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE],
      receivers: [],
      calcArea: null,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();

    renderLayers(map);

    expect(map.layout.get(`${LAYER_IDS.buildingsFill}/visibility`)).toBe(
      "none",
    );
    expect(map.layout.get(`${LAYER_IDS.buildingsOutline}/visibility`)).toBe(
      "none",
    );
    // A group with no stored answer keeps its own default.
    expect(map.layout.get(`${LAYER_IDS.receiversPoint}/visibility`)).toBe(
      "visible",
    );
  });

  it("lets the review flags be turned off once the review is done", () => {
    // 608 dashed casings is a lot of ink to leave on the map afterwards, so
    // the flags are a layer group like any other rather than always-on paint.
    useMapStore.getState().setLayerVisible("review-required", false);
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE],
      receivers: [],
      calcArea: null,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();

    renderLayers(map);

    expect(map.layout.get(`${LAYER_IDS.sourcesReviewLine}/visibility`)).toBe(
      "none",
    );
    expect(map.layout.get(`${LAYER_IDS.sourcesReviewPoint}/visibility`)).toBe(
      "none",
    );
    // The sources themselves stay: hiding the flag must not hide the road.
    expect(map.layout.get(`${LAYER_IDS.sourcesLine}/visibility`)).toBe(
      "visible",
    );
  });

  it("draws the review flags under the sources they belong to", () => {
    // A casing, not a covering: a flagged road has to stay a red road.
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE],
      receivers: [],
      calcArea: null,
      crs: "EPSG:4326",
    });
    const map = new FakeMap();

    renderLayers(map);

    const order = map.layerOrder;
    expect(order).toContain(LAYER_IDS.sourcesReviewLine);
    expect(order.indexOf(LAYER_IDS.sourcesReviewLine)).toBeLessThan(
      order.indexOf(LAYER_IDS.sourcesLine),
    );
  });
});

describe("ModelLayers selection", () => {
  const WGS84_BUILDING: ModelFeature = {
    id: "bld-1",
    kind: "building",
    heightM: 8,
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [10, 51],
          [10.01, 51],
          [10.01, 51.01],
          [10, 51],
        ],
      ],
    },
  };

  const WGS84_RECEIVER: ModelReceiver = {
    id: "R1",
    heightM: 4,
    geometry: { type: "Point", coordinates: [10.5, 51.5] },
  };

  function loadWGS84() {
    useModelStore.getState().loadModel({
      features: [WGS84_FEATURE, WGS84_BUILDING],
      receivers: [WGS84_RECEIVER],
      calcArea: null,
      crs: "EPSG:4326",
    });
  }

  it("marks a feature on the source its kind is drawn from", () => {
    loadWGS84();
    const map = new FakeMap();

    renderLayers(map, "bld-1");

    expect(selectionState(map, SOURCE_IDS.buildings, "bld-1")).toBe(true);
    // Not on the sources layer, where a kind-blind write would have put it.
    expect(selectionState(map, SOURCE_IDS.sources, "bld-1")).toBeUndefined();
  });

  it("marks a receiver on the receiver source", () => {
    loadWGS84();
    const map = new FakeMap();

    renderLayers(map, "R1");

    expect(selectionState(map, SOURCE_IDS.receivers, "R1")).toBe(true);
  });

  it("moves the mark rather than leaving two features highlighted", () => {
    loadWGS84();
    const map = new FakeMap();
    const { rerender } = renderLayers(map, "road");

    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <Harness selectedFeatureId="R1" />
      </MapContext>,
    );

    expect(selectionState(map, SOURCE_IDS.sources, "road")).toBeUndefined();
    expect(selectionState(map, SOURCE_IDS.receivers, "R1")).toBe(true);
  });

  it("clears the mark when the editor closes", () => {
    loadWGS84();
    const map = new FakeMap();
    const { rerender } = renderLayers(map, "road");

    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <Harness selectedFeatureId={null} />
      </MapContext>,
    );

    expect(selectionState(map, SOURCE_IDS.sources, "road")).toBeUndefined();
  });

  it("writes nothing for an id that is in neither collection", () => {
    // A selected feature can be deleted out from under the panel, and
    // `setFeatureState` stores a state whether or not the feature exists — so
    // a stale write comes back as a highlight on whatever id is reused next.
    loadWGS84();
    const map = new FakeMap();

    renderLayers(map, "deleted-1");

    expect(map.featureStates.size).toBe(0);
  });

  it("waits for the reprojection before marking a metric model", async () => {
    // Nothing is drawn while the display model is projecting, so a state
    // written then would address a feature that is not in the source yet.
    useModelStore.getState().loadModel({
      features: [METRIC_FEATURE],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const map = new FakeMap();

    renderLayers(map, "road");
    expect(map.featureStates.size).toBe(0);

    await waitFor(() => {
      expect(selectionState(map, SOURCE_IDS.sources, "road")).toBe(true);
    });
  });
});
