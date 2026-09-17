/**
 * The model as the map may draw it, which is not always the model as the store
 * holds it.
 *
 * `useModelStore` carries the CRS its coordinates are in, and an import or a
 * restored draft can put a metric one there — `readCollectionCRS` reads it off
 * the file, and `pages/import.tsx` passes it in **both** modes. Nothing under
 * `src/map/` used to read that field: a model in EPSG:25832 imported correctly,
 * ran correctly, and was drawn a few hundred kilometres off the coast of Africa
 * or, more often, threw `Invalid LngLat latitude value` out of the effect that
 * fitted it and left the map blank.
 *
 * The answer is split by capability, not by mode. Where a projector is reachable
 * the model is projected into {@link DISPLAY_CRS} through the Go kernel — never
 * a second transverse-Mercator implementation in TypeScript, and never the
 * store's own arrays written back. Where none is reachable the map refuses and
 * says what to do, rather than drawing metres as if they were degrees.
 *
 * The invariant this file exists to hold: *the map is a projection **of** the
 * model, never a source **for** it.* Nothing here writes a projected coordinate
 * back into the store, and nothing here changes `store.crs`.
 */

import { useEffect, useRef, useState } from "react";
import { backend } from "@/api/backend";
import { projectWorkspace } from "@/model/compute-crs";
import type { CoordinateTransform } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

/**
 * The CRS MapLibre draws in. It takes lon/lat and nothing else, so this is a
 * property of the renderer rather than a preference.
 */
export const DISPLAY_CRS = "EPSG:4326";

/** What the map may draw, and what it must say when it may not. */
export type DisplayModel =
  | {
      status: "ready";
      features: ModelFeature[];
      receivers: ModelReceiver[];
      calcArea: CalcArea | null;
      /** The CRS the store holds, which is what a run and a save still use. */
      sourceCRS: string;
      /** True when getting to {@link DISPLAY_CRS} moved the coordinates. */
      reprojected: boolean;
    }
  | { status: "projecting"; sourceCRS: string }
  /** No projector in this mode — see `BackendCapabilities.canReprojectForDisplay`. */
  | { status: "unsupported"; sourceCRS: string }
  | { status: "failed"; sourceCRS: string; error: Error };

/**
 * The store's model in {@link DISPLAY_CRS}.
 *
 * A store already in {@link DISPLAY_CRS} answers `ready` synchronously, with the
 * store's own arrays **by reference** and without touching the backend. That
 * identity short-circuit is load-bearing rather than an optimisation:
 * `wasmkernel.resolveTarget` transforms unconditionally on an explicit target,
 * so a 4326 → 4326 request would build a projection pipeline per render for
 * nothing — and the common render path stays exactly the code it was.
 */
export function useDisplayModel(): DisplayModel {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const calcArea = useModelStore((s) => s.calcArea);
  const crs = useModelStore((s) => s.crs);

  const canReproject = backend.capabilities.canReprojectForDisplay;

  const [projected, setProjected] = useState<DisplayModel | null>(null);
  // Monotonic, and compared on arrival: a CRS change or an edit mid-flight
  // must not be overwritten by the answer to the question before it, which
  // would draw the previous model in the current model's place.
  const requestRef = useRef(0);

  useEffect(() => {
    // Both of these are answered in the render below without asking anyone, so
    // there is nothing to start here — only anything in flight to disown, and
    // a stale answer to drop that would otherwise keep the map on a model the
    // store no longer holds.
    if (crs === DISPLAY_CRS || !canReproject) {
      requestRef.current += 1;
      setProjected(null);
      return;
    }

    const request = (requestRef.current += 1);

    setProjected({ status: "projecting", sourceCRS: crs });

    const transform: CoordinateTransform = (req) =>
      backend.transformCoordinates(req);

    void projectWorkspace(
      transform,
      { features, receivers, calcArea, crs },
      DISPLAY_CRS,
    ).then(
      (model) => {
        if (requestRef.current !== request) return;
        setProjected({
          status: "ready",
          features: model.features,
          receivers: model.receivers,
          calcArea: model.calcArea,
          sourceCRS: crs,
          reprojected: model.projection.applied,
        });
      },
      (error: unknown) => {
        if (requestRef.current !== request) return;
        setProjected({
          status: "failed",
          sourceCRS: crs,
          error: error instanceof Error ? error : new Error(String(error)),
        });
      },
    );
  }, [features, receivers, calcArea, crs, canReproject]);

  if (crs === DISPLAY_CRS) {
    return {
      status: "ready",
      features,
      receivers,
      calcArea,
      sourceCRS: crs,
      reprojected: false,
    };
  }

  // Answered in the render rather than from the effect, so the map never shows
  // "projecting" for a projection that is never going to be attempted.
  if (!canReproject) {
    return { status: "unsupported", sourceCRS: crs };
  }

  // Before the effect has run once — the first render after a CRS change —
  // there is no answer yet, and "projecting" is what that is.
  return projected ?? { status: "projecting", sourceCRS: crs };
}
