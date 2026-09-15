import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { RunSummary } from "@/api/client";
import { useRunFromRoute } from "./use-run-from-route";

const runsState = vi.hoisted(() => ({
  data: undefined as RunSummary[] | undefined,
  isLoading: false,
  error: null as Error | null,
}));

vi.mock("@/api/hooks", () => ({
  useRuns: () => runsState,
}));

function run(id: string, status: RunSummary["status"]): RunSummary {
  return {
    id,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2020",
    profile: "default",
    status,
    started_at: "2026-09-15T10:00:00Z",
    finished_at: "2026-09-15T10:00:12Z",
    log_path: `runs/${id}/run.log`,
    artifacts: [],
  };
}

const completed = run("run-0001", "completed");
const running = run("run-0002", "running");

function isCompleted(r: RunSummary): boolean {
  return r.status === "completed";
}

function resolve(runId: string | undefined, data?: RunSummary[]) {
  runsState.data = data;
  return renderHook(() => useRunFromRoute(runId, isCompleted)).result.current;
}

describe("useRunFromRoute", () => {
  it("says nothing is selected when the route carries no id", () => {
    expect(resolve(undefined, [completed]).state).toBe("none");
  });

  it("resolves an eligible id", () => {
    const r = resolve("run-0001", [completed, running]);
    expect(r.state).toBe("selected");
    expect(r.run).toBe(completed);
  });

  it("distinguishes a run that exists but is not eligible", () => {
    const r = resolve("run-0002", [completed, running]);
    expect(r.state).toBe("ineligible");
    expect(r.run).toBeNull();
  });

  it("calls an id unknown once the list has arrived without it", () => {
    expect(resolve("run-0009", [completed]).state).toBe("unknown");
  });

  it("calls nothing unknown before the list has arrived", () => {
    // The whole point of deciding on `data !== undefined` rather than on
    // `runs.length`: a deep link would otherwise flash a warning at a perfectly
    // good URL on every cold load.
    expect(resolve("run-0001", undefined).state).toBe("none");
  });

  it("calls an id unknown against an empty list, which is an answer", () => {
    expect(resolve("run-0001", []).state).toBe("unknown");
  });

  it("filters the list it offers, and keeps the unfiltered one", () => {
    const r = resolve("run-0001", [completed, running]);
    expect(r.eligibleRuns).toEqual([completed]);
    expect(r.runs).toEqual([completed, running]);
  });

  it("offers every run when no predicate is given", () => {
    runsState.data = [completed, running];
    const { result } = renderHook(() => useRunFromRoute("run-0002"));
    expect(result.current.state).toBe("selected");
    expect(result.current.eligibleRuns).toEqual([completed, running]);
  });
});
