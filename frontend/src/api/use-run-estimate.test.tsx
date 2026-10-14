import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { RunSpec } from "./backend";
import { useRunEstimate } from "./use-run-estimate";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelReceiver } from "@/model/types";

const estimates = vi.hoisted(() => ({ calls: 0 }));

vi.mock("./backend", () => ({
  backend: {
    estimateRun: () => {
      estimates.calls += 1;
      return Promise.resolve({
        available: true,
        standard_id: "rls19-road",
        wall_s: { low: 1, estimate: estimates.calls, high: 10 },
      });
    },
  },
}));

const SPEC: RunSpec = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: {},
  receiverMode: "custom",
};

const RECEIVER: ModelReceiver = {
  id: "r1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [9.73, 52.37] },
};

function square(x: number): CalcArea {
  return {
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [x, 52.37],
          [x + 0.01, 52.37],
          [x + 0.01, 52.38],
          [x, 52.38],
          [x, 52.37],
        ],
      ],
    },
  };
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  estimates.calls = 0;
  useModelStore.getState().reset();
  useModelStore.getState().addReceiver(RECEIVER);
  useModelStore.getState().setCalcArea(square(9.73));
});

describe("useRunEstimate", () => {
  it("is idle without a spec", () => {
    const { result } = renderHook(() => useRunEstimate(null), { wrapper });
    expect(result.current).toEqual({ status: "idle" });
  });

  // Every store field the browser request reads (`prepareRLS19Request`, via
  // `resolveComputeModel`) is a model input of the estimate. An edit to any of
  // them, with the spec unchanged, has to make the old answer non-current and
  // ask again.
  it.each([
    [
      "a feature is added",
      () => {
        useModelStore.getState().addFeature({
          id: "b1",
          kind: "building",
          heightM: 10,
          geometry: square(9.75).geometry,
        });
      },
    ],
    [
      "a receiver moves",
      () => {
        useModelStore.getState().updateReceiverGeometry("r1", [9.74, 52.375]);
      },
    ],
    [
      "the calc area changes",
      () => {
        useModelStore.getState().setCalcArea(square(9.72));
      },
    ],
    [
      "the CRS changes",
      () => {
        useModelStore.getState().setCRS("EPSG:25832");
      },
    ],
  ])(
    "does not show the estimate from before as current when %s",
    async (_, edit) => {
      const { result } = renderHook(() => useRunEstimate(SPEC), { wrapper });

      await waitFor(
        () => {
          expect(result.current.status).toBe("ready");
        },
        { timeout: 3000 },
      );
      expect(estimates.calls).toBe(1);

      // The model changes; the spec does not. Until the edited model has been
      // estimated, the old answer must not read as this model's.
      act(edit);
      expect(result.current.status).toBe("estimating");

      await waitFor(
        () => {
          expect(result.current.status).toBe("ready");
        },
        { timeout: 3000 },
      );
      expect(estimates.calls).toBe(2);
    },
  );

  it("does not ask again for a store change the request does not read", async () => {
    const { result } = renderHook(() => useRunEstimate(SPEC), { wrapper });

    await waitFor(
      () => {
        expect(result.current.status).toBe("ready");
      },
      { timeout: 3000 },
    );

    // Saving marks the model clean and moves none of its inputs.
    act(() => {
      useModelStore.getState().markClean();
    });
    expect(result.current.status).toBe("ready");
    expect(estimates.calls).toBe(1);
  });
});
