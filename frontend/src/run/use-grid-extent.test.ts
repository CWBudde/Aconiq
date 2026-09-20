import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { resetComputeModelCacheForTests } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature } from "@/model/types";
import { useGridExtent } from "./use-grid-extent";

/**
 * The run dialog's extent, which is the run's extent.
 *
 * The projection itself is `compute-crs.test.ts`'s business; the fake here
 * doubles every ordinate, which is enough to tell "the hook projected" from
 * "the hook counted degrees". What this file pins is which states the dialog
 * can be shown and that a geographic model never reaches the arithmetic
 * unprojected.
 */
const projection = vi.hoisted(() => {
  const doubled = (req: TransformRequest): Promise<TransformResponse> =>
    Promise.resolve({
      source_crs: req.source_crs,
      target_crs: "EPSG:25832",
      applied: true,
      coordinates: req.coordinates.map((value) => value * 2),
    });

  const value: {
    canReproject: boolean;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
    readonly doubled: (req: TransformRequest) => Promise<TransformResponse>;
  } = { canReproject: true, requests: [], respond: doubled, doubled };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: "browser",
        canReprojectForDisplay: projection.canReproject,
      };
    },
    transformCoordinates: (req: TransformRequest) => {
      projection.requests.push(req);
      return projection.respond(req);
    },
  },
}));

const ROAD: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  properties: {},
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [100, 50],
    ],
  },
};

const AREA: CalcArea = {
  id: "calc-area",
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
};

function loadModel(patch: { features?: ModelFeature[]; calcArea?: CalcArea }) {
  act(() => {
    useModelStore.setState({
      features: patch.features ?? [],
      calcArea: patch.calcArea ?? null,
    });
  });
}

beforeEach(() => {
  useModelStore.getState().reset();
  resetComputeModelCacheForTests();
  projection.canReproject = true;
  projection.requests = [];
  projection.respond = projection.doubled;
});

describe("useGridExtent", () => {
  it("answers with the extent in the CRS the run computes in", async () => {
    loadModel({ features: [ROAD] });
    const { result } = renderHook(() => useGridExtent(true));

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    // Doubled, not the store's own numbers: a preview that counted 10 m cells
    // over degrees would report one receiver for a whole city.
    expect(result.current).toEqual({
      status: "ready",
      extent: { minX: 0, minY: 0, maxX: 200, maxY: 100 },
    });
  });

  it("prefers the calculation area over the sources", async () => {
    loadModel({ features: [ROAD], calcArea: AREA });
    const { result } = renderHook(() => useGridExtent(true));

    await waitFor(() => {
      expect(result.current).toEqual({
        status: "ready",
        extent: { minX: 0, minY: 0, maxX: 20, maxY: 20 },
      });
    });
  });

  it("says there is no extent, without asking the projection", async () => {
    // A real state, not an error: the run refuses in it too, and an empty
    // model has no coordinate to send.
    const { result } = renderHook(() => useGridExtent(true));

    await waitFor(() => {
      expect(result.current.status).toBe("no-extent");
    });
    expect(projection.requests).toEqual([]);
  });

  it("stays idle while nothing asks", async () => {
    loadModel({ features: [ROAD] });
    const { result } = renderHook(() => useGridExtent(false));

    await waitFor(() => {
      expect(result.current.status).toBe("idle");
    });
    expect(projection.requests).toEqual([]);
  });

  it("reports a backend that cannot project", async () => {
    projection.canReproject = false;
    loadModel({ features: [ROAD] });
    const { result } = renderHook(() => useGridExtent(true));

    await waitFor(() => {
      expect(result.current.status).toBe("unavailable");
    });
    expect(projection.requests).toEqual([]);
  });

  it("reports a failed projection rather than a number", async () => {
    projection.respond = () => Promise.reject(new Error("no zone here"));
    loadModel({ features: [ROAD] });
    const { result } = renderHook(() => useGridExtent(true));

    await waitFor(() => {
      expect(result.current.status).toBe("failed");
    });
  });

  it("re-projects when the model changes", async () => {
    loadModel({ features: [ROAD] });
    const { result } = renderHook(() => useGridExtent(true));
    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    loadModel({ features: [ROAD], calcArea: AREA });
    await waitFor(() => {
      expect(result.current).toEqual({
        status: "ready",
        extent: { minX: 0, minY: 0, maxX: 20, maxY: 20 },
      });
    });
    expect(projection.requests.length).toBe(2);
  });
});
