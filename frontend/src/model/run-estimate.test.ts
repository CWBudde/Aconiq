import { describe, expect, it } from "vitest";
import { gridShape } from "./grid-estimate";
import {
  ESTIMATE_TARGET_SECONDS,
  roundBand,
  suggestFasterRun,
  type RunEstimate,
} from "./run-estimate";

/** A 1 km square at 10 m: 105 × 105 = 11 025 receivers. */
const EXTENT = { minX: 0, minY: 0, maxX: 1000, maxY: 1000 };
const SIZING = { resolutionM: 10, paddingM: 20 };

/** 11 025 receivers at 2 s CPU each on 8 workers: ~46 min. */
function longEstimate(withScaled = true): RunEstimate {
  const receivers = 11_025;
  const cpuPerReceiver = { low: 1.5, estimate: 2, high: 3 };
  const wall = (receivers * 2) / 8;
  return {
    available: true,
    standard_id: "rls19-road",
    wall_s: { low: wall * 0.7, estimate: wall, high: wall * 1.6 },
    cpu_s: {
      low: receivers * 1.5,
      estimate: receivers * 2,
      high: receivers * 3,
    },
    cpu_per_receiver_s: cpuPerReceiver,
    parallelism: 8,
    inputs: { receivers, workers: 8, units: receivers * 4000 },
    probe: {
      probe_receivers: 16,
      measured_receivers: 16,
      units_timed: 400,
      budget_s: 3,
      elapsed_s: 3,
      prepare_s: 0.5,
      budget_exhausted: true,
      clock: "thread_cpu",
    },
    ...(withScaled
      ? {
          alternatives: [
            {
              params: { segment_length_mode: "distance_scaled" },
              measured: true,
              units: receivers * 200,
              cpu_s: {
                low: receivers * 0.1,
                estimate: receivers * 0.15,
                high: receivers * 0.25,
              },
              wall_s: { low: 150, estimate: 210, high: 340 },
            },
          ],
        }
      : {}),
  };
}

describe("suggestFasterRun", () => {
  it("offers nothing for a run under the warning threshold", () => {
    const short: RunEstimate = {
      ...longEstimate(),
      wall_s: { low: 100, estimate: 120, high: 200 },
    };
    expect(suggestFasterRun(short, EXTENT, SIZING, {})).toEqual([]);
  });

  it("offers a coarser grid whose receiver count is the run's own arithmetic", () => {
    const [grid] = suggestFasterRun(longEstimate(false), EXTENT, SIZING, {});
    expect(grid?.resolutionM).toBeGreaterThan(SIZING.resolutionM);
    const shape = gridShape(EXTENT, {
      resolutionM: grid?.resolutionM ?? 0,
      paddingM: SIZING.paddingM,
    });
    expect(grid?.receivers).toBe(shape?.count);
    expect(grid?.params).toEqual({
      grid_resolution_m: String(grid?.resolutionM),
    });
    // Under a minute at the point estimate, which is what it was chosen for.
    expect(grid?.wall.estimate).toBeLessThanOrEqual(ESTIMATE_TARGET_SECONDS);
  });

  it("offers distance-scaled Teilstücke, then both together", () => {
    const suggestions = suggestFasterRun(longEstimate(), EXTENT, SIZING, {});
    const scaled = suggestions.find(
      (s) => s.distanceScaled && s.resolutionM === undefined,
    );
    expect(scaled?.params).toEqual({ segment_length_mode: "distance_scaled" });
    expect(scaled?.wall.estimate).toBe(210);

    const both = suggestions.find(
      (s) => s.distanceScaled && s.resolutionM !== undefined,
    );
    expect(both?.params.segment_length_mode).toBe("distance_scaled");
    // Cheaper per receiver, so a finer grid fits the same minute.
    const grid = suggestions.find((s) => !s.distanceScaled);
    expect(both?.resolutionM).toBeLessThanOrEqual(grid?.resolutionM ?? 0);
  });

  it("offers no distance-scaled run the budget never measured", () => {
    const estimate: RunEstimate = {
      ...longEstimate(),
      alternatives: [
        { params: { segment_length_mode: "distance_scaled" }, measured: false },
      ],
    };
    const suggestions = suggestFasterRun(estimate, EXTENT, SIZING, {});
    expect(suggestions.some((s) => s.distanceScaled)).toBe(false);
    expect(suggestions).toHaveLength(1);
  });

  it("offers no spacing for a custom receiver set, and no mode it already has", () => {
    const suggestions = suggestFasterRun(longEstimate(), null, SIZING, {
      segment_length_mode: "distance_scaled",
    });
    expect(suggestions).toEqual([]);
  });

  it("offers nothing when there is no estimate", () => {
    expect(
      suggestFasterRun(
        { available: false, reason: "no probe" },
        EXTENT,
        SIZING,
        {},
      ),
    ).toEqual([]);
  });
});

describe("roundBand", () => {
  it("rounds outwards in the slow end's unit", () => {
    expect(roundBand({ low: 390, estimate: 420, high: 492 })).toEqual({
      unit: "min",
      low: 6,
      high: 9,
    });
  });

  it("steps down a unit where the fast end would read as zero", () => {
    expect(roundBand({ low: 2400, estimate: 7200, high: 14_400 })).toEqual({
      unit: "min",
      low: 40,
      high: 240,
    });
  });

  it("says under a second for a run that is", () => {
    expect(roundBand({ low: 0.1, estimate: 0.2, high: 0.4 })).toBe(
      "under-a-second",
    );
  });
});
