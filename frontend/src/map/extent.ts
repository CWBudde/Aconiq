import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { DISPLAY_CRS } from "./display-model";

export type ViewState = {
  center: [number, number];
  zoom: number;
};

/** A workspace's extent, in whatever CRS the workspace is in. */
export type Bounds = {
  west: number;
  south: number;
  east: number;
  north: number;
};

function visitBounds(coords: unknown, bounds: Bounds): void {
  if (!Array.isArray(coords)) return;
  if (
    coords.length >= 2 &&
    typeof coords[0] === "number" &&
    typeof coords[1] === "number"
  ) {
    bounds.west = Math.min(bounds.west, coords[0]);
    bounds.south = Math.min(bounds.south, coords[1]);
    bounds.east = Math.max(bounds.east, coords[0]);
    bounds.north = Math.max(bounds.north, coords[1]);
    return;
  }
  for (const value of coords) {
    visitBounds(value, bounds);
  }
}

/**
 * The extent of everything the workspace holds — features, receivers and the
 * calculation area — or `null` when it holds no coordinate at all.
 *
 * The one traversal. `model-layers.tsx` kept a copy that visited features
 * alone, so a project consisting of imported receivers never came into view:
 * the one-shot fit computed no bounds for it and fitted nothing.
 */
export function computeWorkspaceBounds(
  features: ModelFeature[],
  receivers: ModelReceiver[],
  calcArea: CalcArea | null,
): Bounds | null {
  const bounds: Bounds = {
    west: Number.POSITIVE_INFINITY,
    south: Number.POSITIVE_INFINITY,
    east: Number.NEGATIVE_INFINITY,
    north: Number.NEGATIVE_INFINITY,
  };

  for (const feature of features) {
    visitBounds(feature.geometry.coordinates, bounds);
  }

  if (calcArea) {
    visitBounds(calcArea.geometry.coordinates, bounds);
  }

  for (const receiver of receivers) {
    visitBounds(receiver.geometry.coordinates, bounds);
  }

  if (!Number.isFinite(bounds.west)) return null;
  return bounds;
}

/**
 * The extent of one geometry's coordinates, or `null` when it holds none.
 *
 * The single-feature counterpart to {@link computeWorkspaceBounds}, for the
 * camera that takes a reader to one finding. It is a separate function rather
 * than `computeWorkspaceBounds([feature], [], null)` because the workspace
 * traversal takes the three collections a workspace is made of, and a caller
 * holding one geometry should not have to name the other two as empty.
 *
 * A Point answers with a zero-span box — `west === east` — which is correct and
 * is the caller's problem: `fitBounds` needs a `maxZoom` to make sense of it.
 */
export function computeGeometryBounds(coordinates: unknown): Bounds | null {
  const bounds: Bounds = {
    west: Number.POSITIVE_INFINITY,
    south: Number.POSITIVE_INFINITY,
    east: Number.NEGATIVE_INFINITY,
    north: Number.NEGATIVE_INFINITY,
  };
  visitBounds(coordinates, bounds);
  if (!Number.isFinite(bounds.west)) return null;
  return bounds;
}

/**
 * The same extent as a MapLibre `LngLatBoundsLike`, or `null` when it is not
 * one.
 *
 * With the display projection in place the bounds handed to `fitBounds` are
 * lon/lat by construction, and this guard still earns its keep: it covers the
 * case reprojection cannot fix, a store *labelled* EPSG:4326 that actually
 * holds metres. MapLibre answers a 5 644 000 latitude by throwing
 * `Invalid LngLat latitude value` out of the effect that called it, which took
 * the model layers down with it.
 */
export function toLngLatBounds(
  bounds: Bounds,
): [[number, number], [number, number]] | null {
  const values = [bounds.west, bounds.south, bounds.east, bounds.north];
  if (!values.every((value) => Number.isFinite(value))) return null;
  if (Math.abs(bounds.west) > 180 || Math.abs(bounds.east) > 180) return null;
  if (Math.abs(bounds.south) > 90 || Math.abs(bounds.north) > 90) return null;

  return [
    [bounds.west, bounds.south],
    [bounds.east, bounds.north],
  ];
}

/**
 * The viewport the workspace opens at, computed once on mount.
 *
 * `crs` is taken rather than assumed because this runs in a `useState`
 * initialiser and therefore cannot await a projection. A workspace in anything
 * but {@link DISPLAY_CRS} gets the fallback centre instead of a centre computed
 * from eastings and northings, which is how a German model used to open on
 * `[667000, 5644000]` — a point MapLibre clamps to the edge of the world. The
 * correct framing arrives one frame later, from `ModelLayers`' `fitBounds` over
 * the reprojected model.
 */
export function fitViewToWorkspace(
  features: ModelFeature[],
  receivers: ModelReceiver[],
  calcArea: CalcArea | null,
  fallbackCenter: [number, number],
  crs: string,
): ViewState {
  const bounds =
    crs === DISPLAY_CRS
      ? computeWorkspaceBounds(features, receivers, calcArea)
      : null;
  if (!bounds) {
    return { center: fallbackCenter, zoom: 6 };
  }

  const centerLng = (bounds.west + bounds.east) / 2;
  const centerLat = (bounds.south + bounds.north) / 2;

  const lonSpan = Math.max(0.01, bounds.east - bounds.west);
  const latSpan = Math.max(0.01, bounds.north - bounds.south);
  const span = Math.max(lonSpan, latSpan);

  let zoom = 14;
  if (span > 20) zoom = 4;
  else if (span > 8) zoom = 6;
  else if (span > 2) zoom = 8;
  else if (span > 0.5) zoom = 10;
  else if (span > 0.1) zoom = 12;

  return {
    center: [centerLng, centerLat],
    zoom,
  };
}
