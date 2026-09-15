import { createElement, type ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ModelSaveResult, RunSpec } from "./backend";
import type { RunLog, RunSummary } from "./client";
import { buildCreateRunRequest } from "./http-backend";
import {
  useDeleteRun,
  useIsSavingModel,
  useRunLog,
  useRuns,
  useSaveModel,
} from "./hooks";
import { queryKeys } from "./query-keys";
import { queryClient } from "./query-client";

const saveState = vi.hoisted(() => ({
  respond: (): Promise<ModelSaveResult> =>
    Promise.resolve({ featureCount: 0, warnings: [], hash: null }),
}));

const backendState = vi.hoisted(() => ({
  runsChangeExternally: true,
  runs: [] as RunSummary[],
  log: { run_id: "", lines: [] } as RunLog,
}));

const getRuns = vi.hoisted(() =>
  vi.fn(() => Promise.resolve(backendState.runs)),
);
const getRunLog = vi.hoisted(() =>
  vi.fn<(runId: string) => Promise<RunLog>>(() =>
    Promise.resolve(backendState.log),
  ),
);

vi.mock("./backend", () => ({
  backend: {
    capabilities: {
      kind: "http",
      canExport: false,
      runsAgainstSavedModel: true,
      get runsChangeExternally() {
        return backendState.runsChangeExternally;
      },
    },
    saveModel: () => saveState.respond(),
    getProjectStatus: () => Promise.resolve(null),
    getRuns,
    getRunLog,
    deleteRun: (runId: string) => Promise.resolve({ runId, retainedPaths: [] }),
  },
}));

function wrapper({ children }: { children: ReactNode }) {
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

function runWithStatus(status: RunSummary["status"]): RunSummary {
  return {
    id: `run-${status}`,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    status,
    started_at: "2026-01-01T00:00:00Z",
    finished_at: "",
    log_path: "",
    artifacts: [],
  };
}

/**
 * Advances fake time and lets the queries settle in between. TanStack hands
 * React its notifications through `setTimeout(cb, 0)`, and fake-timers gives
 * a zero-delay timer created during a tick a 1 ms delay, so one scheduled by
 * a poll firing at the very end of the window needs one more millisecond.
 */
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
    await vi.advanceTimersByTimeAsync(1);
  });
}

const baseSpec: RunSpec = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: {},
  receiverMode: "auto-grid",
};

/**
 * The request body is the contract the API gates on: a scaffold-tier standard
 * is refused unless `experimental` is present, and a standard that needs no
 * acknowledgement must not carry one.
 */
describe("buildCreateRunRequest", () => {
  it("carries the acknowledgement for a scaffold-tier run", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      standardId: "cnossos-road",
      experimental: true,
    });

    expect(request.experimental).toBe(true);
    expect(request.standard_id).toBe("cnossos-road");
  });

  it("omits the field entirely when nothing was acknowledged", () => {
    const request = buildCreateRunRequest(baseSpec);

    expect(request).not.toHaveProperty("experimental");
    expect(Object.keys(request)).not.toContain("experimental");
  });

  it("omits the field rather than sending false", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      experimental: false,
    });

    expect(request).not.toHaveProperty("experimental");
  });

  it("maps the rest of the spec onto the API field names", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      profile: "strict",
      params: { temperature_c: "10" },
      receiverMode: "custom",
    });

    expect(request).toMatchObject({
      standard_id: "rls19-road",
      standard_version: "2019",
      standard_profile: "strict",
      receiver_mode: "custom",
      params: { temperature_c: "10" },
    });
  });
});

/**
 * `useIsSavingModel` reads the mutation cache by key rather than the state of
 * one `useSaveModel` instance, so every surface sees a save any surface
 * started. This runs both under the app's own query client, the way the
 * app does.
 */
describe("useIsSavingModel", () => {
  beforeEach(() => {
    queryClient.clear();
  });

  it("is true while a save is pending and false after it settles", async () => {
    let resolve: (() => void) | undefined;
    saveState.respond = () =>
      new Promise((r) => {
        resolve = () => {
          r({ featureCount: 1, warnings: [], hash: null });
        };
      });

    const save = renderHook(() => useSaveModel(), { wrapper });
    const observer = renderHook(() => useIsSavingModel(), { wrapper });
    expect(observer.result.current).toBe(false);

    let done: Promise<ModelSaveResult> | undefined;
    act(() => {
      done = save.result.current.mutateAsync({
        model: { type: "FeatureCollection", features: [] },
      });
    });
    await waitFor(() => {
      expect(observer.result.current).toBe(true);
    });

    await act(async () => {
      resolve?.();
      await done;
    });
    await waitFor(() => {
      expect(observer.result.current).toBe(false);
    });
  });
});

/**
 * Runs are polled rather than pushed (see the note in `hooks.ts`), so the
 * interval is what keeps a run started from the CLI or another tab visible.
 * The assertions count calls on the mocked backend rather than reading the
 * query options: what matters is that a fetch actually happens.
 */
describe("useRuns", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    queryClient.clear();
    getRuns.mockClear();
    backendState.runsChangeExternally = true;
    backendState.runs = [];
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("polls every 2 s while a run is running", async () => {
    backendState.runs = [runWithStatus("running"), runWithStatus("completed")];

    const hook = renderHook(() => useRuns(), { wrapper });
    await advance(0);
    expect(getRuns).toHaveBeenCalledTimes(1);
    expect(hook.result.current.data).toHaveLength(2);

    await advance(2_000);
    expect(getRuns).toHaveBeenCalledTimes(2);
    await advance(2_000);
    expect(getRuns).toHaveBeenCalledTimes(3);
  });

  it("polls every 15 s, not every 2 s, while every run is settled", async () => {
    backendState.runs = [runWithStatus("completed"), runWithStatus("failed")];

    renderHook(() => useRuns(), { wrapper });
    await advance(0);
    expect(getRuns).toHaveBeenCalledTimes(1);

    await advance(2_000);
    expect(getRuns).toHaveBeenCalledTimes(1);
    await advance(13_000);
    expect(getRuns).toHaveBeenCalledTimes(2);
  });

  it("speeds up once a run appears that is still running", async () => {
    backendState.runs = [runWithStatus("completed")];

    renderHook(() => useRuns(), { wrapper });
    await advance(0);
    expect(getRuns).toHaveBeenCalledTimes(1);

    // Started elsewhere between polls; the idle poll picks it up.
    backendState.runs = [runWithStatus("completed"), runWithStatus("pending")];
    await advance(15_000);
    expect(getRuns).toHaveBeenCalledTimes(2);
    await advance(2_000);
    expect(getRuns).toHaveBeenCalledTimes(3);
  });

  it("does not poll in browser mode, where a run completes inside startRun", async () => {
    backendState.runsChangeExternally = false;
    backendState.runs = [runWithStatus("running")];

    renderHook(() => useRuns(), { wrapper });
    await advance(0);
    expect(getRuns).toHaveBeenCalledTimes(1);

    await advance(30_000);
    expect(getRuns).toHaveBeenCalledTimes(1);
  });
});

describe("useRunLog", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    queryClient.clear();
    getRuns.mockClear();
    getRunLog.mockClear();
    backendState.runsChangeExternally = true;
    backendState.runs = [{ ...runWithStatus("running"), id: "run-1" }];
    backendState.log = { run_id: "run-1", lines: ["started"] };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  /** The runs list is what carries a run from running to settled. */
  function settleRun1() {
    backendState.runs = [{ ...runWithStatus("completed"), id: "run-1" }];
    backendState.log = { run_id: "run-1", lines: ["started", "done"] };
  }

  it("polls while the run is running", async () => {
    renderHook(() => useRunLog("run-1", true), { wrapper });
    await advance(0);
    expect(getRunLog).toHaveBeenCalledTimes(1);

    await advance(2_000);
    expect(getRunLog).toHaveBeenCalledTimes(3);
    expect(getRunLog).toHaveBeenLastCalledWith("run-1");
  });

  it("fetches once more after the run completes, then stops", async () => {
    // Both hooks, the way `RunDetail` under `RunPage` has them: the status
    // that stops the log poll is the same one `useRuns` sees settle.
    const hook = renderHook(
      ({ isRunning }: { isRunning: boolean }) => {
        useRuns();
        return useRunLog("run-1", isRunning);
      },
      { wrapper, initialProps: { isRunning: true } },
    );
    await advance(1_000);
    expect(getRunLog).toHaveBeenCalledTimes(2);

    // The final lines land between the last poll and the status flip.
    settleRun1();
    hook.rerender({ isRunning: false });
    await advance(1_000); // the runs poll at 2 s picks up the settled run
    expect(getRunLog).toHaveBeenCalledTimes(3);
    expect(hook.result.current.data?.lines).toEqual(["started", "done"]);

    await advance(30_000);
    expect(getRunLog).toHaveBeenCalledTimes(3);
  });

  it("refetches a log cached while running when it is mounted again after the run settled", async () => {
    renderHook(() => useRuns(), { wrapper });
    const detail = renderHook(() => useRunLog("run-1", true), { wrapper });
    await advance(0);
    expect(getRunLog).toHaveBeenCalledTimes(1);

    // The user looks elsewhere while the run finishes.
    detail.unmount();
    settleRun1();
    await advance(2_000);
    expect(getRunLog).toHaveBeenCalledTimes(1);

    // Back within `staleTime`: the cached log is truncated and must not be
    // served as final.
    const again = renderHook(() => useRunLog("run-1", false), { wrapper });
    await advance(0);
    expect(getRunLog).toHaveBeenCalledTimes(2);
    expect(again.result.current.data?.lines).toEqual(["started", "done"]);

    await advance(30_000);
    expect(getRunLog).toHaveBeenCalledTimes(2);
  });

  it("does not poll a run that is not running", async () => {
    renderHook(() => useRunLog("run-1", false), { wrapper });
    await advance(0);
    expect(getRunLog).toHaveBeenCalledTimes(1);

    await advance(30_000);
    expect(getRunLog).toHaveBeenCalledTimes(1);
  });
});

describe("useDeleteRun", () => {
  it("drops the deleted run's cached payloads and keeps everyone else's", async () => {
    // The artifact ids travel with the request because `runId` alone cannot
    // address `queryKeys.artifacts.content(...)`, and these are the largest
    // things in the cache: a receiver table and a raster for a run that no
    // longer exists are bytes nothing can ever ask for again.
    queryClient.setQueryData(queryKeys.runs.log("run-1"), {
      run_id: "run-1",
      lines: ["done"],
    });
    queryClient.setQueryData(queryKeys.artifacts.content("art-1"), "receivers");
    queryClient.setQueryData(queryKeys.artifacts.content("art-2"), "raster");
    // Another run's artifact, which this deletion has no business touching.
    queryClient.setQueryData(queryKeys.artifacts.content("art-9"), "other run");

    const { result } = renderHook(() => useDeleteRun(), { wrapper });
    act(() => {
      result.current.mutate({
        runId: "run-1",
        artifactIds: ["art-1", "art-2"],
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(
      queryClient.getQueryData(queryKeys.runs.log("run-1")),
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(queryKeys.artifacts.content("art-1")),
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(queryKeys.artifacts.content("art-2")),
    ).toBeUndefined();
    expect(queryClient.getQueryData(queryKeys.artifacts.content("art-9"))).toBe(
      "other run",
    );
  });
});
