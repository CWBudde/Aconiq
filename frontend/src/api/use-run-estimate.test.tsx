import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { RunSpec } from "./backend";
import { useRunEstimate } from "./use-run-estimate";

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
  receiverMode: "auto-grid",
};

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  estimates.calls = 0;
});

describe("useRunEstimate", () => {
  it("is idle without a spec", () => {
    const { result } = renderHook(() => useRunEstimate(null, []), { wrapper });
    expect(result.current).toEqual({ status: "idle" });
  });

  it("does not show the estimate of the model before an edit as current", async () => {
    const before: unknown[] = [];
    const { result, rerender } = renderHook(
      ({ model }: { model: object }) => useRunEstimate(SPEC, model),
      { wrapper, initialProps: { model: before } },
    );

    await waitFor(
      () => {
        expect(result.current.status).toBe("ready");
      },
      { timeout: 3000 },
    );

    // The model changes; the spec does not. Until the edited model has been
    // estimated, the old answer must not read as this model's.
    act(() => {
      rerender({ model: [] });
    });
    expect(result.current.status).toBe("estimating");

    await waitFor(
      () => {
        expect(result.current.status).toBe("ready");
      },
      { timeout: 3000 },
    );
    expect(estimates.calls).toBe(2);
  });
});
