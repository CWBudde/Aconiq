package wasmkernel_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// Pool sizes covering the shapes that go wrong differently: 1 is the whole
// run, 2 and 3 split 23 receivers unevenly, 4 is the browser's cap, and 24 is
// more shards than there are receivers, so some get nothing at all.
var shardCounts = []int{1, 2, 3, 4, 24}

// mergeShards gathers every shard's chunks and puts them back in receiver
// order, the way the browser client does.
func mergeShards(t *testing.T, chunks []wasmkernel.ChunkOutput) []road.ReceiverOutput {
	t.Helper()

	ordered := slices.Clone(chunks)
	slices.SortFunc(ordered, func(a, b wasmkernel.ChunkOutput) int { return a.Chunk - b.Chunk })

	merged := make([]road.ReceiverOutput, 0)

	for position, chunk := range ordered {
		if chunk.Chunk != position {
			t.Fatalf("chunk %d is missing or duplicated (got index %d)", position, chunk.Chunk)
		}

		if chunk.Start != len(merged) {
			t.Fatalf("chunk %d starts at %d but %d receivers precede it",
				chunk.Chunk, chunk.Start, len(merged))
		}

		merged = append(merged, chunk.Outputs...)
	}

	return merged
}

func gatherShards(
	t *testing.T,
	receivers []geo.PointReceiver,
	cfg road.PropagationConfig,
	count int,
) []wasmkernel.ChunkOutput {
	t.Helper()

	all := make([]wasmkernel.ChunkOutput, 0)

	for index := range count {
		chunks, err := wasmkernel.ComputeRLS19RoadShard(
			receivers, sampleSources(), sampleBarriers(), cfg,
			wasmkernel.Shard{Index: index, Count: count}, 0, nil,
		)
		if err != nil {
			t.Fatalf("shard %d/%d: %v", index, count, err)
		}

		all = append(all, chunks...)
	}

	return all
}

// The claim the browser pool rests on: however many Workers split the run,
// reassembling their chunks gives the walk a single Worker would have made.
// Full struct equality, because docs/policies/determinism.md has no tolerance
// in it.
func TestShardsReproduceTheWholeWalk(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()
	cfg := road.DefaultPropagationConfig()

	want, err := road.ComputeReceiverOutputs(receivers, sampleSources(), sampleBarriers(), cfg)
	if err != nil {
		t.Fatalf("unsharded walk: %v", err)
	}

	for _, count := range shardCounts {
		t.Run(fmt.Sprintf("shards=%d", count), func(t *testing.T) {
			t.Parallel()

			got := mergeShards(t, gatherShards(t, receivers, cfg, count))

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("shards=%d moved a result", count)
			}
		})
	}
}

// Every chunk must be computed by exactly one shard. A chunk computed twice
// is wasted work; a chunk computed by nobody is a short receiver table, which
// renders as a plausible raster of a smaller model.
func TestShardChunksPartitionTheReceivers(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()
	cfg := road.DefaultPropagationConfig()

	for _, count := range shardCounts {
		seen := map[int]int{}
		covered := 0

		for _, chunk := range gatherShards(t, receivers, cfg, count) {
			seen[chunk.Chunk]++
			covered += len(chunk.Outputs)
		}

		for index, times := range seen {
			if times != 1 {
				t.Fatalf("shards=%d: chunk %d was computed %d times", count, index, times)
			}
		}

		if covered != len(receivers) {
			t.Fatalf("shards=%d: %d receivers computed, want %d", count, covered, len(receivers))
		}
	}
}

// A shard reports its own receivers, and reaches all of them.
//
// The client sums these into one bar, so a shard that stopped short or
// overshot would make the bar stall or run past its end.
func TestShardProgressReachesItsOwnTotal(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()
	cfg := road.DefaultPropagationConfig()

	const count = 4

	summed := 0

	for index := range count {
		var reports [][2]int

		_, err := wasmkernel.ComputeRLS19RoadShard(
			receivers, sampleSources(), sampleBarriers(), cfg,
			wasmkernel.Shard{Index: index, Count: count}, 1,
			func(done, total int) { reports = append(reports, [2]int{done, total}) },
		)
		if err != nil {
			t.Fatalf("shard %d: %v", index, err)
		}

		if len(reports) == 0 {
			continue
		}

		for i, r := range reports {
			if i > 0 && r[0] <= reports[i-1][0] {
				t.Fatalf("shard %d report %d went backwards: %v", index, i, reports)
			}
		}

		last := reports[len(reports)-1]
		if last[0] != last[1] {
			t.Fatalf("shard %d finished at %d of %d", index, last[0], last[1])
		}

		summed += last[1]
	}

	// The shards partition the receivers, so their totals add up to the run's
	// — which is why the client can sum the dones and land exactly on it.
	if summed != len(receivers) {
		t.Fatalf("shard totals summed to %d, want %d", summed, len(receivers))
	}
}

// The trap this whole design exists to rule out.
//
// road.PropagationConfig carries one ReceiverTerrainZ for the whole grid, and
// TerrainAtGridCenter derives it from the centroid of the receiver list it is
// given. A pool that handed each Worker only its own slice would give each a
// different centroid and therefore a different ground elevation, and the
// levels would differ across the whole shard with nothing in the output to
// say so.
//
// Sharding by window rather than by slice is what prevents it, and this is
// the test that fails if anyone "optimises" the whole receiver list out of
// the request.
func TestShardsAgreeOnTheGridCentreElevation(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()

	// A stand-in DTM with a strong gradient, so a centroid that moved even a
	// little reads a different elevation. A flat model would pass this test
	// whether or not the bug were present.
	cfg := road.DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = wasmkernel.TerrainAtGridCenter(slopeModel{}, receivers)

	if cfg.ReceiverTerrainZ == 0 {
		t.Fatal("the test terrain must not read as sea level, or this proves nothing")
	}

	want, err := road.ComputeReceiverOutputs(receivers, sampleSources(), sampleBarriers(), cfg)
	if err != nil {
		t.Fatalf("unsharded walk: %v", err)
	}

	for _, count := range shardCounts {
		got := mergeShards(t, gatherShards(t, receivers, cfg, count))

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("shards=%d disagreed with the whole walk over sloping ground", count)
		}

		// And the elevation a shard would derive from its own slice really is
		// different — otherwise the guard above is vacuous.
		if count > 1 {
			half := wasmkernel.TerrainAtGridCenter(slopeModel{}, receivers[:len(receivers)/count])
			if half == cfg.ReceiverTerrainZ {
				t.Fatalf("shards=%d: the slice centroid matches the whole grid's, so this test cannot fail", count)
			}
		}
	}
}

// slopeModel is a DTM that rises steadily eastwards, so any movement of the
// sampled point changes the answer. A flat stand-in would let this test pass
// whether or not the bug it guards were present.
type slopeModel struct{}

func (slopeModel) ElevationAt(x, _ float64) (float64, bool) { return 100 + x/10, true }

func (slopeModel) Bounds() [4]float64 { return slopeBounds }

func (slopeModel) Info() terrain.Info {
	return terrain.Info{Bounds: slopeBounds, PixelSize: [2]float64{1, 1}, GridSize: [2]int{2, 2}}
}

var slopeBounds = [4]float64{-1e6, -1e6, 1e6, 1e6}

// A shard index outside its pool is a caller bug and must be refused: a pool
// that silently answered with every chunk would compute the run N times and
// still look right.
func TestComputeRLS19RoadShardRefusesAnIndexOutsideThePool(t *testing.T) {
	t.Parallel()

	cfg := road.DefaultPropagationConfig()

	for _, shard := range []wasmkernel.Shard{
		{Index: -1, Count: 4}, {Index: 4, Count: 4}, {Index: 0, Count: 0}, {Index: 0, Count: -2},
	} {
		_, err := wasmkernel.ComputeRLS19RoadShard(
			sampleReceivers(), sampleSources(), sampleBarriers(), cfg, shard, 0, nil,
		)
		if err == nil {
			t.Fatalf("shard %+v was accepted", shard)
		}
	}
}
