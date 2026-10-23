package facadeset

import (
	"fmt"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// Complete reports whether the attributes are the set a receiver table row
// takes: a placement rule, a facade key and a storey. That is the one shape
// results.ReceiverTable accepts; a partial set stays in the model.
func (a Attributes) Complete() bool {
	return a.PlacementRule != "" && a.Facade != nil && a.Storey != nil
}

// Custom is the receiver set of a custom run: the model's receivers, in the
// order given, with the facade attributes of those that carry any. A rejected
// point never enters a run, so it is dropped and counted, and a model whose
// every receiver is rejected leaves nothing to compute at.
//
// `aconiq run --receiver-mode custom` and the WASM kernel's `customReceivers`
// both come through here, before Gate, so a browser run over hand-placed
// facade points drops and refuses what the CLI does.
func Custom(receivers []geo.PointReceiver, model modelgeojson.Model) (Listed, int, error) {
	byID := make(map[string]modelgeojson.Feature, len(receivers))

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindReceiver {
			byID[feature.ID] = feature
		}
	}

	listed := Listed{
		Receivers: make([]geo.PointReceiver, 0, len(receivers)),
		Facade:    make(map[string]Attributes),
	}
	rejected := 0

	for _, receiver := range receivers {
		attrs, carries, err := AttributesOf(byID[receiver.ID])
		if err != nil {
			return Listed{}, 0, domainerrors.New(domainerrors.KindValidation, op,
				fmt.Sprintf("receiver %q: %v", receiver.ID, err), nil)
		}

		if carries && attrs.ReviewStatus == geo.ReviewStatusRejected {
			rejected++

			continue
		}

		if carries {
			listed.Facade[receiver.ID] = attrs
		}

		listed.Receivers = append(listed.Receivers, receiver)
	}

	if len(listed.Receivers) == 0 {
		return Listed{}, 0, domainerrors.New(domainerrors.KindUserInput, op,
			"custom receiver mode found no receiver to compute at: every receiver in the model is rejected", nil)
	}

	return listed, rejected, nil
}
