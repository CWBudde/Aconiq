package industry

import (
	"context"
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput = acoustics.ReceiverOutput

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsContext(context.Background(), receivers, sources, cfg)
}

// ComputeReceiverOutputsContext is ComputeReceiverOutputs under a context: once
// ctx is done it stops before the next receiver and returns ctx.Err().
func ComputeReceiverOutputsContext(ctx context.Context, receivers []geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsProgress(ctx, receivers, sources, cfg, nil)
}

// ComputeReceiverOutputsProgress is ComputeReceiverOutputsContext that also
// reports (receivers done, receivers in total) after each receiver, through
// acoustics.ComputeReceiverOutputsProgress: never before the first receiver
// has completed, done increasing by one, and exactly one call with
// done == total on a walk that succeeds. A failed or cancelled walk stops
// reporting at the last receiver it completed. A nil progress reports nothing,
// and the report is read by nothing in the walk, so it cannot move a level.
func ComputeReceiverOutputsProgress(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []IndustrySource,
	cfg PropagationConfig,
	progress func(done, total int),
) ([]ReceiverOutput, error) {
	outputs, err := acoustics.ComputeReceiverOutputsProgress(ctx, receivers, sources,
		func(receiver geo.PointReceiver, sources []IndustrySource) (PeriodLevels, error) {
			return ComputeReceiverPeriodLevels(receiver, sources, cfg)
		}, progress)
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}
