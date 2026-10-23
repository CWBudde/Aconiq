package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/framework"
)

// FacadeReceiversRequest is the JSON `aconiq.facadeReceivers` takes.
//
// Model is a GeoJSON FeatureCollection in the v1 model schema, already in CRS,
// the run's compute CRS: the browser projects its model through `transform`
// before anything reads a coordinate, as `aconiq run` does, so facade keys are
// canonical in the CRS they are validated in (facade-receivers.md, The facade
// key). It must hold every building, not only the requested ones: each masks
// the others' party walls. Buildings, SpacingM and IncludeCourtyard are
// `--facade-building`, `--facade-spacing-m` and `--facade-courtyard`, and
// AllowUnconfirmed is `--allow-unconfirmed`.
type FacadeReceiversRequest struct {
	CRS              string          `json:"crs"`
	Model            json.RawMessage `json:"model"`
	StandardID       string          `json:"standard_id"`
	Buildings        []string        `json:"buildings"`
	SpacingM         float64         `json:"spacing_m"`
	IncludeCourtyard bool            `json:"include_courtyard,omitempty"`
	AllowUnconfirmed bool            `json:"allow_unconfirmed,omitempty"`
}

// FacadeReceiver is one generated receiver: the point a run computes at, and
// the attributes its receiver table row carries, in results.ReceiverRecord's
// own field names and shape, plus the two review fields the gate counts.
type FacadeReceiver struct {
	geo.PointReceiver

	PlacementRule    string                 `json:"placement_rule"`
	Facade           results.ReceiverFacade `json:"facade"`
	Storey           int                    `json:"storey"`
	SlabHeightSource string                 `json:"slab_height_source"`
	ReviewStatus     string                 `json:"review_status"`
}

// FacadeReceiversResponse is the generated set in the order a run computes
// it, the counts a run summary records, and what provenance records about the
// request — the keys `aconiq run` writes, `allow_unconfirmed` included.
type FacadeReceiversResponse struct {
	Receivers          []FacadeReceiver  `json:"receivers"`
	ProposedCount      int               `json:"proposed_count"`
	AssumedHeightCount int               `json:"assumed_height_count"`
	Metadata           map[string]string `json:"metadata"`
}

// FacadeReceivers places the facade receivers of a browser-mode run, through
// facadeset — the package `aconiq run --receiver-mode facade` calls — and
// refuses what that run refuses, in its words. The browser must not place a
// facade point of its own: the station rule, the party-wall mask and the
// storey heights would be a second implementation of each.
func FacadeReceivers(input []byte) ([]byte, error) {
	var req FacadeReceiversRequest

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.CRS == "" {
		return nil, errors.New("crs is required: the compute CRS the model's coordinates are in")
	}

	if len(req.Buildings) == 0 {
		return nil, errors.New(`receiver mode facade needs at least one building, by feature id or "<street> <number>"`)
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "facadeReceivers")
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}

	listed, metadata, err := facadeset.Generate(facadeset.Request{
		Buildings:        req.Buildings,
		SpacingM:         req.SpacingM,
		IncludeCourtyard: req.IncludeCourtyard,
	}, req.StandardID, model)
	if err != nil {
		return nil, fmt.Errorf("facade receivers: %w", err)
	}

	standard, err := resolveStandard(req.StandardID)
	if err != nil {
		return nil, err
	}

	err = facadeset.Gate(listed, standard, model, req.AllowUnconfirmed)
	if err != nil {
		return nil, fmt.Errorf("facade receivers: %w", err)
	}

	resp := FacadeReceiversResponse{
		Receivers: make([]FacadeReceiver, 0, len(listed.Receivers)),
		Metadata:  metadata,
	}

	resp.ProposedCount, resp.AssumedHeightCount = listed.CountUnconfirmed()
	if req.AllowUnconfirmed {
		resp.Metadata["allow_unconfirmed"] = "true"
	}

	for _, receiver := range listed.Receivers {
		attrs := listed.Facade[receiver.ID]
		key := attrs.Facade

		resp.Receivers = append(resp.Receivers, FacadeReceiver{
			PointReceiver: receiver,
			PlacementRule: attrs.PlacementRule,
			Facade: results.ReceiverFacade{
				BuildingRef: key.BuildingRef,
				Part:        key.Part,
				Ring:        key.Ring,
				Edge:        key.Edge,
				From:        [2]float64{key.From.X, key.From.Y},
				To:          [2]float64{key.To.X, key.To.Y},
			},
			Storey:           *attrs.Storey,
			SlabHeightSource: attrs.SlabHeightSource,
			ReviewStatus:     attrs.ReviewStatus,
		})
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}

// resolveStandard resolves a standard the kernel runs at its default version
// and profile, which is what carries the evidence tier the gate reads.
func resolveStandard(id string) (framework.ResolvedProfile, error) {
	for _, standard := range Standards() {
		if standard.Descriptor.ID == id {
			resolved, err := standard.Descriptor.ResolveVersionProfile("", "")
			if err != nil {
				return framework.ResolvedProfile{}, fmt.Errorf("resolve %s: %w", id, err)
			}

			return resolved, nil
		}
	}

	return framework.ResolvedProfile{}, fmt.Errorf("the kernel does not run standard %q", id)
}
