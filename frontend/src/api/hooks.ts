import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useIsMutating, useMutation, useQuery } from "@tanstack/react-query";
import type { Query } from "@tanstack/react-query";
import { backend, isRunCancelled } from "./backend";
import type { ContourOptions, OsmImportRequest, RunSpec } from "./backend";
import type {
  ModelSaveRequest,
  RasterMetadata,
  ReceiverTable,
  RunSummary,
} from "./client";
import { queryKeys } from "./query-keys";
import { queryClient } from "./query-client";
import { withLegacyUnits } from "@/map/result-units";

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health.all,
    queryFn: () => backend.getHealth(),
    staleTime: 60_000,
  });
}

export function useProjectStatus() {
  return useQuery({
    queryKey: queryKeys.project.status(),
    queryFn: () => backend.getProjectStatus(),
  });
}

export function useStandards() {
  return useQuery({
    queryKey: queryKeys.standards.all,
    queryFn: () => backend.getStandards(),
    staleTime: 5 * 60_000,
  });
}

/*
 * Runs and their logs are polled, not pushed. The API's SSE stream
 * (`/api/v1/events`) carries project status snapshots, not run logs, and a
 * browser `EventSource` cannot send the `--api-token` header, so the stream is
 * unreachable whenever a token is set (PLAN.md, Priority 6). Polling by
 * activity keeps the cost near zero while nothing is running.
 */

/** Runs-list poll interval while any run is pending or running. */
export const ACTIVE_RUNS_POLL_MS = 2_000;
/**
 * Runs-list poll interval while every run is settled. Slow, but not off: a
 * run started from the CLI or another tab still shows up while a runs list
 * is on screen.
 */
export const IDLE_RUNS_POLL_MS = 15_000;
/** Poll interval for the log of a run that is still running. */
export const RUN_LOG_POLL_MS = 1_000;

function isRunActive(run: RunSummary): boolean {
  return run.status === "pending" || run.status === "running";
}

/** Needs nothing from the render, so it lives outside the hook. */
function runsRefetchInterval(query: Query<RunSummary[]>): number {
  const runs = query.state.data ?? [];
  return runs.some(isRunActive) ? ACTIVE_RUNS_POLL_MS : IDLE_RUNS_POLL_MS;
}

export function useRuns() {
  const query = useQuery({
    queryKey: queryKeys.runs.list(),
    queryFn: () => backend.getRuns(),
    refetchInterval: backend.capabilities.runsChangeExternally
      ? runsRefetchInterval
      : false,
  });

  // A run's log is fetched once more when the run settles, so the lines
  // written between the last log poll and completion arrive. The transition
  // is detected here, where the status data lives, rather than in
  // `useRunLog`: a detail panel may be unmounted while the run completes, and
  // invalidating the cache entry marks it stale so the next mount refetches
  // regardless of `staleTime`, while an active observer refetches at once.
  const runs = query.data;
  const previousRuns = useRef(runs);
  useEffect(() => {
    const was = previousRuns.current;
    previousRuns.current = runs;
    if (!was || !runs) return;
    const wasActive = new Set(was.filter(isRunActive).map((run) => run.id));
    for (const run of runs) {
      if (wasActive.has(run.id) && !isRunActive(run)) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.runs.log(run.id),
        });
      }
    }
  }, [runs]);

  return query;
}

/**
 * @param isRunning Whether the run is still being executed by the server.
 *   While set, the log polls and a remount fetches immediately. The one
 *   extra fetch after the run settles is driven by `useRuns`, which sees the
 *   status change. No capability check is needed here: in browser mode a run
 *   completes inside `startRun`, so a caller never observes a running one
 *   and passes `false` throughout.
 */
export function useRunLog(runId: string | null, isRunning: boolean) {
  return useQuery({
    queryKey: queryKeys.runs.log(runId ?? ""),
    queryFn: () => {
      if (!runId) throw new Error("Run ID is required");
      return backend.getRunLog(runId);
    },
    enabled: runId !== null,
    refetchInterval: isRunning ? RUN_LOG_POLL_MS : false,
    staleTime: isRunning ? 0 : 30_000,
  });
}

export function useArtifactContent<T>(artifactId: string | null) {
  return useQuery({
    queryKey: queryKeys.artifacts.content(artifactId ?? ""),
    queryFn: () => {
      if (!artifactId) throw new Error("Artifact ID is required");
      return backend.getArtifactContent<T>(artifactId);
    },
    enabled: artifactId !== null,
    staleTime: 5 * 60_000,
  });
}

/**
 * The raw bytes of a binary artifact — the result raster the map draws.
 *
 * `staleTime: Infinity` because these bytes are a file inside a finished run:
 * nothing rewrites them, so a refetch could only return what is already held.
 * `gcTime` is bounded all the same, and deliberately not infinite — the entry
 * is megabytes, and once the user has switched away from the run there is no
 * one left to hold them for.
 */
export function useArtifactBytes(artifactId: string | null) {
  return useQuery({
    queryKey: queryKeys.artifacts.bytes(artifactId ?? ""),
    queryFn: () => {
      if (!artifactId) throw new Error("Artifact ID is required");
      return backend.getArtifactBytes(artifactId);
    },
    enabled: artifactId !== null,
    staleTime: Infinity,
    gcTime: 5 * 60_000,
  });
}

/**
 * A run's contours, traced by the same Go marching squares in both modes.
 *
 * `staleTime: Infinity` for the reason `useArtifactBytes` has it: the raster
 * behind this is a file in a finished run, so a refetch could only return what
 * is already held. `gcTime` is bounded all the same — a dense grid is a lot of
 * vertices to hold for a run the user has switched away from.
 *
 * `null` disables it, which is how a caller says "this run has no raster" or
 * "the panel has no CRS to ask in" without the hook having to know either.
 */
export function useRunContours(
  runId: string | null,
  options: ContourOptions | null,
) {
  return useQuery({
    queryKey: queryKeys.contours.forRun(
      runId ?? "",
      options?.crs ?? "",
      options?.interval,
    ),
    queryFn: () => {
      if (!runId || !options) throw new Error("A run and a CRS are required");
      return backend.getRunContours(runId, options);
    },
    enabled: runId !== null && options !== null,
    staleTime: Infinity,
    gcTime: 5 * 60_000,
    // A run whose receivers were not a grid refuses every time, in the same
    // words. Retrying spends three round trips to be told so three times.
    retry: false,
  });
}

// Both of these read an artifact that may predate the per-channel unit, and
// `useArtifactContent` is a type assertion over raw JSON — it converts
// nothing. `withLegacyUnits` is where the old scalar is expanded, mirroring
// what Go does in `LoadReceiverTableJSON` and `RasterMetadata.UnmarshalJSON`.
// Applied here rather than at each call site so a new consumer cannot forget.
export function useReceiverTable(artifactId: string | null) {
  const query = useArtifactContent<ReceiverTable>(artifactId);
  const { data } = query;

  // Memoised on the fetched value. `withLegacyUnits` returns its argument
  // untouched for a current container, but builds a new object for a legacy
  // one — and react-query hands back a stable reference, so without this an
  // old run would produce a fresh table every render and re-run every memo
  // and effect keyed on it, the raster's colourising among them.
  const withUnits = useMemo(
    () =>
      data === undefined
        ? undefined
        : withLegacyUnits(data, data.indicator_order),
    [data],
  );

  return { ...query, data: withUnits };
}

export function useRasterMetadata(artifactId: string | null) {
  const query = useArtifactContent<RasterMetadata>(artifactId);
  const { data } = query;

  const withUnits = useMemo(
    () =>
      data === undefined ? undefined : withLegacyUnits(data, data.band_names),
    [data],
  );

  return { ...query, data: withUnits };
}

export function useImportFromOSM() {
  return useMutation({
    mutationFn: (req: OsmImportRequest) => backend.importFromOSM(req),
  });
}

/**
 * Telling a cancelled run apart from a failed one, re-exported so the UI never
 * imports from `@/wasm/`. See `backend.ts`.
 */
export { isRunCancelled } from "./backend";

/** How far a run has got, as the backend reported it last. */
export interface RunProgress {
  /** Receivers computed so far. */
  done: number;
  /** Receivers this run will compute in total. */
  total: number;
}

export function useCreateRun() {
  // The bar's state, not the mutation's: react-query holds a result, and
  // progress is not one — it changes many times per run and is gone once the
  // run settles. Held here so the dialog gets it without a store of its own.
  const [progress, setProgress] = useState<RunProgress | null>(null);
  // The controller of the run in flight, if this mode can stop one. A ref
  // rather than state: nothing renders differently for holding it, and the
  // Cancel button must reach *this* run's controller, not the one a render
  // captured.
  const abort = useRef<AbortController | null>(null);

  const mutation = useMutation({
    // A refusal is thrown whole: the run dialog reads `code` and `hint` off
    // the envelope to explain one the user can act on.
    mutationFn: (spec: RunSpec) => {
      // Cleared here rather than on settle, so the panel of the previous run
      // cannot be what the next one starts from.
      setProgress(null);
      // Only where aborting means something. A signal handed to a backend
      // that ignores it would let `cancel()` report a cancellation that never
      // happened while the run went on regardless.
      const controller = backend.capabilities.runsAreCancellable
        ? new AbortController()
        : null;
      abort.current = controller;
      return backend.startRun(spec, {
        onProgress: (done, total) => {
          setProgress({ done, total });
        },
        ...(controller ? { signal: controller.signal } : {}),
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
    onSettled: () => {
      // A settled run's controller aborts nothing, and holding it would let a
      // late Cancel tear down the kernel a *later* run is using.
      abort.current = null;
    },
  });

  /**
   * Stop the run in flight. A no-op where nothing is running, and where the
   * mode cannot stop one — `mutationFn` allocates no controller there, which
   * is the same fact stated once rather than twice.
   */
  const cancel = useCallback(() => {
    abort.current?.abort();
  }, []);

  return {
    ...mutation,
    progress,
    cancel,
    /**
     * Derived from the failure rather than from the button press: pressing
     * Cancel asks, and only the rejection that follows says the run actually
     * stopped. A flag set on the press would claim a cancellation for a run
     * that finished in the meantime.
     */
    cancelled: mutation.isError && isRunCancelled(mutation.error),
  };
}

export function useCreateExport() {
  return useMutation({
    mutationFn: (runId: string) => backend.createExport(runId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
    },
  });
}

/**
 * What `useDeleteRun` needs to clear up after itself.
 *
 * The artifact ids travel with the request rather than being looked up here:
 * artifact payloads are cached under `queryKeys.artifacts.content(id)` and
 * `.bytes(id)`, which `runId` alone cannot address, and by the time the
 * deletion has succeeded the run that listed them is gone. The caller holds
 * `run.artifacts` already.
 */
export interface DeleteRunVariables {
  runId: string;
  /** `run.artifacts.map((a) => a.id)` — empty for a run that produced none. */
  artifactIds: string[];
}

export function useDeleteRun() {
  return useMutation({
    mutationFn: ({ runId }: DeleteRunVariables) => backend.deleteRun(runId),
    onSuccess: async (_result, { runId, artifactIds }) => {
      // The run's own cached log and artifact payloads are removed rather than
      // invalidated: there is nothing left to refetch, and the artifact
      // entries are the larger objects.
      queryClient.removeQueries({ queryKey: queryKeys.runs.log(runId) });
      for (const artifactId of artifactIds) {
        queryClient.removeQueries({
          queryKey: queryKeys.artifacts.content(artifactId),
        });
        // Both keys, because the raster is cached under `bytes` and would
        // otherwise outlive the run it belongs to for the whole `gcTime` —
        // megabytes held for something that no longer exists.
        queryClient.removeQueries({
          queryKey: queryKeys.artifacts.bytes(artifactId),
        });
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
      // `ProjectStatusResponse.run_count` changed, exactly as it does when a
      // run is created.
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

/**
 * Keyed so that "is a save in flight?" can be asked from anywhere
 * (`useIsSavingModel`), not only by the component that started it.
 */
export const MODEL_SAVE_MUTATION_KEY = ["model", "save"] as const;

export function useSaveModel() {
  return useMutation({
    mutationKey: MODEL_SAVE_MUTATION_KEY,
    mutationFn: (req: ModelSaveRequest) => backend.saveModel(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

/** True while any `useSaveModel` mutation is running, whichever surface started it. */
export function useIsSavingModel(): boolean {
  return (
    useIsMutating({ mutationKey: MODEL_SAVE_MUTATION_KEY }, queryClient) > 0
  );
}

export function getArtifactContentURL(artifactId: string): string {
  return backend.getArtifactURL(artifactId);
}
