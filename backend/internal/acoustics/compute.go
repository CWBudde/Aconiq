package acoustics

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// PeriodLevelsFunc computes the day/evening/night levels one receiver sees from
// a module's own source type. It is what ComputeReceiverOutputs delegates the
// acoustics to: everything the modules share is the walk around it.
type PeriodLevelsFunc[S any] func(receiver geo.PointReceiver, sources []S) (PeriodLevels, error)

// ComputeReceiverOutputs walks receivers in the order given, validates each one
// and pairs it with the END indicators periodLevels computes for it.
//
// The order of the result is the order of receivers, and nothing about the walk
// depends on the source type, so every module reporting the END set can share
// it: the generic parameter exists only so that each module keeps passing its
// own typed slice rather than an []any.
//
// A receiver without an ID or with non-finite coordinates fails the whole
// batch. Reporting a level for a receiver that cannot be identified, or that
// sits nowhere, would put a number into a receiver table that no consumer can
// trace back to a place.
func ComputeReceiverOutputs[S any](
	receivers []geo.PointReceiver,
	sources []S,
	periodLevels PeriodLevelsFunc[S],
) ([]ReceiverOutput, error) {
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

		levels, err := periodLevels(receiver, sources)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: levels.ToReceiverIndicators(),
		})
	}

	return outputs, nil
}
