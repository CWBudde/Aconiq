package rail

import (
	"context"
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput = acoustics.ReceiverOutput

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsContext(context.Background(), receivers, sources, cfg)
}

// ComputeReceiverOutputsContext is ComputeReceiverOutputs under a context: once
// ctx is done it stops before the next receiver and returns ctx.Err().
func ComputeReceiverOutputsContext(ctx context.Context, receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := acoustics.ComputeReceiverOutputs(ctx, receivers, sources,
		func(receiver geo.PointReceiver, sources []RailSource) (PeriodLevels, error) {
			return ComputeReceiverPeriodLevels(receiver.Point, sources, cfg)
		})
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}
