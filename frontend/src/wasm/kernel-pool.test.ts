/**
 * The arithmetic of splitting a run across kernel workers.
 *
 * No `Worker` and no WASM: jsdom has neither, and the point of keeping these
 * functions pure is that the sizing, the merge and the progress sum can be
 * pinned here while `kernel.ts` stays thin wiring. What a real pool does end
 * to end is Playwright's business — it is the only place a module Worker
 * actually runs.
 */

import { describe, expect, it } from "vitest";

import {
  MaxKernelWorkers,
  MinReceiversPerWorker,
  ShardProgress,
  mergeChunks,
  poolSize,
} from "./kernel-pool";
import type { ChunkOutput } from "./kernel-pool";
import type { ReceiverOutput } from "./types";

function outputs(from: number, count: number): ReceiverOutput[] {
  return Array.from({ length: count }, (_unused, i) => ({
    Receiver: {
      id: `R${String(from + i)}`,
      point: { x: from + i, y: 0 },
      height_m: 4,
    },
    Indicators: { lr_day: 50, lr_night: 40 },
  })) as ReceiverOutput[];
}

function chunk(index: number, start: number, count: number): ChunkOutput {
  return { chunk: index, start, outputs: outputs(start, count) };
}

describe("poolSize", () => {
  const big = MinReceiversPerWorker * 100;

  it.each([
    ["one core leaves nothing to spare", 1, big, 1],
    ["two cores keep one for the UI", 2, big, 1],
    ["four cores give three", 4, big, 3],
    ["eight cores are capped", 8, big, MaxKernelWorkers],
    ["a wildly parallel machine is still capped", 128, big, MaxKernelWorkers],
  ])("%s", (_name, cores, receivers, want) => {
    expect(poolSize(cores, receivers)).toBe(want);
  });

  it("falls back to one worker when the browser will not say", () => {
    // Some embedded webviews leave hardwareConcurrency undefined, and Firefox
    // clamps it. Neither may produce NaN workers.
    expect(poolSize(undefined, big)).toBe(1);
    expect(poolSize(Number.NaN, big)).toBe(1);
    expect(poolSize(0, big)).toBe(1);
  });

  it("does not split a run too small to pay for the split", () => {
    expect(poolSize(16, 1)).toBe(1);
    expect(poolSize(16, MinReceiversPerWorker - 1)).toBe(1);
    expect(poolSize(16, MinReceiversPerWorker)).toBe(1);
    expect(poolSize(16, MinReceiversPerWorker * 2)).toBe(2);
  });
});

describe("mergeChunks", () => {
  it("orders by chunk index, not by arrival", () => {
    // The order a Promise.all settles in is the order the workers happened to
    // finish, which is exactly what must not decide the receiver order.
    const merged = mergeChunks(
      [chunk(2, 4, 2), chunk(0, 0, 2), chunk(1, 2, 2)],
      6,
    );

    expect(merged.map((o) => o.Receiver.id)).toEqual([
      "R0",
      "R1",
      "R2",
      "R3",
      "R4",
      "R5",
    ]);
  });

  it("gives the same answer whatever order the shards report in", () => {
    const pieces = [chunk(0, 0, 3), chunk(1, 3, 3), chunk(2, 6, 1)];
    const forwards = mergeChunks(pieces, 7);
    const backwards = mergeChunks([...pieces].reverse(), 7);

    expect(backwards).toEqual(forwards);
  });

  it("merges an empty run to nothing", () => {
    expect(mergeChunks([], 0)).toEqual([]);
  });

  it("refuses a missing chunk rather than returning a short run", () => {
    // The dangerous case: a silently short array becomes a smaller raster
    // that renders plausibly and is wrong.
    expect(() => mergeChunks([chunk(0, 0, 2), chunk(2, 4, 2)], 6)).toThrow(
      /expected chunk 1/,
    );
  });

  it("refuses a duplicated chunk", () => {
    expect(() => mergeChunks([chunk(0, 0, 2), chunk(0, 0, 2)], 4)).toThrow(
      /expected chunk 1/,
    );
  });

  it("refuses a run whose last chunk never arrived", () => {
    // The hole the index and start checks cannot see: chunks 0..n-2 satisfy
    // both on their own, and the walk just stops early. Only the run's own
    // receiver count knows the answer was meant to be longer.
    expect(() => mergeChunks([chunk(0, 0, 2), chunk(1, 2, 2)], 6)).toThrow(
      /merged 4 receivers, expected 6/,
    );
  });

  it("refuses a run whose last chunk came back short", () => {
    expect(() => mergeChunks([chunk(0, 0, 2), chunk(1, 2, 1)], 4)).toThrow(
      /merged 3 receivers, expected 4/,
    );
  });

  it("refuses an empty reply for a non-empty run", () => {
    expect(() => mergeChunks([], 100)).toThrow(
      /merged 0 receivers, expected 100/,
    );
  });

  it("refuses a chunk that does not start where the previous one ended", () => {
    expect(() => mergeChunks([chunk(0, 0, 2), chunk(1, 99, 2)], 4)).toThrow(
      /starts at receiver 99/,
    );
  });
});

describe("ShardProgress", () => {
  it("sums the shards and reports against the run's own total", () => {
    const seen: [number, number][] = [];
    const progress = new ShardProgress(100, (done, total) => {
      seen.push([done, total]);
    });

    const a = progress.forShard(0);
    const b = progress.forShard(1);

    // Each shard reports its own slice's total; ShardProgress reports the
    // run's, which is what the second argument below is deliberately not.
    a(10, 50);
    b(20, 50);
    a(30, 50);

    // The total is 100 from the first report, not the sum of whichever shard
    // totals have been heard so far — otherwise the bar jumps as shards check
    // in, which reads as a run restarting.
    expect(seen).toEqual([
      [10, 100],
      [30, 100],
      [50, 100],
    ]);
  });

  it("never reports more than the total", () => {
    const seen: [number, number][] = [];
    const progress = new ShardProgress(10, (done, total) => {
      seen.push([done, total]);
    });

    progress.forShard(0)(7, 5);
    progress.forShard(1)(7, 5);

    expect(seen.at(-1)).toEqual([10, 10]);
  });

  it("advances monotonically as shards report out of turn", () => {
    const seen: number[] = [];
    const progress = new ShardProgress(300, (done) => {
      seen.push(done);
    });

    const shards = [0, 1, 2].map((i) => progress.forShard(i));

    shards[2]?.(10, 100);
    shards[0]?.(50, 100);
    shards[1]?.(5, 100);
    shards[2]?.(90, 100);
    shards[0]?.(100, 100);

    expect(seen).toEqual([...seen].sort((a, b) => a - b));
    expect(seen.at(-1)).toBe(195);
  });

  it("works without a listener, so a run that wants no progress costs nothing", () => {
    const progress = new ShardProgress(10);
    expect(() => {
      progress.forShard(0)(5, 10);
    }).not.toThrow();
  });
});
