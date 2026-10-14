import { backend } from "./backend";
import type {
  ProjectStatusEvent,
  RunProgressReport,
  RunSummary,
} from "./client";
import { queryClient } from "./query-client";
import { queryKeys } from "./query-keys";

/**
 * The one connection to the project's event stream, shared by every runs list
 * on screen, and what it does to the query cache.
 *
 * The stream's `project_status` event carries the last run's status and
 * progress, and the server sends one whenever either moves — at most every
 * couple of seconds. That is everything the progress bar of a running run
 * reads, so while the stream is up the runs list does not have to poll for it:
 * the summary in the cache is patched in place, and the list falls back to the
 * idle poll (`useRuns`). Anything the event cannot say — a new run, a status
 * that moved, which brings artifacts and a finish time — invalidates the list,
 * and the next fetch says it.
 *
 * The connection is held only while the tab is visible. A browser allows six
 * HTTP/1.1 connections per host across all of its tabs, `aconiq serve` speaks
 * nothing newer, and a stream never ends: six tabs on a run page would hold
 * every slot, and every other request to the API — from any of them — would
 * queue behind streams that never finish. A hidden tab gives its connection
 * back and the poll covers it; showing the tab opens the stream again.
 *
 * Only the runs list is written. The status document the event carries is
 * also what `useProjectStatus` caches, but the model sync and hydration read
 * that one and decide things off it; feeding it every two seconds of a run is
 * a change of its own, not a side effect of this one.
 */

let subscribers = 0;
let unsubscribe: (() => void) | null = null;
/** Bumped per connection, so a late callback from a closed one is ignored. */
let generation = 0;
let connected = false;
let lastRunId: string | null = null;

/**
 * The id of the run the stream is carrying, or `null` while the stream is down
 * or has not said yet. A running run with this id needs no fast poll.
 */
export function pushedRunId(): string | null {
  return connected ? lastRunId : null;
}

/**
 * Keep the stream open while the returned function has not been called and
 * the tab is visible. The first caller opens it and the last one to release
 * closes it, so two lists on screen share one connection.
 */
export function acquireProjectEvents(): () => void {
  if (!backend.capabilities.streamsProjectEvents) return () => undefined;

  subscribers++;
  if (subscribers === 1) {
    document.addEventListener("visibilitychange", syncConnection);
  }
  syncConnection();

  let released = false;
  return () => {
    if (released) return;
    released = true;
    subscribers--;
    if (subscribers === 0) {
      document.removeEventListener("visibilitychange", syncConnection);
    }
    syncConnection();
  };
}

/** Open or close the stream to match the subscribers and the tab's visibility. */
function syncConnection() {
  const wanted = subscribers > 0 && document.visibilityState !== "hidden";
  if (wanted && unsubscribe === null) open();
  else if (!wanted && unsubscribe !== null) {
    close();
    // As for a dropped stream: the list may be on the idle poll because the
    // stream was carrying its running run.
    if (subscribers > 0) invalidateRuns();
  }
}

/**
 * Refetch the runs list, without cancelling a fetch already under way: events
 * can arrive every couple of seconds, and cancelling each time would keep a
 * slow list from ever answering.
 */
function invalidateRuns() {
  void queryClient.invalidateQueries(
    { queryKey: queryKeys.runs.list() },
    { cancelRefetch: false },
  );
}

function open() {
  const mine = ++generation;
  unsubscribe = backend.subscribeProjectEvents({
    onProjectStatus(event) {
      if (mine !== generation) return;
      applyProjectStatusEvent(event);
    },
    onConnectionChange(next) {
      if (mine !== generation) return;
      connected = next;
      if (!next) {
        lastRunId = null;
        // The list may be on the idle poll because the stream was carrying
        // its running run. Refetching now is what puts it back on the fast
        // one at once — its interval is only re-read when its data changes.
        invalidateRuns();
      }
    },
  });
}

function close() {
  generation++;
  unsubscribe?.();
  unsubscribe = null;
  connected = false;
  lastRunId = null;
}

/** Apply one `project_status` event to the runs list in the cache. */
export function applyProjectStatusEvent(event: ProjectStatusEvent): void {
  if (!event.project_available) return;

  const lastRun = event.project.last_run;
  lastRunId = lastRun?.id ?? null;
  if (lastRun === undefined) return;

  const runs = queryClient.getQueryData<RunSummary[]>(queryKeys.runs.list());
  // Nobody holds the list, so there is nothing to keep current.
  if (runs === undefined) return;

  const known = runs.find((run) => run.id === lastRun.id);
  if (known?.status !== lastRun.status) {
    invalidateRuns();
    return;
  }

  if (sameProgress(known.progress, lastRun.progress)) return;

  queryClient.setQueryData<RunSummary[]>(
    queryKeys.runs.list(),
    runs.map((run) =>
      run.id === lastRun.id ? withProgress(run, lastRun.progress) : run,
    ),
  );
}

function sameProgress(
  a: RunProgressReport | undefined,
  b: RunProgressReport | undefined,
): boolean {
  if (a === undefined || b === undefined) return a === b;
  return (
    a.done === b.done &&
    a.total === b.total &&
    a.updated_at === b.updated_at &&
    a.eta_seconds === b.eta_seconds
  );
}

function withProgress(
  run: RunSummary,
  progress: RunProgressReport | undefined,
): RunSummary {
  if (progress !== undefined) return { ...run, progress };
  // Absent, not undefined: the run reported none (`exactOptionalPropertyTypes`).
  const next = { ...run };
  delete next.progress;
  return next;
}
