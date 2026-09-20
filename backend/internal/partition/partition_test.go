package partition_test

import (
	"testing"

	"github.com/aconiq/backend/internal/partition"
)

// The properties every caller relies on, checked over a wide spread of shapes
// rather than a handful of hand-picked ones: a partition that is gapless,
// non-overlapping and ascending is what lets the merge be a concatenation by
// Chunk.Index, and that concatenation is the whole bit-identity argument for
// parallel compute.
func TestOverCoversEveryReceiverExactlyOnce(t *testing.T) {
	t.Parallel()

	totals := []int{0, 1, 2, 63, 64, 65, 100, 1000, 9973, 250000}
	sizes := []int{1, 7, 64, 128, 4096, 1000000}

	for _, total := range totals {
		for _, size := range sizes {
			chunks := partition.Over(total, size)

			if total == 0 {
				if len(chunks) != 0 {
					t.Fatalf("Over(0, %d) returned %d chunks, want none", size, len(chunks))
				}

				continue
			}

			next := 0

			for i, c := range chunks {
				if c.Index != i {
					t.Fatalf("Over(%d, %d): chunk %d carries Index %d", total, size, i, c.Index)
				}

				if c.Start != next {
					t.Fatalf("Over(%d, %d): chunk %d starts at %d, want %d (gap or overlap)",
						total, size, i, c.Start, next)
				}

				if c.End <= c.Start {
					t.Fatalf("Over(%d, %d): chunk %d is empty: [%d,%d)", total, size, i, c.Start, c.End)
				}

				next = c.End
			}

			if next != total {
				t.Fatalf("Over(%d, %d) covers %d receivers, want %d", total, size, next, total)
			}

			if got := partition.Receivers(chunks); got != total {
				t.Fatalf("Receivers = %d, want %d", got, total)
			}
		}
	}
}

// Size is the line docs/policies/determinism.md's "fixed partitioning derived
// from immutable input ordering" rests on, so it must answer from its two
// arguments and nothing else. Calling it repeatedly is the cheapest check that
// nothing time- or environment-derived crept in.
func TestSizeIsPureInItsArguments(t *testing.T) {
	t.Parallel()

	for total := range 300 {
		for workers := range 9 {
			first := partition.Size(total, workers)

			for range 4 {
				if got := partition.Size(total, workers); got != first {
					t.Fatalf("Size(%d, %d) returned %d then %d", total, workers, first, got)
				}
			}

			if total > 0 && first <= 0 {
				t.Fatalf("Size(%d, %d) = %d, want a positive chunk", total, workers, first)
			}

			if total > 0 && first > total {
				t.Fatalf("Size(%d, %d) = %d, larger than the run", total, workers, first)
			}
		}
	}
}

// A single receiver is one chunk, because there is nothing to split. This is
// the only run size that may come out single-chunked over a pool: see
// TestSizeNeverStarvesThePool for why the rest may not.
func TestSizeKeepsAOneReceiverRunWhole(t *testing.T) {
	t.Parallel()

	for _, workers := range []int{1, 2, 8, 64} {
		size := partition.Size(1, workers)

		if chunks := partition.Over(1, size); len(chunks) != 1 {
			t.Fatalf("Size(1, %d): a 1-receiver run split into %d chunks, want 1", workers, len(chunks))
		}
	}
}

// Every worker must have a chunk to take, whenever there are receivers enough
// to give it one.
//
// This is the property MinChunk used to violate. The floor was written to stop
// the partition choosing chunks so small that the per-chunk bookkeeping
// outweighed the work inside them, which is a real concern at 250 000 cheap
// receivers and no concern at all at 130 expensive ones — but it was applied
// as an unconditional floor, so it also decided the chunk *count* for every
// run below workers*MinChunk receivers, and decided it far too low.
//
// A 130-receiver grid over a city extract is the case that found it: chunk
// size 64 gave three chunks, so three of twelve goroutines ran and the wall
// clock was one goroutine walking 64 receivers in series. Receiver count is
// not receiver cost — the same 130 receivers are milliseconds over open ground
// and minutes inside an OSM import — so the partition cannot use the count to
// decide it is not worth parallelising.
func TestSizeNeverStarvesThePool(t *testing.T) {
	t.Parallel()

	totals := []int{2, 7, 13, 49, 64, 65, 130, 999, 9973, 250000}
	workerCounts := []int{2, 3, 4, 8, 12, 16, 64}

	for _, total := range totals {
		for _, workers := range workerCounts {
			chunks := partition.Over(total, partition.Size(total, workers))

			if want := min(total, workers); len(chunks) < want {
				t.Fatalf("Size(%d, %d) gave %d chunks over %d workers, want at least %d: %d workers would idle",
					total, workers, len(chunks), workers, want, workers-len(chunks))
			}
		}
	}
}

// The exact shape of the run that found the defect, kept as its own case so a
// regression names itself rather than arriving as one row of a property test.
func TestSizeSpreadsTheCityExtractGridOverEveryWorker(t *testing.T) {
	t.Parallel()

	const (
		receivers = 130 // an 86 m x 57 m calculation area on a 10 m grid
		workers   = 12
	)

	chunks := partition.Over(receivers, partition.Size(receivers, workers))

	if len(chunks) < workers {
		t.Fatalf("130 receivers over 12 workers gave %d chunks, want at least %d", len(chunks), workers)
	}

	// The longest chunk sets the wall clock, and at minutes per receiver the
	// difference between 11 and 64 is the difference between a run and an
	// afternoon.
	longest := 0
	for _, c := range chunks {
		longest = max(longest, c.Len())
	}

	if longest > 11 {
		t.Fatalf("longest chunk covers %d receivers, want at most 11 (it was 64 before the clamp)", longest)
	}
}

// The floor still does its job where it was meant to: with receivers enough to
// go round, chunks stay big enough that the bookkeeping disappears against the
// work inside them.
func TestSizeStillPrefersLargeChunksOnALargeRun(t *testing.T) {
	t.Parallel()

	if size := partition.Size(250000, 12); size < partition.MinChunk {
		t.Fatalf("Size(250000, 12) = %d, want at least MinChunk (%d)", size, partition.MinChunk)
	}
}

// Sharding assigns chunks; it never redefines them. Every chunk must be
// computed by exactly one shard, and the union must be the same partition a
// single shard would have walked — which is what makes the shard count
// invisible in the output.
func TestForShardPartitionsTheSameChunks(t *testing.T) {
	t.Parallel()

	const total = 9973

	for _, shards := range []int{1, 2, 3, 4, 8, 17} {
		size := partition.Size(total, shards)
		want := partition.Over(total, size)
		seen := make(map[int]partition.Chunk, len(want))

		for shard := range shards {
			for _, c := range partition.ForShard(total, size, shard, shards) {
				if prev, dup := seen[c.Index]; dup {
					t.Fatalf("shards=%d: chunk %d assigned twice (%v and %v)", shards, c.Index, prev, c)
				}

				seen[c.Index] = c
			}
		}

		if len(seen) != len(want) {
			t.Fatalf("shards=%d: %d chunks assigned, want %d", shards, len(seen), len(want))
		}

		for _, w := range want {
			if got := seen[w.Index]; got != w {
				t.Fatalf("shards=%d: chunk %d came back as %v, want %v", shards, w.Index, got, w)
			}
		}
	}
}

// A shard index outside the pool is a caller bug, not a silent full walk:
// answering with every chunk would make a mis-wired pool compute the run N
// times over and still look correct.
func TestForShardRefusesAnIndexOutsideThePool(t *testing.T) {
	t.Parallel()

	for _, shard := range []int{-1, 4, 99} {
		if got := partition.ForShard(1000, 64, shard, 4); got != nil {
			t.Fatalf("ForShard(shard=%d, shards=4) returned %d chunks, want none", shard, len(got))
		}
	}
}
