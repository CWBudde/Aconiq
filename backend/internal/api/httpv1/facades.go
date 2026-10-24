package httpv1

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// facadeElevationRequest is the body of POST /api/v1/facades/elevation, and
// facadeElevationResponse is what it answers with.
//
// They are declared here rather than reused from facadeset for the reason
// transform.go gives: httpv1 owns its wire contract, and facadeset's types
// also serve the WebAssembly kernel's `facadeElevation`, which must not be able
// to move an HTTP contract as a side effect.
// TestFacadeElevationDTOsMatchTheFacadesetContract is what stops the two
// drifting while they are meant to agree.
type facadeElevationRequest struct {
	CRS      string          `json:"crs"`
	Model    json.RawMessage `json:"model"`
	Building string          `json:"building"`
}

// facadeBuildingField and facadeWindowField are request members, as the
// refusals' details and the OpenAPI document spell them.
const (
	facadeBuildingField = "building"
	facadeWindowField   = "window"
)

type facadeElevationResponse struct {
	BuildingRef      string                    `json:"building_ref"`
	HeightM          float64                   `json:"height_m"`
	Levels           int                       `json:"levels,omitempty"`
	SlabHeightsM     []float64                 `json:"slab_heights_m"`
	SlabHeightSource string                    `json:"slab_height_source"`
	Facades          []facadeResponse          `json:"facades"`
	Detached         []elevationWindowResponse `json:"detached"`
}

type facadeResponse struct {
	Part       int                       `json:"part"`
	Ring       int                       `json:"ring"`
	Edge       int                       `json:"edge"`
	From       [2]float64                `json:"from"`
	To         [2]float64                `json:"to"`
	LengthM    float64                   `json:"length_m"`
	FacingDeg  float64                   `json:"facing_deg"`
	Courtyard  bool                      `json:"courtyard"`
	PartyWalls []facadeSpanResponse      `json:"party_walls"`
	Windows    []elevationWindowResponse `json:"windows"`
}

type facadeSpanResponse struct {
	FromUM float64 `json:"from_um"`
	ToUM   float64 `json:"to_um"`
}

type elevationWindowResponse struct {
	ID           string                     `json:"id"`
	Window       windowFrameResponse        `json:"window"`
	Sensitive    bool                       `json:"sensitive"`
	ReviewStatus string                     `json:"review_status"`
	OnPartyWall  bool                       `json:"on_party_wall"`
	Findings     []elevationFindingResponse `json:"findings"`
}

type windowFrameResponse struct {
	UM      float64 `json:"u_m"`
	SillZM  float64 `json:"sill_z_m"`
	WidthM  float64 `json:"width_m"`
	HeightM float64 `json:"height_m"`
}

// elevationFindingResponse keeps the level validationIssueResponse leaves
// out: a window's findings are one list, so it is the level that says whether
// the window refuses a save or only a facade run.
type elevationFindingResponse struct {
	Level     string `json:"level"`
	Code      string `json:"code"`
	FeatureID string `json:"feature_id,omitempty"`
	Message   string `json:"message"`
}

// handleFacadeElevation unfolds one building of the model in the body into its
// facades, storeys and windows, for the frontend's elevation view in API mode
// (P18 C2). Browser mode asks the WebAssembly kernel's `facadeElevation`, and
// both answer from facadeset.Elevation, so neither mode measures a facade of
// its own.
//
// Like POST /api/v1/transform it is a pure function of its body: it reads no
// project, so it answers for the model the map holds, saved or not, and it
// cannot write one. The model is capped as POST /api/v1/model caps it.
func (h Handler) handleFacadeElevation(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req facadeElevationRequest
	if !decodeJSONBody(w, r, maxModelSaveBodyBytes, &req) {
		return
	}

	if strings.TrimSpace(req.CRS) == "" || strings.TrimSpace(req.Building) == "" || isJSONAbsent(req.Model) {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "crs, model and building are required",
			Hint:    "Send the model as a GeoJSON FeatureCollection, the CRS its coordinates are in, and the building's feature id.",
		})

		return
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "facadeElevation")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "model is not a valid GeoJSON FeatureCollection: " + err.Error(),
		})

		return
	}

	elevation, err := facadeset.Elevation(model, req.Building)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
			Details: map[string]any{facadeBuildingField: req.Building},
		})

		return
	}

	writeJSON(w, http.StatusOK, facadeElevationResponseOf(elevation))
}

func facadeElevationResponseOf(e facadeset.BuildingElevation) facadeElevationResponse {
	resp := facadeElevationResponse{
		BuildingRef:      e.BuildingRef,
		HeightM:          e.HeightM,
		Levels:           e.Levels,
		SlabHeightsM:     e.SlabHeightsM,
		SlabHeightSource: e.SlabHeightSource,
		Facades:          make([]facadeResponse, len(e.Facades)),
		Detached:         elevationWindowsResponse(e.Detached),
	}

	for i, f := range e.Facades {
		walls := make([]facadeSpanResponse, len(f.PartyWalls))
		for j, s := range f.PartyWalls {
			walls[j] = facadeSpanResponse{FromUM: s.FromUM, ToUM: s.ToUM}
		}

		resp.Facades[i] = facadeResponse{
			Part: f.Part, Ring: f.Ring, Edge: f.Edge,
			From: f.From, To: f.To,
			LengthM:    f.LengthM,
			FacingDeg:  f.FacingDeg,
			Courtyard:  f.Courtyard,
			PartyWalls: walls,
			Windows:    elevationWindowsResponse(f.Windows),
		}
	}

	return resp
}

func elevationWindowsResponse(windows []facadeset.ElevationWindow) []elevationWindowResponse {
	out := make([]elevationWindowResponse, len(windows))

	for i, w := range windows {
		findings := make([]elevationFindingResponse, len(w.Findings))
		for j, f := range w.Findings {
			findings[j] = elevationFindingResponse{Level: f.Level, Code: f.Code, FeatureID: f.FeatureID, Message: f.Message}
		}

		out[i] = elevationWindowResponse{
			ID: w.ID,
			Window: windowFrameResponse{
				UM: w.Frame.UM, SillZM: w.Frame.SillZM, WidthM: w.Frame.WidthM, HeightM: w.Frame.HeightM,
			},
			Sensitive:    w.Sensitive,
			ReviewStatus: w.ReviewStatus,
			OnPartyWall:  w.OnPartyWall,
			Findings:     findings,
		}
	}

	return out
}

// placeWindowRequest is the body of POST /api/v1/facades/window, and
// placeWindowResponse is what it answers with; declared here for the reason
// facadeElevationRequest is, and held to facadeset's JSON by
// TestPlaceWindowDTOsMatchTheFacadesetContract.
type placeWindowRequest struct {
	CRS    string               `json:"crs"`
	Model  json.RawMessage      `json:"model"`
	Window windowPlacementInput `json:"window"`
}

type windowPlacementInput struct {
	ID           string              `json:"id"`
	BuildingRef  string              `json:"building_ref"`
	Part         int                 `json:"part"`
	Ring         int                 `json:"ring"`
	Edge         int                 `json:"edge"`
	Window       windowFrameResponse `json:"window"`
	Sensitive    bool                `json:"sensitive"`
	ReviewStatus string              `json:"review_status,omitempty"`
	Evidence     []any               `json:"evidence,omitempty"`
}

type placeWindowResponse struct {
	Feature placedFeatureResponse `json:"feature"`
}

type placedFeatureResponse struct {
	Type       string                 `json:"type"`
	ID         any                    `json:"id,omitempty"`
	Properties map[string]any         `json:"properties"`
	Geometry   placedGeometryResponse `json:"geometry"`
}

type placedGeometryResponse struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

// handleFacadeWindow places one window drawn or moved in the frontend's
// elevation view in API mode (P18 C2), through facadeset.PlaceWindow — the
// code the WebAssembly kernel's `placeWindow` calls, and the placer `aconiq
// import --windows` uses — so neither mode keys a window of its own. Like the
// elevation it is a pure function of its body: the client stores the feature
// it answers with in its model, and a save writes it.
func (h Handler) handleFacadeWindow(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req placeWindowRequest
	if !decodeJSONBody(w, r, maxModelSaveBodyBytes, &req) {
		return
	}

	if strings.TrimSpace(req.CRS) == "" || isJSONAbsent(req.Model) {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "crs and model are required",
			Hint:    "Send the model holding the window's building, the CRS its coordinates are in, and the window.",
		})

		return
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "placeWindow")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "model is not a valid GeoJSON FeatureCollection: " + err.Error(),
		})

		return
	}

	feature, err := facadeset.PlaceWindow(model, req.Window.placement())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
			Details: map[string]any{facadeWindowField: req.Window.ID},
		})

		return
	}

	writeJSON(w, http.StatusOK, placeWindowResponseOf(feature))
}

func (in windowPlacementInput) placement() facadeset.WindowPlacement {
	return facadeset.WindowPlacement{
		ID: in.ID, BuildingRef: in.BuildingRef,
		Part: in.Part, Ring: in.Ring, Edge: in.Edge,
		Frame: geo.FacadeWindow{
			UM: in.Window.UM, SillZM: in.Window.SillZM, WidthM: in.Window.WidthM, HeightM: in.Window.HeightM,
		},
		Sensitive:    in.Sensitive,
		ReviewStatus: in.ReviewStatus,
		Evidence:     in.Evidence,
	}
}

func placeWindowResponseOf(f modelgeojson.GeoJSONFeature) placeWindowResponse {
	return placeWindowResponse{Feature: placedFeatureResponse{
		Type:       f.Type,
		ID:         f.ID,
		Properties: f.Properties,
		Geometry:   placedGeometryResponse{Type: f.Geometry.Type, Coordinates: f.Geometry.Coordinates},
	}}
}
