package iso9613

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

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
	err := cfg.Validate()
	if err != nil {
		return ReceiverIndicators{}, err
	}

	if receiver.ID == "" {
		return ReceiverIndicators{}, errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
		return ReceiverIndicators{}, errors.New("receiver point is not finite")
	}

	if len(sources) == 0 {
		return ReceiverIndicators{}, errors.New("at least one source is required")
	}

	for _, source := range sources {
		err := source.Validate()
		if err != nil {
			return ReceiverIndicators{}, err
		}
	}

	dwLevel := ComputeDownwindLevel(receiver, sources, cfg)

	ltLevel := dwLevel
	if cfg.C0 > 0 && len(sources) > 0 {
		dp := 0.0
		for _, source := range sources {
			d := geo.Distance(receiver.Point, source.Point)
			if d > dp {
				dp = d
			}
		}

		hs := sources[0].SourceHeightM
		hr := receiver.HeightM
		cmet := MeteorologicalCorrection(cfg.C0, hs, hr, dp)
		ltLevel = dwLevel - cmet
	}

	return ReceiverIndicators{LpAeqDW: dwLevel, LpAeqLT: ltLevel}, nil
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
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		indicators, err := ComputeReceiverIndicators(receiver, sources, cfg)
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: indicators,
		})
	}

	return outputs, nil
}
