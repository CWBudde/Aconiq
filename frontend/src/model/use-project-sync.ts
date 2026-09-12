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

import { useCallback, useRef, useState } from "react";
import { backend } from "@/api/backend";
import { useProjectStatus, useSaveModel } from "@/api/hooks";
import { useModelStore } from "@/model/model-store";
import { modelToGeoJSON } from "@/model/to-geojson";

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

export function useProjectSync(): ProjectSync {
  const dirty = useModelStore((s) => s.dirty);
  const markClean = useModelStore((s) => s.markClean);
  const project = useProjectStatus();
  const { mutateAsync, isPending } = useSaveModel();
  const [error, setError] = useState<Error | null>(null);
  // Re-entrancy guard that does not wait for a render: `isPending` is a
  // snapshot from the last render, so a second Ctrl+S inside the same tick
  // would still see `false`.
  const inFlight = useRef(false);

  const enabled =
    backend.capabilities.runsAgainstSavedModel && project.data != null;

  const save = useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
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
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      inFlight.current = false;
    }
  }, [mutateAsync, markClean]);

  let status: ProjectSyncStatus;
  if (isPending) status = "saving";
  else if (error !== null) status = "error";
  else if (dirty) status = "dirty";
  else status = "clean";

  return { enabled, status, dirty, error, save };
}
