package industry

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput = acoustics.ReceiverOutput

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := acoustics.ComputeReceiverOutputs(receivers, sources,
		func(receiver geo.PointReceiver, sources []IndustrySource) (PeriodLevels, error) {
			return ComputeReceiverPeriodLevels(receiver, sources, cfg)
		})
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}
