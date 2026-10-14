package wasmkernel

import (
	"context"
	"fmt"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// EstimateBudget is the probe budget browser mode asks for when the caller
// names none. Shorter than the CLI's three seconds because the probe occupies
// a Worker the run is about to want, and the dialog asks again whenever a
// setting changes.
//
// The Worker is busy for longer than this: preparing the scene comes first,
// and runestimate.Run lets three things run past the budget — counting every
// receiver's units, the first probe receiver's setup (its mirror table, a few
// tenths of a second on a city extract) and the unit in flight.
const EstimateBudget = 1500 * time.Millisecond

// EstimateRLS19Road says what ComputeRLS19RoadShard over workers shards will
// cost, from the same probe `aconiq run --estimate` runs.
//
// It is road.Estimate with the browser's schedule: chunks are dealt to shards
// up front (partition.ForShard) rather than pulled, so a shard that drew the
// expensive chunks is not helped by one that finished early, and the wall
// time is the slowest shard's.
//
// The arguments are a run's arguments — the whole receiver list, the config
// ApplyTerrain resolved — so the estimate prices the run the same request
// would start, and refuses what it would refuse in the same words.
func EstimateRLS19Road(
	receivers []geo.PointReceiver,
	sources []road.RoadSource,
	barriers []road.Barrier,
	cfg road.PropagationConfig,
	workers int,
	budget time.Duration,
) (runestimate.Estimate, error) {
	// The empty-receiver refusal is ComputeReceiverOutputs' sentence, as in
	// ComputeRLS19Road.
	if len(receivers) == 0 {
		_, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)

		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	if budget <= 0 {
		budget = EstimateBudget
	}

	estimate, err := road.Estimate(context.Background(), receivers, sources, barriers, cfg, runestimate.Options{
		Budget:   budget,
		Workers:  max(workers, 1),
		Schedule: runestimate.ScheduleSharded,
	})
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	return estimate, nil
}
