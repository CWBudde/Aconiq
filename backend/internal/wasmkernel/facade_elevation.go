package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// FacadeElevationRequest is the JSON `aconiq.facadeElevation` takes.
//
// Model is a GeoJSON FeatureCollection in the v1 model schema, in CRS, holding
// the building and the windows keyed to it. The browser sends it in the compute
// CRS it sends a facade run, projected through `transform`, so the facades are
// the canonical edges the run's keys are numbered on (facade-receivers.md, The
// facade key). Building is the building's feature id.
type FacadeElevationRequest struct {
	CRS      string          `json:"crs"`
	Model    json.RawMessage `json:"model"`
	Building string          `json:"building"`
}

// FacadeElevation unfolds one building into its facades, storeys and windows
// for the elevation view (P18 C2), through facadeset.Elevation — the code
// `POST /api/v1/facades/elevation` answers from — so the browser measures no
// facade of its own. It answers facadeset.BuildingElevation as JSON.
func FacadeElevation(input []byte) ([]byte, error) {
	var req FacadeElevationRequest

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.CRS == "" {
		return nil, errors.New("crs is required: the CRS the model's coordinates are in")
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "facadeElevation")
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}

	elevation, err := facadeset.Elevation(model, req.Building)
	if err != nil {
		return nil, fmt.Errorf("facade elevation: %w", err)
	}

	out, err := json.Marshal(elevation)
	if err != nil {
		return nil, fmt.Errorf("encode facade elevation: %w", err)
	}

	return out, nil
}

// PlaceWindowRequest is the JSON `aconiq.placeWindow` takes: a window drawn
// or moved in the elevation view, and the model holding its building, in CRS
// — the compute CRS FacadeElevation was asked in, so Part, Ring and Edge name
// the edge the view showed.
type PlaceWindowRequest struct {
	CRS    string                    `json:"crs"`
	Model  json.RawMessage           `json:"model"`
	Window facadeset.WindowPlacement `json:"window"`
}

// PlaceWindowResponse is the window feature the model stores, in CRS.
type PlaceWindowResponse struct {
	Feature modelgeojson.GeoJSONFeature `json:"feature"`
}

// PlaceWindow places one window of the elevation view through
// facadeset.PlaceWindow — the code `POST /api/v1/facades/window` answers
// from, and the placer `aconiq import --windows` uses — so the browser keys
// no window and measures no frame of its own.
func PlaceWindow(input []byte) ([]byte, error) {
	var req PlaceWindowRequest

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.CRS == "" {
		return nil, errors.New("crs is required: the CRS the model's coordinates are in")
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "placeWindow")
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}

	feature, err := facadeset.PlaceWindow(model, req.Window)
	if err != nil {
		return nil, fmt.Errorf("place window: %w", err)
	}

	out, err := json.Marshal(PlaceWindowResponse{Feature: feature})
	if err != nil {
		return nil, fmt.Errorf("encode window: %w", err)
	}

	return out, nil
}

// RectifyFacade solves the homography that draws a photo of a facade onto
// the elevation view (P18 D1), through facadeset.Rectify — the code `POST
// /api/v1/facades/rectify` answers from, and the solve window validation
// checks a window's photo evidence with. It takes the four point pairs as
// facadeset.PhotoTransform JSON and answers facadeset.Rectification. The
// photo stays in the browser: only the pairs cross.
func RectifyFacade(input []byte) ([]byte, error) {
	var req facadeset.PhotoTransform

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	rect, err := facadeset.Rectify(req)
	if err != nil {
		return nil, fmt.Errorf("rectify facade: %w", err)
	}

	out, err := json.Marshal(rect)
	if err != nil {
		return nil, fmt.Errorf("encode rectification: %w", err)
	}

	return out, nil
}
