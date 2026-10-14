package iso9613

import (
	"context"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// errNoReceivers is how a run, and its estimate, refuse an empty receiver set.
var errNoReceivers = errors.New("at least one receiver is required")

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	LpAeqDW float64
	LpAeqLT float64
}

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeReceiverIndicators computes both DW and LT indicators for one receiver.
func ComputeReceiverIndicators(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) (ReceiverIndicators, error) {
	return computeReceiverIndicators(context.Background(), receiver, sources, cfg)
}

// computeReceiverIndicators is ComputeReceiverIndicators under a context,
// checked before every source→receiver path (computeDownwindAndLongTermLevels).
func computeReceiverIndicators(ctx context.Context, receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) (ReceiverIndicators, error) {
	err := checkReceiverInputs(receiver, sources, cfg)
	if err != nil {
		return ReceiverIndicators{}, err
	}

	// C_met is applied per source-receiver path inside the summation
	// (Eq. 6 with clause 8), not once to the summed level.
	dwLevel, ltLevel, err := computeDownwindAndLongTermLevels(ctx, receiver, sources, cfg)
	if err != nil {
		return ReceiverIndicators{}, err
	}

	return ReceiverIndicators{LpAeqDW: dwLevel, LpAeqLT: ltLevel}, nil
}

// checkReceiverInputs is what the walk checks before every receiver's first
// path: the config, the receiver, and every source. The config and the
// sources are the same for every receiver of a run, but the walk checks them
// again each time, so on a scene of many sources this is a per-receiver cost
// the run-time estimate has to time (see estimateWalker.Begin). The order of
// the checks decides which error a doubly broken input reports.
func checkReceiverInputs(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) error {
	err := cfg.Validate()
	if err != nil {
		return err
	}

	err = checkReceiver(receiver)
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		return errors.New("at least one source is required")
	}

	for _, source := range sources {
		err := source.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// checkReceiver is checkReceiverInputs' check of the receiver itself.
func checkReceiver(receiver geo.PointReceiver) error {
	if receiver.ID == "" {
		return errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
		return errors.New("receiver point is not finite")
	}

	return nil
}

// ComputeReceiverLevel computes the combined downwind receiver level for one receiver.
// Retained for backward compatibility; prefer ComputeReceiverIndicators.
func ComputeReceiverLevel(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) (float64, error) {
	indicators, err := ComputeReceiverIndicators(receiver, sources, cfg)
	if err != nil {
		return 0, err
	}

	return indicators.LpAeqDW, nil
}

// ComputeReceiverOutputs computes ISO 9613-2 preview outputs for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []PointSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsContext(context.Background(), receivers, sources, cfg)
}

// ComputeReceiverOutputsContext is ComputeReceiverOutputs under a context: once
// ctx is done it stops before the next source→receiver path, inside the
// receiver it is computing, and returns ctx.Err().
func ComputeReceiverOutputsContext(ctx context.Context, receivers []geo.PointReceiver, sources []PointSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsProgress(ctx, receivers, sources, cfg, nil)
}

// ComputeReceiverOutputsProgress is ComputeReceiverOutputsContext that also
// reports (receivers done, receivers in total) after each receiver, the shape
// road.ProgressFunc documents. A nil progress reports nothing. The report is
// read by nothing in the walk, so it cannot move a level.
func ComputeReceiverOutputsProgress(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []PointSource,
	cfg PropagationConfig,
	progress func(done, total int),
) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errNoReceivers
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		indicators, err := computeReceiverIndicators(ctx, receiver, sources, cfg)
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: indicators,
		})

		if progress != nil {
			progress(len(outputs), len(receivers))
		}
	}

	return outputs, nil
}
