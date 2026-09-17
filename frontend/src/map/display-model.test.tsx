import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { DISPLAY_CRS, useDisplayModel } from "./display-model";

/**
 * The map's half of the CRS story: does the model reach MapLibre in lon/lat,
 * and does the hook say so honestly when it cannot?
 *
 * The real projection is `display-projection.parity.test.ts`'s business — a
 * fake cannot catch a swapped source/target. What this file pins is the state
 * machine around it: the identity short-circuit, the request ordering, and the
 * two refusals.
 */
const state = vi.hoisted(() => {
  /** Divides by 100 000, so a projected coordinate is recognisable on sight. */
  const scaled = (req: TransformRequest): Promise<TransformResponse> =>
    Promise.resolve({
      source_crs: req.source_crs,
      target_crs: req.target_crs,
      applied: true,
      coordinates: req.coordinates.map((value) => value / 100000),
    });

  const value: {
    canReprojectForDisplay: boolean;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
    readonly scaled: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    requests: [],
    respond: scaled,
    scaled,
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

const METRIC_AREA: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [666000, 5643000],
        [668000, 5643000],
        [668000, 5645000],
        [666000, 5643000],
      ],
    ],
  },
};

function loadMetric(crs = "EPSG:25832") {
  useModelStore.getState().loadModel({
    features: [METRIC_FEATURE],
    receivers: [METRIC_RECEIVER],
    calcArea: METRIC_AREA,
    crs,
  });
}

beforeEach(() => {
  state.canReprojectForDisplay = true;
  state.requests = [];
  state.respond = state.scaled;
  useModelStore.getState().reset();
});

describe("useDisplayModel", () => {
  it("answers ready on the first render for a WGS84 store, without a transform", () => {
    useModelStore.getState().loadModel({
      features: [
        {
          id: "s1",
          kind: "source",
          sourceType: "point",
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
      receivers: [],
      calcArea: null,
      crs: DISPLAY_CRS,
    });
    const fixture = useModelStore.getState().features;

    const { result } = renderHook(() => useDisplayModel());

    expect(result.current.status).toBe("ready");
    if (result.current.status !== "ready") throw new Error("not ready");
    // Identity, not deep equality: the common render path must be exactly the
    // arrays the store holds, projected through nothing.
    expect(result.current.features).toBe(fixture);
    expect(result.current.features[0]).toBe(fixture[0]);
    expect(result.current.reprojected).toBe(false);
    expect(state.requests).toEqual([]);
  });

  it("projects a metric store and reports it as reprojected", async () => {
    loadMetric();

    const { result } = renderHook(() => useDisplayModel());

    expect(result.current.status).toBe("projecting");
    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    if (result.current.status !== "ready") throw new Error("not ready");

    expect(result.current.sourceCRS).toBe("EPSG:25832");
    expect(result.current.reprojected).toBe(true);
    expect(result.current.features[0]?.geometry.coordinates).toEqual([
      [6.67, 56.44],
      [6.671, 56.441],
    ]);
    expect(result.current.receivers[0]?.geometry.coordinates).toEqual([
      6.672, 56.442,
    ]);
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]?.source_crs).toBe("EPSG:25832");
    expect(state.requests[0]?.target_crs).toBe(DISPLAY_CRS);
  });

  it("never writes the projected model back into the store", async () => {
    loadMetric();
    const fixture = useModelStore.getState().features;

    const { result } = renderHook(() => useDisplayModel());
    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    expect(useModelStore.getState().features).toBe(fixture);
    expect(useModelStore.getState().features[0]).toBe(fixture[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");
  });

  it("drops the answer to a CRS that is no longer the store's", async () => {
    // A stale answer arriving last would draw the previous model in the
    // current model's place — and look entirely plausible doing it.
    const pending: ((response: TransformResponse) => void)[] = [];
    state.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        pending.push(resolve);
      });
    loadMetric();

    const { result } = renderHook(() => useDisplayModel());
    expect(result.current.status).toBe("projecting");

    act(() => {
      loadMetric("EPSG:25833");
    });
    await waitFor(() => {
      expect(state.requests).toHaveLength(2);
    });

    // The first request answers last, and is discarded.
    act(() => {
      pending[1]?.({
        source_crs: "EPSG:25833",
        target_crs: DISPLAY_CRS,
        applied: true,
        coordinates: state.requests[1]?.coordinates.map(() => 2) ?? [],
      });
    });
    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    act(() => {
      pending[0]?.({
        source_crs: "EPSG:25832",
        target_crs: DISPLAY_CRS,
        applied: true,
        coordinates: state.requests[0]?.coordinates.map(() => 1) ?? [],
      });
    });

    if (result.current.status !== "ready") throw new Error("not ready");
    expect(result.current.sourceCRS).toBe("EPSG:25833");
    expect(result.current.features[0]?.geometry.coordinates).toEqual([
      [2, 2],
      [2, 2],
    ]);
  });

  it("refuses without asking when no projector is reachable", () => {
    state.canReprojectForDisplay = false;
    loadMetric();

    const { result } = renderHook(() => useDisplayModel());

    expect(result.current.status).toBe("unsupported");
    expect(result.current.sourceCRS).toBe("EPSG:25832");
    expect(state.requests).toEqual([]);
  });

  it("reports a rejected transform rather than drawing nothing in silence", async () => {
    state.respond = () => Promise.reject(new Error("kernel is unavailable"));
    loadMetric();

    const { result } = renderHook(() => useDisplayModel());

    await waitFor(() => {
      expect(result.current.status).toBe("failed");
    });
    if (result.current.status !== "failed") throw new Error("not failed");
    expect(result.current.error.message).toBe("kernel is unavailable");
  });
});
