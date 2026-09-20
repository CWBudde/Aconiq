package wasmkernel

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/partition"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// Shard names one member of a browser worker pool.
//
// Index is zero-based and Count is the pool size, so Shard{0, 1} is the whole
// run and is what ComputeRLS19Road is.
type Shard struct {
	Index int `json:"index"`
	Count int `json:"count"`
}

// ChunkOutput is one partition chunk's results, with the index that fixes its
// place in the merge.
//
// Chunk, not the order the shards' replies happened to arrive in. Four
// Workers finish in whatever order the machine gives them, and
// docs/policies/determinism.md requires partial results to be merged "in a
// fixed order independent of worker scheduling" — so the index travels with
// the payload and the main thread sorts by it. Start is carried too, and the
// client checks it: a chunk that does not begin where the previous one ended
// means the merge is about to produce a short receiver table, which renders
// as a perfectly plausible raster of a smaller model.
type ChunkOutput struct {
	Chunk   int                   `json:"chunk"`
	Start   int                   `json:"start"`
	Outputs []road.ReceiverOutput `json:"outputs"`
}

// ComputeRLS19RoadShard computes the partition chunks assigned to one shard.
//
// # Every argument but shard is the whole run's
//
// receivers is the entire receiver list, not this shard's slice of it, and
// that is the invariant this function exists to enforce rather than an
// inefficiency to be tidied away later.
//
// road.PropagationConfig carries ReceiverTerrainZ, a single elevation for the
// whole grid, and TerrainAtGridCenter derives it from the *centroid of the
// receiver list it is handed*. Give each Worker only its own slice and each
// computes a different centroid, samples the DTM at a different point, and
// produces different levels — across its whole shard, silently, with no
// refusal and nothing in the output to say so. Nothing in the app calls
// loadTerrain today, so that bug would ship green and surface the first time
// a DTM is wired up.
//
// Handing every shard the identical list makes it unwritable. Every
// receiver-set-derived quantity is then derived from the same input in every
// Worker, and the equality of the shards' configs stops being a floating-point
// argument and becomes a structural one.
//
// The cost is one JSON parse of the receiver list per Worker. That is real —
// and it is what a binary receiver descriptor removes, once the grid case
// stops sending receivers one JSON object at a time.
//
// # Progress
//
// progress reports this shard's own receivers, not the run's: a shard cannot
// see what the others have finished. The client sums them, which is monotone
// because each shard's count is, and lands exactly on the run's total because
// the shards partition the receiver set.
func ComputeRLS19RoadShard(
	receivers []geo.PointReceiver,
	sources []road.RoadSource,
	barriers []road.Barrier,
	cfg road.PropagationConfig,
	shard Shard,
	chunkSize int,
	progress func(done, total int),
) ([]ChunkOutput, error) {
	if shard.Count < 1 {
		return nil, fmt.Errorf("shard count must be at least 1, got %d", shard.Count)
	}

	if shard.Index < 0 || shard.Index >= shard.Count {
		return nil, fmt.Errorf("shard index %d is outside a pool of %d", shard.Index, shard.Count)
	}

	total := len(receivers)

	// Delegated rather than restated, as in ComputeRLS19Road: the
	// empty-receiver refusal is ComputeReceiverOutputs' sentence to word.
	if total == 0 {
		_, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)

		return nil, fmt.Errorf("%w", err)
	}

	// Every shard prepares the scene, and every shard therefore refuses a bad
	// model in the same words at the same moment. A pool where only shard 0
	// validated would answer differently depending on which Worker a defect
	// happened to land in.
	scene, err := road.PrepareSceneFor(receivers, sources, barriers, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	chunks := partition.ForShard(
		total,
		partition.Size(total, shard.Count),
		shard.Index,
		shard.Count,
	)

	walk := newProgressWalk(chunkSize, partition.Receivers(chunks), progress)
	outputs := make([]ChunkOutput, 0, len(chunks))

	for _, chunk := range chunks {
		computed, err := walk.over(
			scene,
			receivers[chunk.Start:chunk.End],
			cfg,
			make([]road.ReceiverOutput, 0, chunk.Len()),
		)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, ChunkOutput{
			Chunk:   chunk.Index,
			Start:   chunk.Start,
			Outputs: computed,
		})
	}

	return outputs, nil
}
