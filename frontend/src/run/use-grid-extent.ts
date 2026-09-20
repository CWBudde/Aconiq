/**
 * The extent the automatic receiver grid will cover, in metres, while the run
 * dialog is open.
 *
 * The dialog cannot simply read the store and count cells. The store holds
 * EPSG:4326 by default, and `grid_resolution_m` is metres: over a whole city
 * a geographic extent measures 0.05 units, so a 10 m grid over it would
 * preview as a single receiver and the field would read as though it cost
 * nothing. The run does not have that problem because `resolveComputeModel`
 * projects the workspace before any builder reads a coordinate — so this hook
 * calls that same function, not a projection of its own. The preview's extent
 * is the run's extent, and not an approximation of it.
 *
 * Calling it rather than `projectWorkspace` also means the two share its
 * memo, which is keyed on the workspace alone: opening the dialog warms the
 * projection the run then reuses, so the preview costs the round trip the run
 * was going to pay anyway. The transform is reached through `backend` rather
 * than through a kernel handle because the dialog has none and API mode has
 * no kernel at all; in browser mode it is the very same kernel call.
 *
 * The whole workspace goes into the batch, not just the features that decide
 * the extent. `crstransform.resolveTarget` picks the UTM zone from the centre
 * of the batch it is given, so projecting a subset could land the preview in
 * a different zone than the run.
 *
 * It re-projects when the model changes and never when a parameter does:
 * resolution and padding are arithmetic on top of this, which is what lets
 * the count follow the field keystroke by keystroke without a round trip.
 *
 * TODO: the exact receiver count is the free half of "what will this run
 * cost". The other half is wall time, and it cannot be derived from the count
 * alone — a scene with buildings in it costs tens of milliseconds a receiver
 * and a bare one costs well under a millisecond. The follow-up is a timing
 * probe: compute a fixed 16-receiver sample of this very grid through the
 * kernel, measure it, and scale. That needs the kernel (so browser mode only,
 * with API mode saying nothing rather than guessing) and a way to run a
 * throwaway compute that never becomes a run, which is why it is not here.
 */

import { useEffect, useRef, useState } from "react";
import { backend } from "@/api/backend";
import { resolveComputeModel } from "@/model/compute-crs";
import { resolveGridExtent, type GridExtent } from "@/model/grid-estimate";
import { useModelStore } from "@/model/model-store";

/** Where the preview's extent is. */
export type GridExtentState =
  /** Nobody asked — the dialog is in custom-receiver mode. */
  | { status: "idle" }
  /** The backend cannot project, so no metric extent can be had. */
  | { status: "unavailable" }
  /** A projection is in flight. */
  | { status: "projecting" }
  /**
   * The model has no calculation area and no source, so there is nothing to
   * lay a grid over. A real state: the run refuses in it too.
   */
  | { status: "no-extent" }
  /** The projection failed; the dialog says so rather than quoting a number. */
  | { status: "failed" }
  /** The extent, in the CRS the run will compute in. */
  | { status: "ready"; extent: GridExtent };

export function useGridExtent(enabled: boolean): GridExtentState {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const calcArea = useModelStore((s) => s.calcArea);
  const crs = useModelStore((s) => s.crs);

  const [state, setState] = useState<GridExtentState>({ status: "idle" });

  // Monotonic and compared on arrival, as in `useInverseProjection`: a model
  // edited while a projection is out must not have the earlier answer land on
  // top of the later one.
  const requestRef = useRef(0);

  useEffect(() => {
    const request = (requestRef.current += 1);

    if (!enabled) {
      setState({ status: "idle" });
      return;
    }
    if (!backend.capabilities.canReprojectForDisplay) {
      setState({ status: "unavailable" });
      return;
    }
    // Asked before the transform, not after: an empty model has no coordinate
    // to send, and "there is nothing to lay a grid over" is true in every CRS.
    if (resolveGridExtent({ features, calcArea }) === null) {
      setState({ status: "no-extent" });
      return;
    }

    setState({ status: "projecting" });
    void resolveComputeModel(
      { transform: (req) => backend.transformCoordinates(req) },
      { features, receivers, calcArea, crs },
    ).then(
      (model) => {
        if (requestRef.current !== request) return;
        const extent = resolveGridExtent(model);
        setState(
          extent === null
            ? { status: "no-extent" }
            : { status: "ready", extent },
        );
      },
      () => {
        if (requestRef.current !== request) return;
        // The reason is deliberately dropped. A refused CRS reads as a wall of
        // Go error text, and this is a preview: the dialog says it cannot
        // estimate, and the run — which refuses in the same words — is where
        // the reader is owed the detail.
        setState({ status: "failed" });
      },
    );
  }, [enabled, features, receivers, calcArea, crs]);

  return state;
}
