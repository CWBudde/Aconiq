// How a run is spread across several kernel workers, and how the pieces come
// back together.
//
// This file holds only the parts that are pure: how many workers a run gets,
// how per-shard progress adds up, and how per-shard results merge. The wiring
// — spawning, terminating, replaying terrain — lives in `kernel.ts`, and the
// Go side of the split lives in `wasmkernel`. Keeping the arithmetic here is
// what lets it be tested without a real `Worker`, which jsdom does not have.
//
// # Why a pool at all
//
// A WebAssembly module has one stack and `GOMAXPROCS` is 1 under `js/wasm`, so
// goroutines inside the kernel buy nothing. The only parallelism available to
// the browser is more Workers, each with its own instance and its own linear
// memory. Receivers are independent and there is no reduction across them, so
// the split changes no floating-point operation and no floating-point order —
// only which thread runs an unchanged sequence.

import type { ReceiverOutput } from "./types";

/**
 * The most workers a run will ever use.
 *
 * Each one is a separate Go linear memory holding the module plus a
 * scene-sized heap — tens of megabytes apiece. Four of those is already the
 * risk on a 2 GB mobile tab, and a tab the browser kills for memory takes the
 * model's pending IndexedDB write with it, which is a far worse outcome than a
 * slow run. Past four the marginal shard is also smaller than the fixed cost
 * of starting it: one WASM instantiation plus one `PrepareScene` plus one
 * parse of the request.
 */
export const MaxKernelWorkers = 4;

/**
 * The least work a worker has to be given before it is worth starting.
 *
 * Below this the shard's share of the walk costs less than the scene
 * preparation it has to repeat, and the run would be slower for having been
 * split. The figure comes from `BenchmarkPrepareScene` against the per-receiver
 * cost in `BenchmarkComputeReceiverOutputs`, not from taste — revisit it when
 * either moves.
 */
export const MinReceiversPerWorker = 2000;

/**
 * How many workers to run a job of this size on.
 *
 * `cores` is `navigator.hardwareConcurrency`, which is a hint and not a
 * budget: it reports the machine rather than this tab's share of it, some
 * embedded webviews leave it undefined, and Firefox clamps it under
 * resist-fingerprinting. So it is read defensively and capped hard.
 *
 * One core is left for the main thread. MapLibre, React and the progress bar
 * all live there, and a pool that saturates every core makes the UI stutter —
 * which is the thing the worker was introduced to fix in the first place.
 */
export function poolSize(cores: number | undefined, receivers: number): number {
  const usable =
    typeof cores === "number" && Number.isFinite(cores) ? Math.floor(cores) : 1;

  const byHardware = Math.min(Math.max(usable - 1, 1), MaxKernelWorkers);
  const byWork = Math.max(Math.floor(receivers / MinReceiversPerWorker), 1);

  return Math.min(byHardware, byWork);
}

/**
 * One partition chunk's results, as a shard reports them.
 *
 * `chunk` is the index the merge orders by — never the order shards happened
 * to finish in. It mirrors `partition.Chunk.Index` on the Go side.
 */
export interface ChunkOutput {
  chunk: number;
  start: number;
  outputs: ReceiverOutput[];
}

/**
 * Put the shards' chunks back into receiver order.
 *
 * Sorting by `chunk` rather than concatenating in arrival order is the whole
 * determinism argument: `docs/policies/determinism.md` requires partial
 * results to merge "in a fixed order independent of worker scheduling", with
 * no "first finished worker wins".
 *
 * A gap or a duplicate throws rather than returning a short array. A short
 * array is the dangerous failure here — it becomes a smaller raster that
 * renders perfectly plausibly and is wrong, and nothing downstream would
 * notice.
 */
export function mergeChunks(chunks: ChunkOutput[]): ReceiverOutput[] {
  const ordered = [...chunks].sort((a, b) => a.chunk - b.chunk);
  const merged: ReceiverOutput[] = [];

  ordered.forEach((chunk, position) => {
    if (chunk.chunk !== position) {
      throw new Error(
        `kernel pool: expected chunk ${String(position)}, got ${String(chunk.chunk)}` +
          ` — a shard's results are missing or arrived twice`,
      );
    }

    if (chunk.start !== merged.length) {
      throw new Error(
        `kernel pool: chunk ${String(chunk.chunk)} starts at receiver ${String(chunk.start)},` +
          ` but ${String(merged.length)} receivers precede it`,
      );
    }

    merged.push(...chunk.outputs);
  });

  return merged;
}

/** What a caller is told as the run advances: receivers done, receivers total. */
export type ProgressListener = (done: number, total: number) => void;

/**
 * Adds the shards' separate progress reports into one.
 *
 * Each shard counts only its own receivers, because a shard cannot see what
 * the others have finished. The sum is monotone because each shard's own count
 * is, and it lands exactly on the total because the shards partition the
 * receiver set.
 *
 * The total reported is the run's own, not the sum of the shards' totals.
 * They are equal once every shard has reported once — but before that, summing
 * what has been heard so far would make the bar jump as each shard checks in,
 * which is precisely the "is this stuck?" reading the determinate bar exists
 * to prevent.
 */
export class ShardProgress {
  // A Map rather than an array indexed by shard: shards report in whatever
  // order they get there, so an array would be sparse until every one has
  // checked in, and a hole reads as `undefined` at runtime while the type says
  // `number`.
  private readonly done = new Map<number, number>();

  constructor(
    private readonly total: number,
    private readonly sink?: ProgressListener,
  ) {}

  /**
   * The listener to hand shard `index`.
   *
   * The shard's own total is ignored: it is this shard's slice of the run, and
   * what a caller wants to see is the run.
   */
  forShard(index: number): ProgressListener {
    return (done) => {
      this.done.set(index, done);
      this.sink?.(this.sum(), this.total);
    };
  }

  private sum(): number {
    let n = 0;

    for (const done of this.done.values()) {
      n += done;
    }

    // Clamped because a caller rendering done/total must never be handed
    // something above 1. The arithmetic should not overshoot — the shards
    // partition the receiver set — but a miscounted shard must degrade to a
    // full bar, not to a bar running off the end of its track.
    return Math.min(n, this.total);
  }
}
