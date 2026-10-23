package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/results"
)

// CustomReceiversRequest is the JSON `aconiq.customReceivers` takes.
//
// Model is a GeoJSON FeatureCollection in the v1 model schema, in CRS, the
// run's compute CRS, holding the buildings and the receivers with their
// properties: the facade links are validated against the buildings there, as
// `aconiq run` validates them. Receivers is the list the browser computes at,
// in model order, already projected; the kernel decides which of them a run
// keeps. AllowUnconfirmed is `--allow-unconfirmed`.
type CustomReceiversRequest struct {
	CRS              string              `json:"crs"`
	Model            json.RawMessage     `json:"model"`
	StandardID       string              `json:"standard_id"`
	Receivers        []geo.PointReceiver `json:"receivers"`
	AllowUnconfirmed bool                `json:"allow_unconfirmed,omitempty"`
}

// CustomReceiver is one kept receiver, with the facade attributes its receiver
// table row carries, in results.ReceiverRecord's field names and shape. They
// are present only as the complete set a row accepts (Attributes.Complete).
type CustomReceiver struct {
	geo.PointReceiver

	PlacementRule string                  `json:"placement_rule,omitempty"`
	Facade        *results.ReceiverFacade `json:"facade,omitempty"`
	Storey        *int                    `json:"storey,omitempty"`
}

// CustomReceiversResponse is the kept set in the order a run computes it, the
// counts a run summary records, and the metadata provenance records.
type CustomReceiversResponse struct {
	Receivers          []CustomReceiver  `json:"receivers"`
	ProposedCount      int               `json:"proposed_count"`
	AssumedHeightCount int               `json:"assumed_height_count"`
	RejectedCount      int               `json:"rejected_count"`
	Metadata           map[string]string `json:"metadata"`
}

// CustomReceivers settles the receiver set of a browser-mode custom run
// through facadeset.Custom and facadeset.Gate, the calls `aconiq run
// --receiver-mode custom` makes: a rejected point is dropped, and a normative
// run over unconfirmed points, a broken facade link or another standard's rule
// is refused in the run's words. The browser must not decide this itself: the
// facade-link check is model validation, and a second one would disagree.
func CustomReceivers(input []byte) ([]byte, error) {
	var req CustomReceiversRequest

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.CRS == "" {
		return nil, errors.New("crs is required: the compute CRS the model's coordinates are in")
	}

	if len(req.Receivers) == 0 {
		return nil, errors.New("custom receiver mode requires at least one explicit receiver in the model")
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "customReceivers")
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}

	listed, rejected, err := facadeset.Custom(req.Receivers, model)
	if err != nil {
		return nil, fmt.Errorf("custom receivers: %w", err)
	}

	standard, err := resolveStandard(req.StandardID)
	if err != nil {
		return nil, err
	}

	err = facadeset.Gate(listed, standard, model, req.AllowUnconfirmed)
	if err != nil {
		return nil, fmt.Errorf("facade receivers: %w", err)
	}

	resp := CustomReceiversResponse{
		Receivers:     make([]CustomReceiver, 0, len(listed.Receivers)),
		RejectedCount: rejected,
		Metadata:      map[string]string{},
	}

	resp.ProposedCount, resp.AssumedHeightCount = listed.CountUnconfirmed()
	if req.AllowUnconfirmed {
		resp.Metadata["allow_unconfirmed"] = "true"
	}

	for _, receiver := range listed.Receivers {
		kept := CustomReceiver{PointReceiver: receiver}

		if attrs, ok := listed.Facade[receiver.ID]; ok && attrs.Complete() {
			key := attrs.Facade
			storey := *attrs.Storey
			kept.PlacementRule = attrs.PlacementRule
			kept.Storey = &storey
			kept.Facade = &results.ReceiverFacade{
				BuildingRef: key.BuildingRef,
				Part:        key.Part,
				Ring:        key.Ring,
				Edge:        key.Edge,
				From:        [2]float64{key.From.X, key.From.Y},
				To:          [2]float64{key.To.X, key.To.Y},
			}
		}

		resp.Receivers = append(resp.Receivers, kept)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}
