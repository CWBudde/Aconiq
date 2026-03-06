package road

import (
	"fmt"

	"github.com/soundplan/soundplan/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RoadSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, fmt.Errorf("at least one receiver is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		if receiver.ID == "" {
			return nil, fmt.Errorf("receiver id is required")
		}
		if !receiver.Point.IsFinite() {
			return nil, fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
		}
		periodLevels, err := ComputeReceiverPeriodLevels(receiver.Point, sources, cfg)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: periodLevels.ToReceiverIndicators(),
		})
	}

	return outputs, nil
}
