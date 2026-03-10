package road

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
// This is the top-level entry point for RLS-19 road calculations.
// Barriers are optional; pass nil for free-field calculation.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RoadSource, barriers []Barrier, cfg PropagationConfig) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		if receiver.ID == "" {
			return nil, errors.New("receiver id is required")
		}

		if !receiver.Point.IsFinite() {
			return nil, fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
		}

		receiverCfg := cfg
		receiverCfg.ReceiverHeightM = receiver.HeightM

		periodLevels, err := ComputeReceiverLevels(receiver.Point, sources, barriers, receiverCfg)
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
