// What a run tells the console about itself while it is running.
//
// # Why this exists
//
// A progress bar answers "how far", and it answers it only once the first
// receiver is done. Until then a run is indistinguishable from a hung tab,
// and on the scenes that most need explaining it stays that way for minutes:
// an RLS-19 receiver costs well under a millisecond over open ground and
// upwards of ten *seconds* inside a building-dense OSM extract, because
// Nr. 3.5 gives every Spiegelschallquelle its own diffraction search and the
// count of those grows faster than the building count does.
//
// So the bar alone cannot say the one thing a user in that position needs to
// know, which is not "how far" but "is this going to finish, and why is it
// like this". That needs the run's *shape* — how many receivers against how
// many Teilstücke against how many buildings — and it needs a projected
// finish as soon as one receiver has been timed. Both are printed here.
//
// # This computes nothing the kernel computes
//
// {@link teilstueckCount} reproduces `SplitLineIntoSegments`' count, and that
// is a duplicated rule, so keep it honest about what it is: a log line. It
// must never reach a request, a result or a UI number. The kernel remains the
// only thing that decides where a Teilstück falls; this only says how many
// there will be, which is the single number that explains the run's cost.

import type { ComputeRequest, Point2D, RoadSource } from "./types";

/** How often a run in flight repeats itself to the console, at most. */
const HeartbeatMS = 5000;

/** The plan length of a polyline, in the CRS the request is already in. */
function polylineLength(line: Point2D[]): number {
  let total = 0;

  for (let i = 1; i < line.length; i += 1) {
    const from = line[i - 1];
    const to = line[i];

    if (from === undefined || to === undefined) continue;

    total += Math.hypot(to.x - from.x, to.y - from.y);
  }

  return total;
}

/**
 * How many Teilstücke one source will be split into.
 *
 * `max(ceil(length / target), 1)`, which is `SplitLineIntoSegments` in
 * `road/propagation.go` — including its floor, so a source shorter than one
 * Teilstück still counts as one rather than as none. A line of under two
 * points, or a degenerate one, produces no segments there and none here.
 */
function sourceSegments(source: RoadSource, targetLengthM: number): number {
  if (source.centerline.length < 2 || targetLengthM <= 0) return 0;

  const length = polylineLength(source.centerline);
  if (!(length > 0)) return 0;

  return Math.max(Math.ceil(length / targetLengthM), 1);
}

/** How many Teilstücke the whole request will be split into. */
export function teilstueckCount(req: ComputeRequest): number {
  const target = req.config?.SegmentLengthM ?? 1;

  return req.sources.reduce(
    (total, source) => total + sourceSegments(source, target),
    0,
  );
}

/** The shape of a run, as the numbers that decide what it will cost. */
export interface RunShape {
  receivers: number;
  sources: number;
  /** Teilstücke over every source, at this request's segment length. */
  segments: number;
  segmentLengthM: number;
  buildings: number;
  barriers: number;
  workers: number;
}

export function describeRun(req: ComputeRequest, workers: number): RunShape {
  return {
    receivers: req.receivers.length,
    sources: req.sources.length,
    segments: teilstueckCount(req),
    segmentLengthM: req.config?.SegmentLengthM ?? 1,
    buildings: req.config?.Buildings?.length ?? 0,
    barriers: req.barriers.length,
    workers,
  };
}

/** A duration a person can read at a glance, from milliseconds. */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "unknown";

  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${String(seconds)}s`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${String(minutes)}m ${String(seconds % 60)}s`;

  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${String(hours)}h ${String(minutes % 60)}m`;

  return `${String(Math.floor(hours / 24))}d ${String(hours % 24)}h`;
}

/** Where the console output goes. Injected so a test can read it. */
export interface DiagnosticsSink {
  info(message: string): void;
  warn(message: string): void;
}

const consoleSink: DiagnosticsSink = {
  info: (message) => {
    console.info(message);
  },
  warn: (message) => {
    console.warn(message);
  },
};

/**
 * Narrates one run to the console.
 *
 * The interesting line is {@link progress}' first one. Everything before a
 * receiver has been computed is a guess — the scene has to be prepared before
 * anything can be timed, and preparation is not proportional to the walk — so
 * the run's cost is stated as a measurement the moment there is one, and not
 * before.
 *
 * `now` is injected because the whole class is a clock, and a test that had to
 * wait five seconds for the heartbeat would be a test of `setTimeout`.
 */
export class RunDiagnostics {
  private readonly startedAt: number;
  private firstReportAt: number | null = null;
  private lastHeartbeatAt = 0;
  private warned = false;

  constructor(
    private readonly shape: RunShape,
    private readonly now: () => number = () => Date.now(),
    private readonly sink: DiagnosticsSink = consoleSink,
  ) {
    this.startedAt = this.now();
    this.sink.info(`[aconiq] run started — ${this.describe()}`);
  }

  private describe(): string {
    const s = this.shape;
    const pairs = s.receivers * s.segments;

    return (
      `${String(s.receivers)} receivers x ${String(s.segments)} Teilstücke ` +
      `(${String(s.sources)} sources at ${String(s.segmentLengthM)} m) ` +
      `= ${pairs.toExponential(2)} source-receiver pairs; ` +
      `${String(s.buildings)} buildings, ${String(s.barriers)} barriers; ` +
      `${String(s.workers)} worker${s.workers === 1 ? "" : "s"}`
    );
  }

  /**
   * A progress report from the run.
   *
   * `done === 0` is the dispatch marker the pool sends before any work has
   * happened, not a measurement, so it is ignored: dividing by it would
   * report a rate of zero and an infinite finish.
   */
  progress(done: number, total: number): void {
    if (done <= 0) return;

    const at = this.now();
    const elapsed = at - this.startedAt;

    if (this.firstReportAt === null) {
      this.firstReportAt = at;

      const perReceiver = elapsed / done;

      this.sink.info(
        `[aconiq] first ${String(done)} receiver${done === 1 ? "" : "s"} after ` +
          `${formatDuration(elapsed)} (scene preparation included) — ` +
          `about ${formatDuration(perReceiver)} per receiver`,
      );
    }

    const remaining = total - done;
    const eta = remaining * (elapsed / done);

    if (!this.warned && eta > 10 * 60 * 1000) {
      this.warned = true;
      this.sink.warn(
        `[aconiq] this run is projected to take ${formatDuration(eta)}. ${this.advice()}`,
      );
    }

    if (at - this.lastHeartbeatAt < HeartbeatMS) return;

    this.lastHeartbeatAt = at;

    this.sink.info(
      `[aconiq] ${String(done)}/${String(total)} receivers ` +
        `(${((done / total) * 100).toFixed(1)}%) — ` +
        `${formatDuration(elapsed)} elapsed, about ${formatDuration(eta)} left`,
    );
  }

  /**
   * What to change to make a run like this one finish.
   *
   * Measured rather than assumed, and the ranking is not the obvious one. On
   * a 25 km network at a 25 m Teilstück length one receiver costs 555 µs with
   * no buildings in the scene and 2.9 *seconds* with a hundred — a factor of
   * five thousand, because RLS-19 Nr. 3.6 requires first- and second-order
   * reflections and Nr. 3.5 gives every resulting Spiegelschallquelle its own
   * diffraction search. Receiver count and Teilstück count are each roughly
   * linear on top of that.
   *
   * So a building-dense model is told about its buildings first: halving the
   * grid resolution there buys a factor of four against a term that is
   * already thousands, and the honest lever is a smaller calculation area.
   */
  private advice(): string {
    const s = this.shape;

    if (s.buildings > 0) {
      return (
        `${String(s.buildings)} buildings are in the scene, and reflections off them ` +
        `dominate the cost — each one is a barrier and a reflector at once. ` +
        `A smaller calculation area is the effective lever; raising ` +
        `segment_length_m (currently ${String(s.segmentLengthM)} m) and the grid ` +
        `resolution each help roughly proportionally.`
      );
    }

    return (
      `Cost is receivers x Teilstücke: raising segment_length_m (currently ` +
      `${String(s.segmentLengthM)} m) or the grid resolution cuts it roughly ` +
      `proportionally.`
    );
  }

  finished(): void {
    this.sink.info(
      `[aconiq] run finished in ${formatDuration(this.now() - this.startedAt)}`,
    );
  }

  failed(reason: unknown): void {
    const what = reason instanceof Error ? reason.message : String(reason);

    this.sink.warn(
      `[aconiq] run ended after ${formatDuration(this.now() - this.startedAt)} without a result: ${what}`,
    );
  }
}
