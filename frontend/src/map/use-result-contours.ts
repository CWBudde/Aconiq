/**
 * A run's contours, fetched and turned into a GeoJSON source — or why there
 * are none.
 *
 * The shape is `use-result-raster.ts`'s, deliberately: one hook so the whole
 * path is reachable by `renderHook` without a map, a status for every refusal
 * rather than a silent absence, and a pure resolver so the table of outcomes
 * is one `expect` away.
 *
 * What is *not* here is any geometry. The lines are traced by Go — the same
 * `contour.FromRaster` `aconiq export` calls — and arrive already in the CRS
 * this asked for, so nothing in the frontend traces or projects a contour.
 * That is the whole reason the map had no contour layer until the kernel grew
 * the export: a TypeScript marching squares would be a second answer to where
 * a 55 dB line falls.
 */
import { useMemo } from "react";
import { backend } from "@/api/backend";
import type { ContourLine } from "@/api/backend";
import type { RunSummary } from "@/api/client";
import { useRunContours } from "@/api/hooks";
import { DISPLAY_CRS } from "./display-model";
import { RESULT_LEVEL_PROPERTY } from "./layers";

/** Drawable contours, or the reason there are none. */
export type ResultContours =
  /** No run, or a run that wrote no raster to trace. */
  | { status: "none" }
  | { status: "loading" }
  /** No projector in this mode, so no CRS to ask for them in. */
  | { status: "unsupported" }
  /** The backend refused: not a grid, or a CRS it could not move them into. */
  | { status: "failed"; reason: string }
  | { status: "ready"; collection: GeoJSON.FeatureCollection };

const NONE: ResultContours = { status: "none" };
const LOADING: ResultContours = { status: "loading" };
const EMPTY_COLLECTION: GeoJSON.FeatureCollection = {
  type: "FeatureCollection",
  features: [],
};

/**
 * One feature per line, carrying its level under the property the paint
 * expression reads.
 *
 * Filtered to the chosen band, because one request returns every band the
 * raster carries — the same shape the raster layer's picker works on, and the
 * reason neither re-requests when the picker moves.
 *
 * A line of fewer than two points is dropped, as `ExportContourGeoJSON` drops
 * it: MapLibre would accept a one-vertex LineString and draw nothing.
 */
function toCollection(
  lines: ContourLine[],
  indicator: string,
): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  for (const line of lines) {
    if (line.band_name !== indicator) continue;
    if (line.points.length < 2) continue;

    features.push({
      type: "Feature",
      properties: { [RESULT_LEVEL_PROPERTY]: line.level },
      geometry: { type: "LineString", coordinates: line.points },
    });
  }

  return { type: "FeatureCollection", features };
}

export function useResultContours(
  run: RunSummary | null,
  indicator: string,
): ResultContours {
  // Asked for in the CRS the map draws in, so Go reprojects them and the
  // frontend never touches a contour vertex. `canReprojectForDisplay` is a
  // property of the interface rather than of the two implementations that
  // happen to exist, so a backend without a projector still has to be handled.
  const canReproject = backend.capabilities.canReprojectForDisplay;

  // Only a run that wrote a raster has contours. Asking about one that did not
  // would spend a request to be told so, and the refusal is already knowable
  // from the artifact list the run is carrying.
  const hasRaster =
    run?.artifacts.some(
      (artifact) => artifact.kind === "run.result.raster_metadata",
    ) ?? false;

  const {
    data,
    isLoading,
    error: fetchError,
  } = useRunContours(
    canReproject && hasRaster ? (run?.id ?? null) : null,
    canReproject && hasRaster ? { crs: DISPLAY_CRS } : null,
  );

  const collection = useMemo(
    () => (data ? toCollection(data.lines, indicator) : EMPTY_COLLECTION),
    [data, indicator],
  );

  // Memoised because the caller feeds this into an effect's dependency list; a
  // fresh object per render would call `setData` on every render.
  return useMemo((): ResultContours => {
    if (run === null || !hasRaster) return NONE;
    if (!canReproject) return { status: "unsupported" };
    if (fetchError) {
      // The backend's own words, whichever boundary answered — "these
      // receivers were placed individually", or a CRS it could not move them
      // into. Restating them here would make two answers to one question.
      return { status: "failed", reason: fetchError.message };
    }
    if (data === undefined) return isLoading ? LOADING : NONE;

    return { status: "ready", collection };
  }, [run, hasRaster, canReproject, fetchError, data, isLoading, collection]);
}
