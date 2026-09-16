/**
 * Read the workspace back from the project on startup.
 *
 * Without this the frontend only ever wrote: a reload showed an empty map over
 * a populated project, and the only thing standing between a user and losing
 * their work to that empty map was the draft in localStorage. It lives beside
 * `use-project-sync.ts` for the same reason that one does — "does the
 * workspace differ from the project?" is a model question, and this is the
 * other half of the answer.
 *
 * Accepted consequence: hydration rebuilds the payload in store order, so the
 * first save after hydrating a model that `aconiq import` wrote may produce
 * different bytes and therefore a different hash. Nothing breaks — the client
 * never computes a hash, it only keeps the receipt the POST hands it — but a
 * reader wondering why a round trip changed the hash should find the answer
 * here rather than suspect the hash.
 */

import { useEffect } from "react";
import { create } from "zustand";
import { backend } from "@/api/backend";
import { useProjectStatus } from "@/api/hooks";
import { useModelStore } from "@/model/model-store";
import { normalizeModelGeoJSON } from "@/model/normalize";
import { hasDraft, loadDraft } from "@/model/use-autosave";

/**
 * The map draws in WGS84, so that is what a hydrated model has to arrive in.
 * `GET /api/v1/model` answers in the project CRS when asked for nothing —
 * metres in EPSG:25832 for a typical German project — which the map would
 * draw a few hundred kilometres off the coast of Africa.
 */
const MODEL_CRS = "EPSG:4326";

export type HydrationStatus = "idle" | "fetching" | "settled" | "error";

interface ProjectHydrationState {
  /**
   * Hydration has run and will not run again. Set synchronously, before any
   * `await`, so a StrictMode double-mount, a remount and HMR all see it —
   * which is also why it lives here rather than in a `useRef`, and why the
   * fetch is imperative rather than a `useQuery`: a query refetches on focus
   * and on remount, and would fight one-shot semantics rather than implement
   * them.
   *
   * It is the one-shot guard and not the settled signal — those parted company
   * when a failed project status stopped counting as an answer. Read
   * `useHydrationSettled()` to ask whether startup is finished.
   */
  started: boolean;
  status: HydrationStatus;
  /**
   * How many times the user has asked for a retry.
   *
   * Re-arming the hook is not enough on its own when it was the project-status
   * request that failed: that query is React Query's, it is sitting in its own
   * error state, and nothing would ask it again. The counter is what the hook
   * watches to refetch it.
   */
  attempt: number;
  /** The rejection to show the user; `null` unless `status === "error"`. */
  error: Error | null;
  /**
   * Whether the saved draft should be offered for restore.
   *
   * This rule used to live in `DraftBanner`'s own mount effect as
   * `features.length === 0 && receivers.length === 0 && hasDraft()`. It cannot
   * stay there: child effects run before parent effects, so the banner decided
   * before this hook did, and after a hash-mismatch fetch the store is no
   * longer empty — the divergent draft, which is exactly the one a user needs
   * offered, would never be. Deciding it here, from the store's state at
   * module init, is what keeps the offer honest.
   *
   * The banner shows it only once `started` is set, so a draft that turns out
   * to be the project's own model never flashes an offer to restore it.
   */
  draftOffered: boolean;
}

function initialState(): ProjectHydrationState {
  const model = useModelStore.getState();
  return {
    started: false,
    status: "idle",
    attempt: 0,
    error: null,
    draftOffered:
      model.features.length === 0 && model.receivers.length === 0 && hasDraft(),
  };
}

/**
 * Module-level, not per hook instance: the one-shot guard has to survive a
 * remount, and the draft offer is read by a component that does not mount the
 * hook. Exported for tests to reset between cases.
 */
export const projectHydrationStore = create<ProjectHydrationState>(() =>
  initialState(),
);

/** Back to "startup has not happened yet". For test isolation. */
export function resetProjectHydration(): void {
  projectHydrationStore.setState(initialState());
}

/**
 * Let the user retry a hydration that was refused. Clearing `started` is what
 * re-arms the hook; the effect depends on it, so the next render runs the
 * decision again.
 */
export function retryProjectHydration(): void {
  projectHydrationStore.setState((state) => ({
    started: false,
    status: "idle",
    attempt: state.attempt + 1,
    error: null,
  }));
}

/**
 * Whether startup has finished asking whether the project holds a model.
 *
 * "Finished" includes having failed: an error is an answer, and the surfaces
 * waiting on this one — the draft offer, and the routes themselves — must not
 * wait forever on a backend that is down. The one thing they may not do is act
 * while the question is still open.
 */
export function useHydrationSettled(): boolean {
  const status = projectHydrationStore((s) => s.status);

  return status === "settled" || status === "error";
}

/** Everything is decided and nothing is left to fetch. */
function settle(): void {
  projectHydrationStore.setState({
    started: true,
    status: "settled",
    error: null,
  });
}

function asError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}

/** The workspace holds something the project has not been told about. */
function hasUnsavedWork(model: ReturnType<typeof useModelStore.getState>) {
  return (
    model.dirty ||
    model.features.length > 0 ||
    model.receivers.length > 0 ||
    model.calcArea !== null
  );
}

/**
 * Mount once, in `RootLayout` rather than on a page: a reload can land on any
 * route, and every route draws the same model.
 */
export function useProjectHydration(): void {
  const project = useProjectStatus();
  const started = projectHydrationStore((s) => s.started);
  const attempt = projectHydrationStore((s) => s.attempt);
  const statusFailed = project.isError;
  const refetchStatus = project.refetch;

  // A retry asks the project-status query again, because that is the request
  // that failed whenever the branch below left `started` unset. Re-arming the
  // hook alone would change nothing: the query would still be serving the same
  // cached rejection, so the effect would take the same decision again.
  useEffect(() => {
    if (attempt === 0 || !statusFailed) return;
    void refetchStatus();
  }, [attempt, statusFailed, refetchStatus]);

  useEffect(() => {
    if (started) return;
    // Not the mode string: `runsAgainstSavedModel` already means "the project
    // holds the model a run reads", which is the same fact, and it is the
    // flag every other surface branches on. Browser mode never fetches — the
    // store is the project there — but the decision is still made, because
    // the draft banner waits for it.
    if (!backend.capabilities.runsAgainstSavedModel) {
      settle();
      return;
    }
    // Wait for the status rather than deciding without it; a decision taken
    // while it is still loading would be taken on `data === undefined`, which
    // is indistinguishable here from "no project".
    if (project.isLoading) return;

    // An errored status is not an answer of "no model", it is no answer at
    // all, and the two must not collapse into each other. `started` stays
    // unset on purpose: a refetch that succeeds later — after a reconnect, a
    // window focus, or the Retry button — finds the hook still armed and
    // hydrates after all. Settling here instead would leave an empty map over
    // a populated project for the rest of the session, and the next save
    // would write that emptiness into the project.
    if (project.isError) {
      projectHydrationStore.setState({
        status: "error",
        error: asError(project.error),
      });
      return;
    }

    const hash = project.data?.model?.hash;
    if (hash === undefined) {
      // No project, or a project that holds no model: there is nothing to
      // hydrate from, and the draft stands as it is.
      settle();
      return;
    }

    // Unsaved work is never overwritten. The store at this point is either
    // a restored draft or edits made while the status request was running,
    // and either way it is content the project does not have.
    if (hasUnsavedWork(useModelStore.getState())) {
      settle();
      return;
    }

    const draft = loadDraft();
    if (draft !== null && draft.hash === hash) {
      // The draft is the project's model, byte for byte — that is what the
      // receipt says. Restoring it is not only cheaper than a fetch: it holds
      // the coordinates the user actually drew, rather than ones round-tripped
      // through 4326 → 25832 → 4326. Nothing is left to offer afterwards.
      useModelStore.getState().hydrateModel({
        features: draft.features,
        receivers: draft.receivers,
        calcArea: draft.calcArea,
        // `loadDraft` reads it and `ui/draft-banner.tsx` passes it on; omitting
        // it here relabelled a metric draft EPSG:4326, after which a run
        // projected metres as if they were degrees.
        ...(draft.crs === undefined ? {} : { crs: draft.crs }),
      });
      projectHydrationStore.setState({
        started: true,
        status: "settled",
        error: null,
        draftOffered: false,
      });
      return;
    }

    // Set before the first await, never after: a StrictMode double-mount
    // re-runs this effect synchronously, and a guard set in the async body
    // would let both runs through and fire two requests.
    projectHydrationStore.setState({
      started: true,
      status: "fetching",
      error: null,
    });

    void (async () => {
      try {
        const response = await backend.getModel(MODEL_CRS);
        if (response === null) {
          // The status named a hash and the model is gone — a CLI deleted it
          // between the two requests. Nothing to hydrate.
          projectHydrationStore.setState({ status: "settled" });
          return;
        }
        // Checked again, because the user can draw while the request is in
        // flight and the first check is now stale by a network round trip.
        if (hasUnsavedWork(useModelStore.getState())) {
          projectHydrationStore.setState({ status: "settled" });
          return;
        }
        const model = normalizeModelGeoJSON(response.model);
        useModelStore.getState().hydrateModel({
          features: model.features,
          receivers: model.receivers,
          calcArea: model.calcArea,
          // The server's answer, not the constant that was asked for. They
          // agree today, and leaning on that made the store's CRS a statement
          // about this file's request rather than about the coordinates.
          crs: response.crs,
        });
        // `draftOffered` is left as module init computed it. The draft that
        // did not match is divergent work, and this is the case where the
        // user most needs it offered.
        projectHydrationStore.setState({ status: "settled" });
      } catch (err) {
        // Surfaced, not swallowed. An empty map silently drawn over a
        // populated project is the exact failure this hook exists to fix, and
        // a rejection here reproduces it.
        projectHydrationStore.setState({
          status: "error",
          error: asError(err),
        });
      }
    })();
  }, [
    started,
    attempt,
    project.isLoading,
    project.isError,
    project.error,
    project.data,
  ]);
}
