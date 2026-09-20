// Package partition decides how a receiver list is split for parallel compute.
//
// It exists as a package of its own, depending on nothing, for two reasons.
// Both compute targets have to agree on the split — the CLI's goroutine pool
// and the browser's Web Worker pool — and `js/wasm` has to be able to link it,
// which rules out anything reaching for `os` or a clock.
//
// # Why this is the file docs/policies/determinism.md rests on
//
// The policy requires "receiver/source chunking must be deterministic from
// immutable input ordering" and a "stable reduction tree ... independent of
// worker scheduling". Three different cadences in this system are easy to
// confuse, and only the third one is that chunking:
//
//   - The kernel's progress chunk size (wasmkernel.nextChunkSize) is adaptive
//     on elapsed time. It decides where the walk pauses to *report*.
//   - The worker's postMessage throttle is a wire-rate limiter, one message
//     per ~100 ms.
//   - The partition chunk size is this package, and it may never see a clock.
//
// Size is therefore a pure function of two integers. It takes no measured
// throughput, no worker identity and no wall time, because the moment it did,
// the output would depend on how fast the machine happened to be running.
package partition

// MinChunk is the smallest partition chunk.
//
// Below it the per-chunk bookkeeping — a slice header, a channel receive, a
// bounds check against the results array — outweighs the propagation work
// inside the chunk. It also keeps a small run single-chunked, so the 49-receiver
// digest fixtures take the same path they always have.
const MinChunk = 64

// TargetChunksPerWorker is how many chunks each worker is aimed at.
//
// More than one, because a receiver grid is not uniformly expensive: a receiver
// beside the road walks more segments, crosses more barriers and admits more
// Spiegelschallquellen than one at the far corner of the grid. One contiguous
// block per worker hands whichever worker drew the near rows several times the
// work of the others, and the run then waits for it. Several smaller chunks
// pulled dynamically level that out.
//
// It is not larger because every chunk boundary costs a progress report and a
// results-slice entry, and past this the levelling has already happened.
const TargetChunksPerWorker = 8

// Chunk is a half-open receiver range [Start, End) with the index that fixes
// its place in the merge.
//
// Index is the load-bearing field. Chunks are computed in whatever order
// workers happen to pick them up, and merged strictly by Index — never by
// completion order, which is what the policy means by "no 'first finished
// worker wins' accumulation".
type Chunk struct {
	Index int
	Start int
	End   int
}

// Len is how many receivers the chunk covers.
func (c Chunk) Len() int { return c.End - c.Start }

// Size is the partition chunk size for a run of total receivers over the given
// worker count.
//
// Pure in its two integer arguments. See the package comment for why that is a
// constraint rather than an implementation detail.
func Size(total, workers int) int {
	if total <= 0 {
		return MinChunk
	}

	if workers <= 1 {
		return max(total, 1)
	}

	// Round up, so the chunk count does not overshoot the target and leave a
	// ragged final chunk of one receiver.
	wanted := (total + workers*TargetChunksPerWorker - 1) / (workers * TargetChunksPerWorker)

	return min(max(wanted, MinChunk), total)
}

// Over returns the chunks covering [0, total) at chunkSize, ascending.
//
// The result is gapless, non-overlapping and in ascending Start order, and
// depends on nothing but the two arguments. A total of zero yields no chunks,
// which is the empty walk rather than an error.
func Over(total, chunkSize int) []Chunk {
	if total <= 0 {
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = total
	}

	chunks := make([]Chunk, 0, (total+chunkSize-1)/chunkSize)

	for start := 0; start < total; start += chunkSize {
		chunks = append(chunks, Chunk{
			Index: len(chunks),
			Start: start,
			End:   min(start+chunkSize, total),
		})
	}

	return chunks
}

// ForShard returns the chunks a static block-cyclic assignment gives one shard:
// chunk c belongs to shard c%shards.
//
// Assignment is not partitioning. It decides *who* computes a chunk, never what
// the chunk is or where its results land — the merge is by Chunk.Index, so two
// runs with different shard counts produce the same chunks in the same order
// and therefore the same output, bit for bit.
//
// Block-cyclic rather than contiguous for the reason TargetChunksPerWorker
// exists: consecutive chunks are the expensive ones together, so dealing them
// round-robin spreads the near-the-road rows across every shard.
//
// It is used by the browser, where a shard cannot pull work from a shared queue
// because each Worker is a separate address space. The CLI pool pulls
// dynamically instead, which balances better and is free there.
func ForShard(total, chunkSize, shard, shards int) []Chunk {
	if shards <= 1 {
		return Over(total, chunkSize)
	}

	if shard < 0 || shard >= shards {
		return nil
	}

	all := Over(total, chunkSize)
	mine := make([]Chunk, 0, (len(all)-shard+shards-1)/shards)

	for _, c := range all {
		if c.Index%shards == shard {
			mine = append(mine, c)
		}
	}

	return mine
}

// Receivers is how many receivers a shard is assigned, which is the total its
// progress reports count against.
func Receivers(chunks []Chunk) int {
	n := 0
	for _, c := range chunks {
		n += c.Len()
	}

	return n
}
