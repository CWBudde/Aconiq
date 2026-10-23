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
