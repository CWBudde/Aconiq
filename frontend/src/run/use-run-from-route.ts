import { useMemo } from "react";
import type { RunSummary } from "@/api/client";
import { useRuns } from "@/api/hooks";

/**
 * What the route asked for, and whether it could be honoured.
 *
 * - `none` — no run in the URL. The page shows its list and nothing else.
 * - `selected` — `run` is the one to show.
 * - `ineligible` — the run exists but this page cannot show it (a run that has
 *   not completed has no results). Not the same as unknown, and saying
 *   "unknown" about a run the user can see in the list is a lie.
 * - `unknown` — the list has arrived and holds no such run.
 */
export type RunRouteState = "none" | "selected" | "ineligible" | "unknown";

export interface RunFromRoute {
  runs: RunSummary[];
  /** The runs this page will show, in backend order. */
  eligibleRuns: RunSummary[];
  run: RunSummary | null;
  state: RunRouteState;
  isLoading: boolean;
  error: Error | null;
}

/**
 * Resolves the run id in the path against the run list.
 *
 * **There is deliberately no fallback to the first run.** The list arrives in
 * backend order, so "the first one" was never "the newest one", and putting
 * someone else's numbers under a heading the user did not ask for is worse than
 * showing none.
 *
 * `eligible` is a parameter rather than a fixed rule because the two callers
 * genuinely differ: `/results` shows completed runs only, while `/export`
 * resolves against every run, since its dialog offers every run and a bundle
 * generated for one not yet in the filtered list must not read as unknown.
 *
 * The unknown verdict is decided on `data !== undefined`, never on
 * `runs.length`: an id is unknown only once a list has actually arrived, or a
 * deep link would flash a warning at a good URL on every cold load.
 *
 * The caller reads `runId` from the router itself and passes it in, so this
 * stays a question about data.
 */
export function useRunFromRoute(
  runId: string | undefined,
  eligible?: (run: RunSummary) => boolean,
): RunFromRoute {
  const { data, isLoading, error } = useRuns();
  const runs = useMemo(() => data ?? [], [data]);
  const eligibleRuns = useMemo(
    () => (eligible ? runs.filter(eligible) : runs),
    [runs, eligible],
  );

  const run =
    runId == null ? null : (eligibleRuns.find((r) => r.id === runId) ?? null);

  let state: RunRouteState;
  if (runId == null) {
    state = "none";
  } else if (run !== null) {
    state = "selected";
  } else if (data === undefined) {
    // Still loading. The page renders its transient state; nothing is unknown
    // yet.
    state = "none";
  } else if (runs.some((r) => r.id === runId)) {
    state = "ineligible";
  } else {
    state = "unknown";
  }

  return { runs, eligibleRuns, run, state, isLoading, error };
}
