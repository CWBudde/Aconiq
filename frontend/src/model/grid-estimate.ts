/**
 * What the automatic receiver grid will be, before anything computes on it.
 *
 * Two callers, one arithmetic. `buildReceiverGrid` in `api/browser-backend.ts`
 * turns these numbers into receiver positions; the run dialog prints them as
 * "103 × 103 = 10.609 Immissionsorte" while the user is still typing a
 * resolution into the field. A preview computed by a second copy of the
 * formula is worse than no preview at all — it would go on quoting a number
 * the run stopped producing — so the extent resolution, the parameter
 * defaults and the cell counting live here and nowhere else.
 *
 * It sits under `model/` rather than under `run/` because the run dialog is
 * the junior caller: the backend adapter needs the same arithmetic, and a
 * module `api/` has to reach into the UI folder for is a layering the next
 * reader would undo.
 *
 * Everything here is pure, and everything is in the CRS it is handed.
 * `grid_resolution_m` and `grid_padding_m` are metres by contract, so the
 * caller must have projected the model into a metric CRS first —
 * `resolveComputeModel` does that for a run, `useGridExtent` for the dialog.
 * Fed degrees, these functions answer confidently and absurdly.
 */

import type { CalcArea, ModelFeature } from "./types";

/** An axis-aligned extent in the CRS its coordinates came in. */
export interface GridExtent {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

/** The two grid parameters that decide how many receivers there are. */
export interface GridSizing {
  /** Cell spacing in metres — `grid_resolution_m`. */
  resolutionM: number;
  /** Metres added on all four sides of the extent — `grid_padding_m`. */
  paddingM: number;
}

/** The grid an extent and a sizing produce. */
export interface GridShape {
  /** Columns, west to east. */
  width: number;
  /** Rows, south to north. */
  height: number;
  /** `width * height`: every receiver the run will compute a level for. */
  count: number;
  /**
   * The padded south-west corner, which is the centre of cell (0,0) and the
   * position of the first receiver. `buildReceiverGrid` records it as the
   * raster's georeference origin.
   */
  originX: number;
  originY: number;
}

/**
 * The defaults the backend's parameter schema declares. Repeated here because
 * a preview has to render before any profile is loaded, and because a run
 * whose `--param` was left off uses exactly these.
 */
export const DEFAULT_GRID_RESOLUTION_M = 10;
export const DEFAULT_GRID_PADDING_M = 20;

/**
 * Where the preview stops being reassuring and starts warning.
 *
 * 20 000 receivers is roughly a 141 × 141 grid — a 1.4 km square at 10 m, or
 * a 2.8 km square at 20 m. The number is chosen from what browser mode costs,
 * not from what is representable: the WASM kernel spends well under a
 * millisecond per receiver on a bare scene but tens of milliseconds on one
 * with buildings in it, so 20 000 is somewhere between a few seconds and
 * several minutes. Several minutes is the point at which a reader stops
 * believing the tab is working and reloads it.
 *
 * It deliberately sits below the ~42 000 a 2 km × 2 km site produces at the
 * default 10 m, because that case — a plausible site, an untouched default
 * and an hour of compute — is the one this preview exists for. It is a
 * warning and never a refusal: a large grid is a legitimate thing to want,
 * and the user has been told what it costs.
 */
export const RECEIVER_WARNING_THRESHOLD = 20_000;

/**
 * Reads a numeric run parameter, falling back where the field is empty or
 * holds something that is not a number.
 *
 * Shared with the dialog on purpose: the dialog's fields are free text, and
 * a half-typed one has to resolve to whatever the run would resolve it to,
 * `Number.parseFloat`'s leniency included, or the preview describes a grid
 * nobody is going to get.
 */
function numberParam(
  params: Record<string, string>,
  key: string,
  fallback: number,
): number {
  const parsed = Number.parseFloat(params[key] ?? "");
  return Number.isFinite(parsed) ? parsed : fallback;
}

/** The two grid parameters as the run's parameter map spells them. */
export function gridSizingFromParams(
  params: Record<string, string>,
): GridSizing {
  return {
    resolutionM: numberParam(
      params,
      "grid_resolution_m",
      DEFAULT_GRID_RESOLUTION_M,
    ),
    paddingM: numberParam(params, "grid_padding_m", DEFAULT_GRID_PADDING_M),
  };
}

/**
 * The extent over every coordinate in the trees handed in, or `null` when
 * they hold no finite coordinate pair.
 *
 * Every vertex, not a centroid. The CLI had to be taught the same thing: a
 * grid padded around the middle of an area source sits entirely inside it.
 */
function extentOfCoordinates(trees: readonly unknown[]): GridExtent | null {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;

  function visit(coords: unknown): void {
    if (!Array.isArray(coords)) return;
    if (
      coords.length >= 2 &&
      typeof coords[0] === "number" &&
      typeof coords[1] === "number"
    ) {
      const x = coords[0];
      const y = coords[1];
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
      return;
    }
    for (const item of coords) visit(item);
  }

  for (const tree of trees) visit(tree);

  if (!Number.isFinite(minX)) return null;
  return { minX, minY, maxX, maxY };
}

/**
 * The extent over a set of features' geometry.
 *
 * Exported for the receiver-grid extent test: a Parkplatz is an extended
 * footprint, and a grid padded around a point inside it would sit entirely
 * within the source.
 */
export function getFeatureBBox(features: ModelFeature[]): GridExtent | null {
  return extentOfCoordinates(features.map((f) => f.geometry.coordinates));
}

/**
 * The extent the automatic grid covers: the calculation area when the model
 * carries one, and otherwise the bounding box of every source.
 *
 * The fall-through on a degenerate calculation area is deliberate and matches
 * what `computeRLS19Road` did before this moved: an area whose ring holds no
 * usable coordinate is not an instruction to produce no grid.
 */
export function resolveGridExtent(model: {
  features: ModelFeature[];
  calcArea: CalcArea | null;
}): GridExtent | null {
  if (model.calcArea) {
    const area = extentOfCoordinates([model.calcArea.geometry.coordinates]);
    if (area) return area;
  }
  return getFeatureBBox(
    model.features.filter((feature) => feature.kind === "source"),
  );
}

/**
 * The grid an extent produces at a given resolution and padding, or `null`
 * when the resolution cannot describe one.
 *
 * A resolution of zero — a field the user has just cleared, or a `--param 0` —
 * makes the cell count infinite, which as a loop bound freezes the tab. The
 * old code had no guard and did exactly that; the caller now gets a `null` it
 * has to answer for.
 */
export function gridShape(
  extent: GridExtent,
  sizing: GridSizing,
): GridShape | null {
  const { resolutionM, paddingM } = sizing;
  if (!Number.isFinite(resolutionM) || resolutionM <= 0) return null;
  if (!Number.isFinite(paddingM)) return null;

  const minX = extent.minX - paddingM;
  const minY = extent.minY - paddingM;
  const maxX = extent.maxX + paddingM;
  const maxY = extent.maxY + paddingM;

  // `Math.max(1, …)`: an extent smaller than one cell, or one turned inside
  // out by a negative padding, still yields the single receiver a run has to
  // have something to compute.
  const width = Math.max(1, Math.floor((maxX - minX) / resolutionM) + 1);
  const height = Math.max(1, Math.floor((maxY - minY) / resolutionM) + 1);

  return { width, height, count: width * height, originX: minX, originY: minY };
}

/**
 * Resolutions the suggestion is allowed to offer.
 *
 * A ladder rather than the exact value that lands on the budget, because
 * "17,3 m" is an answer nobody would have typed and nobody can defend in a
 * report. These are the spacings noise maps are actually drawn at.
 */
const RESOLUTION_LADDER = [
  5, 10, 15, 20, 25, 30, 40, 50, 75, 100, 150, 200, 250, 500, 1000,
];

/**
 * The finest resolution from {@link RESOLUTION_LADDER} whose grid stays within
 * `budget` receivers, or `null` when even the coarsest does not.
 *
 * This is the useful half of the warning: telling a reader that 42 000
 * receivers is a lot leaves them to bisect the field themselves.
 */
export function resolutionForReceiverBudget(
  extent: GridExtent,
  paddingM: number,
  budget: number,
): number | null {
  for (const resolutionM of RESOLUTION_LADDER) {
    const shape = gridShape(extent, { resolutionM, paddingM });
    if (shape !== null && shape.count <= budget) return resolutionM;
  }
  return null;
}
