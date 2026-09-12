/**
 * The one owner of "save the workspace to the project". The header's sync
 * state and the run dialog's gate both read from here, so there is a single
 * definition of what `dirty` means, one payload builder, and one place that
 * decides when the flag may be cleared.
 *
 * It lives next to the model store rather than under `api/` because the
 * question it answers — does the workspace differ from the project? — is a
 * model question; the API call is only how the answer gets reset.
 */

import { useCallback } from "react";
import { create } from "zustand";
import { backend } from "@/api/backend";
import { useIsSavingModel, useProjectStatus, useSaveModel } from "@/api/hooks";
import { useModelStore } from "@/model/model-store";
import { modelToGeoJSON } from "@/model/to-geojson";
import { writeDraft } from "@/model/use-autosave";

export type ProjectSyncStatus = "clean" | "dirty" | "saving" | "error";

export interface ProjectSync {
  /**
   * Whether a project save is something this workspace can do at all: the
   * backend runs against the saved model and a project is loaded. When not
   * set, the draft is the persistence and the header shows nothing.
   */
  enabled: boolean;
  status: ProjectSyncStatus;
  /** The workspace differs from what the project holds. */
  dirty: boolean;
  /** The last failed save; cleared by the next successful one. */
  error: Error | null;
  /** Never rejects: a failure lands in `error` and `status === "error"`. */
  save: () => Promise<void>;
}

/**
 * The map draws in WGS84, so that is what the coordinates are in when they
 * leave the store; the server transforms into the project CRS.
 */
const MODEL_CRS = "EPSG:4326";

interface ProjectSyncState {
  /** The last failed save; cleared by the next successful one. */
  error: Error | null;
  /**
   * Re-entrancy guard that does not wait for a render or for react-query's
   * mutation cache: a second `save()` in the same tick — Ctrl+S in the header
   * and the dialog's inline button are two surfaces over one model — must
   * see the first one already running.
   */
  inFlight: boolean;
}

/**
 * Module-level rather than per hook instance: the header and the run dialog
 * each mount their own `useProjectSync`, and a save is a fact about the
 * model, not about whoever started it — an error from one surface has to
 * show on the other, and the in-flight guard has to hold across both. It is
 * a separate store from the model store because this is transport state
 * (did the last POST fail?), not model content, and the model store's
 * undo/redo and `dirty` bookkeeping should not have to know about requests.
 * Exported for tests to reset between cases.
 */
export const projectSyncStore = create<ProjectSyncState>(() => ({
  error: null,
  inFlight: false,
}));

export function useProjectSync(): ProjectSync {
  const dirty = useModelStore((s) => s.dirty);
  const markClean = useModelStore((s) => s.markClean);
  const project = useProjectStatus();
  const { mutateAsync } = useSaveModel();
  // Derived from the mutation cache rather than this instance's mutation, so
  // every surface shows "saving" for a save any surface started.
  const saving = useIsSavingModel();
  const error = projectSyncStore((s) => s.error);

  const enabled =
    backend.capabilities.runsAgainstSavedModel && project.data != null;

  const save = useCallback(async () => {
    if (projectSyncStore.getState().inFlight) return;
    projectSyncStore.setState({ inFlight: true });
    // Read the store directly rather than through selectors: the snapshot
    // taken here is what gets sent, and it is what the flag is compared
    // against afterwards.
    const before = useModelStore.getState();
    try {
      await mutateAsync({ crs: MODEL_CRS, model: modelToGeoJSON(before) });
      // An edit made while the request was in flight is not in the project,
      // so the flag stays set for it. Reference equality is enough: every
      // edit replaces the array, and the calculation area — not part of the
      // payload, but part of the workspace — is included so a change to it
      // is not silently declared synced either.
      const after = useModelStore.getState();
      if (
        after.features === before.features &&
        after.receivers === before.receivers &&
        after.calcArea === before.calcArea
      ) {
        markClean();
      }
      // The draft is crash recovery, and after a save it would be older than
      // the project: a later "Restore draft" would offer the pre-save model.
      // Writing what was just saved keeps the two in agreement. If an edit
      // landed meanwhile the model is still dirty, and the autosave's pending
      // write overtakes this one with the newer state.
      writeDraft({
        features: before.features,
        receivers: before.receivers,
        calcArea: before.calcArea,
      });
      projectSyncStore.setState({ error: null });
    } catch (err) {
      projectSyncStore.setState({
        error: err instanceof Error ? err : new Error(String(err)),
      });
    } finally {
      projectSyncStore.setState({ inFlight: false });
    }
  }, [mutateAsync, markClean]);

  let status: ProjectSyncStatus;
  if (saving) status = "saving";
  else if (error !== null) status = "error";
  else if (dirty) status = "dirty";
  else status = "clean";

  return { enabled, status, dirty, error, save };
}
