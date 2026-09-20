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

// A run too small to be worth splitting must come out as one chunk, so the
// small fixtures keep taking the single-threaded path they always have.
func TestSizeKeepsSmallRunsWhole(t *testing.T) {
	t.Parallel()

	for _, total := range []int{1, 7, 49, partition.MinChunk} {
		size := partition.Size(total, 8)

		if chunks := partition.Over(total, size); len(chunks) != 1 {
			t.Fatalf("a %d-receiver run split into %d chunks, want 1", total, len(chunks))
		}
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
