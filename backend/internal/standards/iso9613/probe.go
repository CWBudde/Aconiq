package iso9613

import (
	"context"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
)

// Estimate says what computing these receivers will cost, by timing a sample
// of the walk ComputeReceiverOutputsProgress would make.
//
// It refuses what the run would refuse, in the run's words: no receivers at
// all, or a receiver whose checks fail — the config, the sources and the
// first receiver are checked by checkReceiverInputs, the call the walk makes
// before every receiver's first path, and every later receiver by
// checkReceiver, the only part of those checks that differs between
// receivers. The run would stop at the first receiver that fails; the
// estimate stops there too, with the same wrapped error.
//
// It computes no level anyone reads. The probe runs the walk's own per-path
// step, pathSums.addPath, into a sum it then drops, so nothing about a run
// can differ for having been estimated first.
//
// The walk is sequential — one receiver after another on one goroutine — so
// the estimate is made for one worker whatever opts.Workers says, and there is
// no scene preparation to report: everything the run checks or derives, it
// does per receiver or per path, and the probe times it there.
func Estimate(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []PointSource,
	cfg PropagationConfig,
	opts runestimate.Options,
) (runestimate.Estimate, error) {
	err := checkRunInputs(receivers, sources, cfg)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	opts.Workers = 1
	opts.Schedule = runestimate.SchedulePull
	opts.Prepare = 0

	points := make([]geo.Point2D, len(receivers))
	for i, r := range receivers {
		points[i] = r.Point
	}

	estimate, err := runestimate.Run(ctx, newEstimateKernel(receivers, sources, cfg), points, opts)
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	estimate.StandardID = StandardID
	estimate.Inputs.Counts = map[string]int{
		"point_sources": len(sources),
		"barriers":      len(cfg.Barriers),
		"ground_zones":  len(cfg.GroundZones),
	}

	return estimate, nil
}

// checkRunInputs refuses what ComputeReceiverOutputsProgress would refuse
// before computing a level, in its words, without repeating the per-receiver
// checks of the config and the sources for every receiver: they pass or fail
// alike for all of them, so checking them with the first one decides it.
func checkRunInputs(receivers []geo.PointReceiver, sources []PointSource, cfg PropagationConfig) error {
	if len(receivers) == 0 {
		return errNoReceivers
	}

	err := checkReceiverInputs(receivers[0], sources, cfg)
	if err != nil {
		return fmt.Errorf("receiver %q: %w", receivers[0].ID, err)
	}

	for _, receiver := range receivers[1:] {
		err := checkReceiver(receiver)
		if err != nil {
			return fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}
	}

	return nil
}

// estimateKernel is runestimate.Kernel over the run's inputs.
//
// A unit is one source→receiver path, in the order
// computeDownwindAndLongTermLevels walks them: unit u of every receiver is
// sources[u].
type estimateKernel struct {
	receivers []geo.PointReceiver
	sources   []PointSource
	cfg       PropagationConfig
}

func newEstimateKernel(receivers []geo.PointReceiver, sources []PointSource, cfg PropagationConfig) *estimateKernel {
	return &estimateKernel{receivers: receivers, sources: sources, cfg: cfg}
}

// Units is how many paths the walk visits for a receiver: one per source,
// for every receiver alike.
func (k *estimateKernel) Units(int) int {
	return len(k.sources)
}

func (k *estimateKernel) NewWalker() runestimate.Walker {
	return k.newWalker()
}

func (k *estimateKernel) newWalker() *estimateWalker {
	return &estimateWalker{kernel: k}
}

// estimateWalker runs single paths of one receiver at a time.
type estimateWalker struct {
	kernel   *estimateKernel
	receiver geo.PointReceiver

	// checked is what the receiver's checks returned. Estimate has already
	// refused a run they could fail on; it is kept only so the checks are
	// work with a result, as they are in the walk.
	checked error

	sums pathSums
}

// Begin does a receiver's one-off work: the checks the walk makes before the
// receiver's first path, which re-validate the config and every source and
// so grow with the scene.
func (w *estimateWalker) Begin(receiver int) {
	w.receiver = w.kernel.receivers[receiver]
	w.checked = checkReceiverInputs(w.receiver, w.kernel.sources, w.kernel.cfg)
}

// Unit runs one path of the receiver last begun and drops what it
// contributed.
func (w *estimateWalker) Unit(unit int) {
	w.sums = pathSums{}

	w.addUnit(unit)
}

// addUnit adds one path's contribution to the walker's sums, by the step the
// walk adds it with.
func (w *estimateWalker) addUnit(unit int) {
	w.sums.addPath(w.receiver, &w.kernel.sources[unit], w.kernel.cfg)
}
