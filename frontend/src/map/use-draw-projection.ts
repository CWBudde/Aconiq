/**
 * The draw-finish half of the map's one inverse projection.
 *
 * The transform itself, and every staleness guard on it, lives in
 * `use-inverse-projection.ts` — read that file's header for why the direction
 * exists at all and why the target CRS is named rather than left to `"auto"`.
 * This file is the thin part: it narrows what terra-draw hands over and passes
 * the drawing mode through to the caller.
 */

import { useCallback, useRef } from "react";
import type { Geometry } from "@/model/types";
import type { DrawMode } from "./use-draw";
import { useInverseProjection } from "./use-inverse-projection";
import type { InverseProjectionStatus } from "./use-inverse-projection";

/** Where a finished shape is on its way from the map into the store. */
export type DrawProjectionStatus = InverseProjectionStatus;

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
  const { status, project, dismiss } = useInverseProjection();

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

      project(geometry as Geometry, (projected) => {
        onGeometryRef.current(mode, projected);
      });
    },
    [project],
  );

  return { status, accept, dismiss };
}
