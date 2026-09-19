package rail

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput = acoustics.ReceiverOutput

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := acoustics.ComputeReceiverOutputs(receivers, sources,
		func(receiver geo.PointReceiver, sources []RailSource) (PeriodLevels, error) {
			return ComputeReceiverPeriodLevels(receiver.Point, sources, cfg)
		})
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}
