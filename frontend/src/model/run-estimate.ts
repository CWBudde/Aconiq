/**
 * What a run will cost in time, before it is started — and what would make
 * it cheaper.
 *
 * Both backends answer with the same JSON, because both are the same Go code:
 * `POST /api/v1/runs/estimate` spawns `aconiq run --estimate --json`, and
 * browser mode calls the kernel's `estimate`, and each is `road.Estimate`
 * timing a stratified sample of the run's own (source, Teilstück) pairs. The
 * shape is declared here rather than under `api/` for the reason
 * `standards/descriptor.ts` gives: neither backend owns it, and `wasm/` must
 * not import from `api/`.
 *
 * The suggestions are computed here, from the estimate and from
 * `model/grid-estimate` — the arithmetic `buildReceiverGrid` builds the
 * receivers from — so a suggested spacing quotes the receiver count the run at
 * that spacing will actually have.
 */

import {
  gridShape,
  resolutionForReceiverBudget,
  type GridExtent,
  type GridSizing,
} from "./grid-estimate";

/** A duration range in seconds: a point estimate and the band around it. */
export interface EstimateBand {
  low: number;
  estimate: number;
  high: number;
}

/** A variant of the run costed from the same probe. */
export interface EstimateAlternative {
  params: Record<string, string>;
  units: number;
  cpu_s: EstimateBand;
  wall_s: EstimateBand;
}

/** Mirrors Go's `runestimate.Estimate`. */
export interface RunEstimate {
  /** False for a standard with no probe; `reason` then says so. */
  available: boolean;
  reason?: string;
  standard_id?: string;
  wall_s?: EstimateBand;
  cpu_s?: EstimateBand;
  cpu_per_receiver_s?: EstimateBand;
  /** The speed-up over one worker the point estimate assumes. */
  parallelism?: number;
  inputs?: {
    receivers: number;
    workers: number;
    units: number;
    counts?: Record<string, number>;
  };
  probe?: {
    probe_receivers: number;
    measured_receivers: number;
    units_timed: number;
    budget_s: number;
    elapsed_s: number;
    prepare_s: number;
    budget_exhausted: boolean;
  };
  alternatives?: EstimateAlternative[];
}

/**
 * Above this the dialog warns. Ten minutes is where a run stops being
 * something a user waits for and becomes something they leave running — and
 * a run that turns out to take hours should not be discovered that way.
 */
export const ESTIMATE_WARNING_SECONDS = 600;

/** What a suggestion aims for: a run that finishes within a minute. */
export const ESTIMATE_TARGET_SECONDS = 60;

/** A change of parameters that would make the run cheaper. */
export interface EstimateSuggestion {
  /** The parameters to write into the form. */
  params: Record<string, string>;
  /** The receiver count at the suggested spacing, when it changes. */
  receivers?: number;
  resolutionM?: number;
  distanceScaled: boolean;
  /** The suggested run's wall time. */
  wall: EstimateBand;
}

const DISTANCE_SCALED = "distance_scaled";

function scale(band: EstimateBand, factor: number): EstimateBand {
  return {
    low: band.low * factor,
    estimate: band.estimate * factor,
    high: band.high * factor,
  };
}

function plus(band: EstimateBand, seconds: number): EstimateBand {
  return {
    low: band.low + seconds,
    estimate: band.estimate + seconds,
    high: band.high + seconds,
  };
}

/**
 * The wall time the same scene would take with `receivers` receivers, from a
 * per-receiver CPU band and the parallelism the estimate assumed. Fewer
 * receivers than workers cannot keep them all busy, hence the `min`.
 */
function wallForReceivers(
  cpuPerReceiver: EstimateBand,
  parallelism: number,
  prepareS: number,
  receivers: number,
): EstimateBand {
  const speedUp = Math.max(1, Math.min(parallelism, receivers));
  return plus(scale(cpuPerReceiver, receivers / speedUp), prepareS);
}

/**
 * Parameter changes that would bring a run the estimate says is long under
 * {@link ESTIMATE_TARGET_SECONDS}, in the order a reader should try them:
 * the coarser grid alone, distance-scaled Teilstücke alone, then both.
 *
 * Only automatic grids get a spacing suggestion — `extent` is null for a
 * custom receiver set, whose count no spacing changes. A suggestion that
 * still misses the target is offered anyway if it is the best there is,
 * because "this is the coarsest spacing that helps" is still worth a click;
 * one that changes nothing is not offered.
 */
export function suggestFasterRun(
  estimate: RunEstimate,
  extent: GridExtent | null,
  sizing: GridSizing,
  params: Record<string, string>,
): EstimateSuggestion[] {
  const wall = estimate.wall_s;
  const cpuPerReceiver = estimate.cpu_per_receiver_s;
  const receivers = estimate.inputs?.receivers ?? 0;
  if (
    !estimate.available ||
    wall === undefined ||
    cpuPerReceiver === undefined ||
    receivers === 0 ||
    wall.estimate <= ESTIMATE_WARNING_SECONDS
  ) {
    return [];
  }

  const parallelism = estimate.parallelism ?? 1;
  const prepareS = estimate.probe?.prepare_s ?? 0;
  const scaled = (estimate.alternatives ?? []).find(
    (alt) => alt.params.segment_length_mode === DISTANCE_SCALED,
  );

  const suggestions: EstimateSuggestion[] = [];

  const coarser = (perReceiver: EstimateBand) => {
    if (extent === null) return null;
    // The receiver budget the target allows, at the point estimate.
    const budget = Math.floor(
      ((ESTIMATE_TARGET_SECONDS - prepareS) * Math.max(1, parallelism)) /
        Math.max(perReceiver.estimate, 1e-9),
    );
    const resolutionM = resolutionForReceiverBudget(
      extent,
      sizing.paddingM,
      Math.max(budget, 1),
    );
    if (resolutionM === null || resolutionM <= sizing.resolutionM) return null;
    const shape = gridShape(extent, {
      resolutionM,
      paddingM: sizing.paddingM,
    });
    if (shape === null) return null;
    return { resolutionM, receivers: shape.count };
  };

  const grid = coarser(cpuPerReceiver);
  if (grid !== null) {
    suggestions.push({
      params: { grid_resolution_m: String(grid.resolutionM) },
      resolutionM: grid.resolutionM,
      receivers: grid.receivers,
      distanceScaled: false,
      wall: wallForReceivers(
        cpuPerReceiver,
        parallelism,
        prepareS,
        grid.receivers,
      ),
    });
  }

  if (scaled !== undefined && params.segment_length_mode !== DISTANCE_SCALED) {
    suggestions.push({
      params: { segment_length_mode: DISTANCE_SCALED },
      distanceScaled: true,
      wall: scaled.wall_s,
    });

    if (scaled.wall_s.estimate > ESTIMATE_TARGET_SECONDS) {
      const scaledPerReceiver = scale(scaled.cpu_s, 1 / receivers);
      const both = coarser(scaledPerReceiver);
      if (both !== null) {
        suggestions.push({
          params: {
            segment_length_mode: DISTANCE_SCALED,
            grid_resolution_m: String(both.resolutionM),
          },
          resolutionM: both.resolutionM,
          receivers: both.receivers,
          distanceScaled: true,
          wall: wallForReceivers(
            scaledPerReceiver,
            parallelism,
            prepareS,
            both.receivers,
          ),
        });
      }
    }
  }

  return suggestions;
}

/** The unit a band is told in, chosen by its slow end. */
export type EstimateUnit = "s" | "min" | "h";

const UNIT_SECONDS: Record<EstimateUnit, number> = { s: 1, min: 60, h: 3600 };

/**
 * A band rounded for a reader, in one unit: the ends are rounded outwards so
 * the band never narrows in the telling. The unit is the slow end's, one step
 * finer where the fast end would round to zero in it — "≈ 40–240 min" rather
 * than "≈ 0–4 h". Mirrors `runestimate.FormatBand`, the CLI's wording of the
 * same JSON.
 */
export function roundBand(
  band: EstimateBand,
): { unit: EstimateUnit; low: number; high: number } | "under-a-second" {
  if (band.high < 1) return "under-a-second";
  let unit: EstimateUnit =
    band.high < 120 ? "s" : band.high < 2 * 3600 ? "min" : "h";
  if (band.low < UNIT_SECONDS[unit]) {
    unit = unit === "h" ? "min" : "s";
  }
  const divisor = UNIT_SECONDS[unit];
  const low = Math.max(0, Math.floor(band.low / divisor));
  const high = Math.ceil(band.high / divisor);
  return { unit, low: Math.min(low, high), high };
}
