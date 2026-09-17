/**
 * The one place a coordinate travels from the map into the model.
 *
 * `display-model.ts` states the invariant this file is the exception to: the
 * map is a projection *of* the model, never a source *for* it. Drawing is the
 * deliberate exception, and it only works in one direction — terra-draw emits
 * WGS84 whatever the store holds, so a shape drawn over a project in EPSG:25832
 * arrives in degrees and has to be moved into the store's CRS before anything
 * writes it. Until `POST /api/v1/transform` landed there was no inverse in API
 * mode at all, and drawing was simply switched off over a metric model.
 *
 * The projection is `aconiq.transform`'s, reached through
 * `backend.transformCoordinates`, and the target CRS is **explicit** rather than
 * `"auto"`: the store's CRS is not a question the kernel gets to answer here,
 * and an `auto` request would resolve a zone from the drawn shape instead of
 * honouring the one the model is already in.
 *
 * A store already in {@link DISPLAY_CRS} takes no transform and stays
 * synchronous, which is the common path and the one where the new-feature
 * dialog must open on the same tick the shape is finished.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { backend } from "@/api/backend";
import { projectGeometry } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import type { Geometry } from "@/model/types";
import { DISPLAY_CRS } from "./display-model";
import type { DrawMode } from "./use-draw";

/** Where a finished shape is on its way from the map into the store. */
export type DrawProjectionStatus =
  | { status: "idle" }
  | { status: "projecting"; targetCRS: string }
  | { status: "failed"; targetCRS: string; error: Error };

export interface DrawProjection {
  status: DrawProjectionStatus;
  /** terra-draw's finish handler: takes the WGS84 shape and hands it on in the store's CRS. */
  accept: (mode: DrawMode, feature: GeoJSON.Feature) => void;
  /** Clears a failure the reader has read. */
  dismiss: () => void;
}

/**
 * Wraps the draw-finish callback in the inverse transform.
 *
 * `onGeometry` is called with coordinates in the store's CRS, or not at all:
 * a failed projection is reported through {@link DrawProjection.status} and the
 * shape is dropped. Dropping it silently is what the old CRS gate was put there
 * to avoid — a user completing a shape and watching it vanish — so the caller
 * must render the failure.
 */
export function useDrawProjection(
  onGeometry: (mode: DrawMode, geometry: Geometry) => void,
): DrawProjection {
  const crs = useModelStore((s) => s.crs);
  const [status, setStatus] = useState<DrawProjectionStatus>({
    status: "idle",
  });

  // Monotonic and compared on arrival, as in `useDisplayModel`: a second shape
  // finished while the first is still in flight, or a CRS change underneath
  // one, must not land the answer to the earlier question.
  const requestRef = useRef(0);

  // `accept` is the only other place the token advances, so without this a CRS
  // change under a pending transform would leave the token matching and the
  // answer landing: coordinates projected for the CRS the store has just left,
  // written into the store that now declares a different one. Undoing a model
  // merge restores `previousCRS` and does exactly that.
  //
  // Clearing the status is the other half. A callback that bails leaves
  // whatever was last set standing, so dropping an answer without this would
  // strand the map on "projecting" for a shape nothing is going to deliver —
  // and `pages/map.tsx` keeps the toolbar disabled while that status is up.
  useEffect(() => {
    requestRef.current += 1;
    setStatus({ status: "idle" });
  }, [crs]);

  const onGeometryRef = useRef(onGeometry);
  onGeometryRef.current = onGeometry;

  const accept = useCallback(
    (mode: DrawMode, feature: GeoJSON.Feature) => {
      const geometry = feature.geometry;
      // terra-draw emits points, lines and polygons and never a
      // GeometryCollection — the one GeoJSON geometry with no `coordinates`.
      // Narrowing rather than casting is what keeps that a fact the compiler
      // checks.
      if (!("coordinates" in geometry)) return;

      if (crs === DISPLAY_CRS) {
        requestRef.current += 1;
        setStatus({ status: "idle" });
        onGeometryRef.current(mode, geometry as Geometry);
        return;
      }

      const request = (requestRef.current += 1);
      setStatus({ status: "projecting", targetCRS: crs });

      void projectGeometry(
        (req) => backend.transformCoordinates(req),
        geometry,
        DISPLAY_CRS,
        crs,
      ).then(
        (projected) => {
          if (requestRef.current !== request) return;
          // The store's CRS as of now, not as of the render that started this
          // request. The effect above catches a CRS change once React has
          // re-rendered; this catches one that lands in the window between the
          // store updating and that render, where the token still matches.
          // Writing here is the irreversible half, so it is checked twice.
          if (useModelStore.getState().crs !== crs) return;
          setStatus({ status: "idle" });
          onGeometryRef.current(mode, projected.geometry as Geometry);
        },
        (error: unknown) => {
          if (requestRef.current !== request) return;
          setStatus({
            status: "failed",
            targetCRS: crs,
            error: error instanceof Error ? error : new Error(String(error)),
          });
        },
      );
    },
    [crs],
  );

  const dismiss = useCallback(() => {
    setStatus({ status: "idle" });
  }, []);

  return { status, accept, dismiss };
}
