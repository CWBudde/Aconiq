import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { useModelStore } from "@/model/model-store";
import type { Geometry } from "@/model/types";
import type { DrawMode } from "./use-draw";
import { useDrawProjection } from "./use-draw-projection";

/**
 * The state machine around the map's one inverse transform. The projection
 * itself is `draw-projection.parity.test.ts`'s business — a fake answers in
 * whichever direction it was asked, so it can never catch a swapped CRS pair.
 * What this file pins is what the page sees while the answer is out, and which
 * answers are allowed to land.
 */
const projection = vi.hoisted(() => {
  const scaled = (req: TransformRequest): Promise<TransformResponse> =>
    Promise.resolve({
      source_crs: req.source_crs,
      target_crs: req.target_crs,
      applied: true,
      coordinates: req.coordinates.map((value) => value * 2),
    });

  const value: {
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
    readonly scaled: (req: TransformRequest) => Promise<TransformResponse>;
  } = { requests: [], respond: scaled, scaled };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    capabilities: { kind: "browser", canReprojectForDisplay: true },
    transformCoordinates: (req: TransformRequest) => {
      projection.requests.push(req);
      return projection.respond(req);
    },
  },
}));

function drawn(lng: number, lat: number): GeoJSON.Feature {
  return {
    type: "Feature",
    properties: {},
    geometry: { type: "Point", coordinates: [lng, lat] },
  };
}

function setCRS(crs: string) {
  act(() => {
    useModelStore.getState().setCRS(crs);
  });
}

function mount() {
  const landed: { mode: DrawMode; geometry: Geometry }[] = [];
  const view = renderHook(() =>
    useDrawProjection((mode, geometry) => {
      landed.push({ mode, geometry });
    }),
  );
  return { landed, ...view };
}

beforeEach(() => {
  useModelStore.getState().reset();
  projection.requests = [];
  projection.respond = projection.scaled;
});

describe("useDrawProjection", () => {
  it("hands a shape straight on where the store is already in WGS 84", () => {
    // Synchronously, and on the same tick: the new-feature dialog opening a
    // frame late is a flicker on the commonest path there is.
    const { landed, result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });

    expect(landed).toEqual([
      { mode: "point", geometry: { type: "Point", coordinates: [10, 51] } },
    ]);
    expect(projection.requests).toEqual([]);
    expect(result.current.status).toEqual({ status: "idle" });
  });

  it("names the store's CRS as the target rather than leaving it to auto", async () => {
    // `auto` would resolve a zone from the drawn shape instead of honouring
    // the one the model is already stored in.
    setCRS("EPSG:25832");
    const { landed, result } = mount();

    act(() => {
      result.current.accept("polygon", drawn(10, 51));
    });

    await waitFor(() => {
      expect(landed).toHaveLength(1);
    });
    expect(projection.requests).toEqual([
      {
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        coordinates: [10, 51],
      },
    ]);
    expect(landed[0]?.geometry).toEqual({
      type: "Point",
      coordinates: [20, 102],
    });
  });

  it("reports the wait and clears it when the shape lands", async () => {
    setCRS("EPSG:25832");
    let answer: ((response: TransformResponse) => void) | null = null;
    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });
    const { landed, result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });

    expect(result.current.status).toEqual({
      status: "projecting",
      targetCRS: "EPSG:25832",
    });
    expect(landed).toEqual([]);

    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: [20, 102],
      });
    });

    await waitFor(() => {
      expect(result.current.status).toEqual({ status: "idle" });
    });
    expect(landed).toHaveLength(1);
  });

  it("drops the shape and says why when the projection fails", async () => {
    setCRS("EPSG:25832");
    projection.respond = () => Promise.reject(new Error("outside the zone"));
    const { landed, result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });

    await waitFor(() => {
      expect(result.current.status.status).toBe("failed");
    });
    expect(landed).toEqual([]);
    const status = result.current.status;
    if (status.status !== "failed") throw new Error("expected a failure");
    expect(status.error.message).toBe("outside the zone");
    expect(status.targetCRS).toBe("EPSG:25832");

    act(() => {
      result.current.dismiss();
    });
    expect(result.current.status).toEqual({ status: "idle" });
  });

  it("does not land an answer the store has moved on from", async () => {
    // A CRS change while a shape is in flight makes the answer an answer to a
    // question nobody is asking any more — and landing it would write metres
    // of the wrong zone into the model.
    setCRS("EPSG:25832");
    let answer: ((response: TransformResponse) => void) | null = null;
    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });
    const { landed, result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });
    setCRS("EPSG:4326");
    act(() => {
      result.current.accept("point", drawn(11, 52));
    });

    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: [20, 102],
      });
    });

    await waitFor(() => {
      expect(landed).toHaveLength(1);
    });
    // The second, unprojected shape — not the first one's late answer.
    expect(landed[0]?.geometry).toEqual({
      type: "Point",
      coordinates: [11, 52],
    });
  });

  it("drops an answer a CRS change alone has invalidated", async () => {
    // The same hazard as above without the second shape to advance the token:
    // undoing a model merge restores the previous CRS while a transform is out.
    // Nothing else moves, so the answer would land coordinates computed for the
    // CRS the store has just left into the one it now declares.
    setCRS("EPSG:25832");
    let answer: ((response: TransformResponse) => void) | null = null;
    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });
    const { landed, result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });
    expect(result.current.status).toEqual({
      status: "projecting",
      targetCRS: "EPSG:25832",
    });

    setCRS("EPSG:25833");

    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: [20, 102],
      });
    });

    await waitFor(() => {
      expect(result.current.status).toEqual({ status: "idle" });
    });
    expect(landed).toHaveLength(0);
  });

  it("does not strand the map on 'projecting' when it drops an answer", async () => {
    // Dropping the answer is only half of it. The status is what `pages/map.tsx`
    // keeps the draw toolbar disabled on, so a shape nothing is going to deliver
    // must not leave it up — that would disable drawing for the rest of the
    // session.
    setCRS("EPSG:25832");
    projection.respond = () => new Promise<TransformResponse>(() => {});
    const { result } = mount();

    act(() => {
      result.current.accept("point", drawn(10, 51));
    });
    expect(result.current.status.status).toBe("projecting");

    setCRS("EPSG:25833");

    await waitFor(() => {
      expect(result.current.status).toEqual({ status: "idle" });
    });
  });
});
