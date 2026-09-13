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
   * The decision has been made. Set synchronously, before any `await`, so a
   * StrictMode double-mount, a remount and HMR all see it — which is also why
   * it lives here rather than in a `useRef`, and why the fetch is imperative
   * rather than a `useQuery`: a query refetches on focus and on remount, and
   * would fight one-shot semantics rather than implement them.
   */
  started: boolean;
  status: HydrationStatus;
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
  projectHydrationStore.setState({
    started: false,
    status: "idle",
    error: null,
  });
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

  useEffect(() => {
    if (started) return;
    // Not the mode string: `runsAgainstSavedModel` already means "the project
    // holds the model a run reads", which is the same fact, and it is the
    // flag every other surface branches on. Browser mode never fetches — the
    // store is the project there — but the decision is still made, because
    // the draft banner waits for it.
    if (!backend.capabilities.runsAgainstSavedModel) {
      projectHydrationStore.setState({ started: true, status: "settled" });
      return;
    }
    // Wait for the status rather than deciding without it; a decision taken
    // while it is still loading would be taken on `data === undefined`, which
    // is indistinguishable here from "no project".
    if (project.isLoading) return;

    const hash = project.isError ? undefined : project.data?.model?.hash;
    if (hash === undefined) {
      // No project, an errored status, or a project that holds no model:
      // there is nothing to hydrate from, and the draft stands as it is.
      projectHydrationStore.setState({ started: true, status: "settled" });
      return;
    }

    // Unsaved work is never overwritten. The store at this point is either
    // a restored draft or edits made while the status request was running,
    // and either way it is content the project does not have.
    if (hasUnsavedWork(useModelStore.getState())) {
      projectHydrationStore.setState({ started: true, status: "settled" });
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
      });
      projectHydrationStore.setState({
        started: true,
        status: "settled",
        draftOffered: false,
      });
      return;
    }

    // Set before the first await, never after: a StrictMode double-mount
    // re-runs this effect synchronously, and a guard set in the async body
    // would let both runs through and fire two requests.
    projectHydrationStore.setState({ started: true, status: "fetching" });

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
          error: err instanceof Error ? err : new Error(String(err)),
        });
      }
    })();
  }, [started, project.isLoading, project.isError, project.data]);
}
