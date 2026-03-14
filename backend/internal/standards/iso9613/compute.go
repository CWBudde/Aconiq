package iso9613

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	LpAeq float64
}

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeReceiverLevel computes the combined receiver level for one receiver.
func ComputeReceiverLevel(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) (float64, error) {
	err := cfg.Validate()
	if err != nil {
		return 0, err
	}

	if receiver.ID == "" {
		return 0, errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
		return 0, errors.New("receiver point is not finite")
	}

	if len(sources) == 0 {
		return 0, errors.New("at least one source is required")
	}

	contributions := make([]float64, 0, len(sources))

	for _, source := range sources {
		err := source.Validate()
		if err != nil {
			return 0, err
		}

		emission, err := ComputeEmission(source)
		if err != nil {
			return 0, err
		}

		terms := attenuation(receiver, source, cfg)
		contributions = append(contributions, emission-totalAttenuation(terms))
	}

	return energySumDB(contributions), nil
}

// ComputeReceiverOutputs computes ISO 9613-2 preview outputs for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []PointSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		level, err := ComputeReceiverLevel(receiver, sources, cfg)
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: ReceiverIndicators{LpAeq: level},
		})
	}

	return outputs, nil
}
