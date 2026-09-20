/**
 * What a run says about itself while it runs.
 *
 * The clock and the sink are both injected, so none of this waits for a
 * heartbeat or reads the real console — the whole class is a clock, and a
 * test that slept through it would be testing `setTimeout`.
 */

import { describe, expect, it } from "vitest";

import {
  RunDiagnostics,
  type DiagnosticsSink,
  type RunShape,
  describeRun,
  formatDuration,
  teilstueckCount,
} from "./kernel-diagnostics";
import type {
  Building,
  ComputeRequest,
  Point2D,
  PropagationConfig,
  RoadSource,
} from "./types";

function source(id: string, centerline: Point2D[]): RoadSource {
  return {
    id,
    centerline,
    surface_type: "SMA",
    speeds: { pkw_kph: 50, lkw1_kph: 50, lkw2_kph: 50, krad_kph: 50 },
    traffic_day: {
      pkw_per_hour: 100,
      lkw1_per_hour: 5,
      lkw2_per_hour: 5,
      krad_per_hour: 1,
    },
    traffic_night: {
      pkw_per_hour: 20,
      lkw1_per_hour: 1,
      lkw2_per_hour: 1,
      krad_per_hour: 0,
    },
  };
}

function config(
  segmentLengthM: number,
  buildings: Building[] = [],
): PropagationConfig {
  return {
    SegmentLengthM: segmentLengthM,
    MinDistanceM: 1,
    ReceiverHeightM: 4,
    Buildings: buildings,
  };
}

function request(
  sources: RoadSource[],
  segmentLengthM: number,
  receivers = 10,
): ComputeRequest {
  return {
    receivers: Array.from({ length: receivers }, (_unused, i) => ({
      id: `R${String(i)}`,
      point: { x: i, y: 0 },
      height_m: 4,
    })),
    sources,
    barriers: [],
    config: config(segmentLengthM),
  };
}

function recordingSink(): DiagnosticsSink & { lines: string[] } {
  const lines: string[] = [];

  return {
    lines,
    info: (message) => lines.push(message),
    warn: (message) => lines.push(message),
  };
}

describe("teilstueckCount", () => {
  it("counts a straight line the way SplitLineIntoSegments does", () => {
    // max(ceil(length / target), 1) — the rule in road/propagation.go.
    const line = [
      { x: 0, y: 0 },
      { x: 100, y: 0 },
    ];

    expect(teilstueckCount(request([source("a", line)], 1))).toBe(100);
    expect(teilstueckCount(request([source("a", line)], 10))).toBe(10);
    expect(teilstueckCount(request([source("a", line)], 30))).toBe(4);
  });

  it("follows the polyline rather than the straight line between its ends", () => {
    // A road that doubles back is twice the Teilstücke of its bounding span,
    // which is the whole reason this walks the vertices.
    const dogleg = [
      { x: 0, y: 0 },
      { x: 50, y: 0 },
      { x: 50, y: 50 },
    ];

    expect(teilstueckCount(request([source("a", dogleg)], 1))).toBe(100);
  });

  it("sums over sources, which is what makes a city expensive", () => {
    const line = [
      { x: 0, y: 0 },
      { x: 1000, y: 0 },
    ];
    const sources = Array.from({ length: 25 }, (_u, i) =>
      source(`s${String(i)}`, line),
    );

    expect(teilstueckCount(request(sources, 1))).toBe(25_000);
  });

  it("gives a source shorter than one Teilstück the floor of one", () => {
    const stub = [
      { x: 0, y: 0 },
      { x: 2, y: 0 },
    ];

    expect(teilstueckCount(request([source("a", stub)], 25))).toBe(1);
  });

  it("counts nothing for a degenerate source", () => {
    expect(teilstueckCount(request([source("a", [{ x: 0, y: 0 }])], 1))).toBe(
      0,
    );
    expect(
      teilstueckCount(
        request(
          [
            source("a", [
              { x: 7, y: 7 },
              { x: 7, y: 7 },
            ]),
          ],
          1,
        ),
      ),
    ).toBe(0);
  });

  it("defaults to the kernel's own 1 m when the request names no length", () => {
    const req = request(
      [
        source("a", [
          { x: 0, y: 0 },
          { x: 60, y: 0 },
        ]),
      ],
      1,
    );
    delete req.config;

    expect(teilstueckCount(req)).toBe(60);
  });
});

describe("formatDuration", () => {
  it.each([
    [0, "0s"],
    [1500, "2s"],
    [95_000, "1m 35s"],
    [3_600_000, "1h 0m"],
    [38_100_000, "10h 35m"],
    [172_800_000, "2d 0h"],
  ])("%d ms reads as %s", (ms, want) => {
    expect(formatDuration(ms)).toBe(want);
  });

  it("does not invent a number it does not have", () => {
    expect(formatDuration(Number.NaN)).toBe("unknown");
    expect(formatDuration(Number.POSITIVE_INFINITY)).toBe("unknown");
  });
});

describe("describeRun", () => {
  it("reports the numbers that decide the cost", () => {
    const req = request(
      [
        source("a", [
          { x: 0, y: 0 },
          { x: 2000, y: 0 },
        ]),
      ],
      1,
      9202,
    );
    req.config = config(1, [{ id: "b", footprint: [], height_m: 9 }]);

    expect(describeRun(req, 4)).toEqual<RunShape>({
      receivers: 9202,
      sources: 1,
      segments: 2000,
      segmentLengthM: 1,
      segmentLengthMode: "fixed",
      buildings: 1,
      barriers: 0,
      workers: 4,
    });
  });

  // An unset mode is fixed, which is the zero value the Go config carries.
  it("reads the mode off the request", () => {
    const req = request([], 1, 1);
    req.config = { ...config(1), SegmentLengthMode: "distance_scaled" };

    expect(describeRun(req, 1).segmentLengthMode).toBe("distance_scaled");
  });
});

describe("RunDiagnostics", () => {
  const shape: RunShape = {
    receivers: 1000,
    sources: 2,
    segments: 2000,
    segmentLengthM: 1,
    segmentLengthMode: "fixed",
    buildings: 100,
    barriers: 0,
    workers: 4,
  };

  const scaled: RunShape = { ...shape, segmentLengthMode: "distance_scaled" };

  function at(times: number[]): () => number {
    let i = 0;

    return () => times[Math.min(i++, times.length - 1)] ?? 0;
  }

  it("states the run's shape before anything has been computed", () => {
    const sink = recordingSink();
    new RunDiagnostics(shape, at([0]), sink);

    expect(sink.lines[0]).toContain("1000 receivers x 2000 Teilstücke");
    expect(sink.lines[0]).toContain("100 buildings");
    expect(sink.lines[0]).toContain("4 workers");
  });

  it("ignores the dispatch marker, which is not a measurement", () => {
    // done === 0 is the pool announcing a run, not timing one. Dividing by it
    // would report an infinite finish before a receiver had been computed.
    const sink = recordingSink();
    const d = new RunDiagnostics(shape, at([0, 100]), sink);

    d.progress(0, 1000);

    expect(sink.lines).toHaveLength(1);
  });

  it("times the first receiver, preparation included", () => {
    const sink = recordingSink();
    const d = new RunDiagnostics(shape, at([0, 40_000]), sink);

    d.progress(1, 1000);

    expect(sink.lines[1]).toContain("first 1 receiver after 40s");
    expect(sink.lines[1]).toContain("about 40s per receiver");
  });

  it("warns once when the projection runs past ten minutes", () => {
    const sink = recordingSink();
    const d = new RunDiagnostics(shape, at([0, 40_000, 80_000]), sink);

    d.progress(1, 1000);
    d.progress(2, 1000);

    const warnings = sink.lines.filter((line) => line.includes("projected"));

    expect(warnings).toHaveLength(1);
    expect(warnings[0]).toContain("segment_length_m");
  });

  it("names the buildings first when there are any", () => {
    // The measured ranking, not the intuitive one: on a 25 km network a
    // receiver costs 555 µs with no buildings and 2.9 s with a hundred, so
    // the grid resolution is the *second* thing to talk about.
    const sink = recordingSink();
    const d = new RunDiagnostics(shape, at([0, 600_000]), sink);

    d.progress(1, 1000);

    const warning = sink.lines.find((line) => line.includes("projected"));

    expect(warning).toContain("100 buildings");
    expect(warning).toContain("reflections");
  });

  it("talks about Teilstücke alone when the scene is open ground", () => {
    const sink = recordingSink();
    const d = new RunDiagnostics(
      { ...shape, buildings: 0 },
      at([0, 600_000]),
      sink,
    );

    d.progress(1, 1000);

    const warning = sink.lines.find((line) => line.includes("projected"));

    expect(warning).toContain("receivers x Teilstücke");
    expect(warning).not.toContain("reflections");
  });

  // The pair count is the finest split. Under distance_scaled the kernel
  // walks a coarser rung per receiver, so printing the number unqualified
  // overstated the work by up to 39x on the extract that motivated the mode.
  it("calls the pair count a ceiling under distance_scaled", () => {
    const sink = recordingSink();
    new RunDiagnostics(scaled, at([0]), sink);

    expect(sink.lines[0]).toContain("segment_length_mode=distance_scaled");
    expect(sink.lines[0]).toContain("at most");
  });

  it("states the pair count plainly under fixed", () => {
    const sink = recordingSink();
    new RunDiagnostics(shape, at([0]), sink);

    expect(sink.lines[0]).toContain("segment_length_mode=fixed");
    expect(sink.lines[0]).not.toContain("at most");
  });

  // Measured: under distance_scaled the bound l_i <= s_i / 2 already decides
  // the split of everything far enough away to matter, and raising
  // segment_length_m from 1 m to 25 m moved the pair count by under 20%. So
  // the advice that helps in fixed mode is advice to spend accuracy for
  // nothing here, and must not be given.
  it("does not offer segment_length_m as a lever under distance_scaled", () => {
    const sink = recordingSink();
    const d = new RunDiagnostics(scaled, at([0, 600_000]), sink);

    d.progress(1, 1000);

    const warning = sink.lines.find((line) => line.includes("projected"));

    expect(warning).toContain("grid resolution");
    expect(warning).toContain("distance_scaled already bounds");
    expect(warning).not.toContain("raising segment_length_m");
  });

  it("stays quiet between heartbeats", () => {
    const sink = recordingSink();
    // The first report always prints; the second is a second later, inside
    // the heartbeat window, so it must not.
    const d = new RunDiagnostics(shape, at([0, 1000, 2000]), sink);

    d.progress(500, 1000);
    const afterFirst = sink.lines.length;
    d.progress(501, 1000);

    expect(sink.lines).toHaveLength(afterFirst);
  });

  it("speaks again once the heartbeat window has passed", () => {
    const sink = recordingSink();
    const d = new RunDiagnostics(shape, at([0, 1000, 20_000]), sink);

    d.progress(500, 1000);
    const afterFirst = sink.lines.length;
    d.progress(600, 1000);

    expect(sink.lines.length).toBeGreaterThan(afterFirst);
    expect(sink.lines.at(-1)).toContain("600/1000");
  });

  it("reports a finish and a failure with the time they took", () => {
    const finished = recordingSink();
    new RunDiagnostics(shape, at([0, 5000]), finished).finished();
    expect(finished.lines.at(-1)).toContain("run finished in 5s");

    const failed = recordingSink();
    new RunDiagnostics(shape, at([0, 5000]), failed).failed(
      new Error("cancelled"),
    );
    expect(failed.lines.at(-1)).toContain("cancelled");
  });
});
