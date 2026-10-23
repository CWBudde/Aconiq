package facadeset

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// BuildingElevation is a building unfolded into its facades (P18 C2): what an
// elevation view draws, measured by the code that places windows and facade
// receivers, so the browser grows no second facade frame. Each facade is one
// canonical edge, in canonical order, with the windows keyed to it.
//
// SlabHeightsM are the storeys' ceiling slabs above the ground, ground floor
// first, at the heights facade mode places its points (geo.StoreySlabs), and
// SlabHeightSource says where their storey height came from. Detached holds
// the windows keyed to this building whose key no longer names one of its
// edges as it is (window.facade.moved), so a view can list them rather than
// draw them on the wrong facade. A window whose building_ref names no
// building of the model (window.facade.unresolved) is in no building's
// elevation: nothing says which building it meant, and the validation report
// lists it.
type BuildingElevation struct {
	BuildingRef      string            `json:"building_ref"`
	HeightM          float64           `json:"height_m"`
	Levels           int               `json:"levels,omitempty"`
	SlabHeightsM     []float64         `json:"slab_heights_m"`
	SlabHeightSource string            `json:"slab_height_source"`
	Facades          []FacadeElevation `json:"facades"`
	Detached         []ElevationWindow `json:"detached"`
}

// FacadeElevation is one facade: its canonical edge, with from / to in the
// model's CRS, as a window on it is keyed, its length in metres, the extent
// u_m and width_m are measured along, and the bearing it faces from grid
// north (geo.FacadeFacingDeg). Courtyard is an inner ring's edge.
//
// PartyWalls are the stretches of it another building of the model stands
// against, found by the probe facade mode drops a point by
// (geo.PartyWallSpans): it places no station there, and a run over the facade
// (a courtyard one only when courtyards are included) refuses a sensitive
// window that is not rejected whose centre is on one.
type FacadeElevation struct {
	Part       int               `json:"part"`
	Ring       int               `json:"ring"`
	Edge       int               `json:"edge"`
	From       [2]float64        `json:"from"`
	To         [2]float64        `json:"to"`
	LengthM    float64           `json:"length_m"`
	FacingDeg  float64           `json:"facing_deg"`
	Courtyard  bool              `json:"courtyard"`
	PartyWalls []geo.FacadeSpan  `json:"party_walls"`
	Windows    []ElevationWindow `json:"windows"`
}

// ElevationWindow is a window feature on the facade it names, with what
// validation finds about it: a window that runs past its edge or above the
// roof is drawn where it is, and says so. OnPartyWall is a window whose centre
// stands on one of its facade's party walls, where facade mode probes it; a
// detached window is on no facade, and never is.
type ElevationWindow struct {
	ID           string                         `json:"id"`
	Frame        geo.FacadeWindow               `json:"window"`
	Sensitive    bool                           `json:"sensitive"`
	ReviewStatus string                         `json:"review_status"`
	OnPartyWall  bool                           `json:"on_party_wall"`
	Findings     []modelgeojson.ValidationIssue `json:"findings"`
}

// Elevation unfolds one building of the model, named by its feature id. It
// refuses a building that is not in the model, has no stable id, or has no
// height to draw its storeys from; a geographic model is measured in the
// compute CRS `aconiq run` would choose for it, as validation measures it.
func Elevation(model modelgeojson.Model, buildingRef string) (BuildingElevation, error) {
	ref := strings.TrimSpace(buildingRef)

	feature, ok := buildingFeature(model, ref)
	if !ok {
		return BuildingElevation{}, fmt.Errorf("building %q is not in the model", ref)
	}

	building, err := BuildingFromFeature(feature)
	if err != nil {
		return BuildingElevation{}, fmt.Errorf("building %q: %w", ref, err)
	}

	slabs, source, err := geo.StoreySlabs(building)
	if err != nil {
		return BuildingElevation{}, fmt.Errorf("building %q: %w", ref, err)
	}

	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		return BuildingElevation{}, fmt.Errorf("no metric frame to measure building %q in: %w", ref, err)
	}

	edges, err := placer.Edges(ref)
	if err != nil {
		return BuildingElevation{}, fmt.Errorf("unfold: %w", err)
	}

	elevation := BuildingElevation{
		BuildingRef:      ref,
		HeightM:          building.HeightM,
		Levels:           building.Levels,
		SlabHeightsM:     slabs,
		SlabHeightSource: source,
		Facades:          make([]FacadeElevation, len(edges)),
		Detached:         []ElevationWindow{},
	}

	onEdge := make(map[[3]int]int, len(edges))

	for i, e := range edges {
		key := e.Key
		elevation.Facades[i] = FacadeElevation{
			Part: key.Part, Ring: key.Ring, Edge: key.Edge,
			From:       [2]float64{key.From.X, key.From.Y},
			To:         [2]float64{key.To.X, key.To.Y},
			LengthM:    e.LengthM,
			FacingDeg:  e.FacingDeg,
			Courtyard:  key.Ring > 0,
			PartyWalls: append([]geo.FacadeSpan{}, e.PartyWalls...),
			Windows:    []ElevationWindow{},
		}
		onEdge[[3]int{key.Part, key.Ring, key.Edge}] = i
	}

	findings := windowFindings(model)

	for _, f := range model.Features {
		if f.Kind != modelgeojson.FeatureKindWindow {
			continue
		}

		// A window validation refuses as malformed names no frame to draw; the
		// validation report lists it.
		window, err := modelgeojson.WindowOf(f)
		if err != nil || window.Facade.BuildingRef != ref {
			continue
		}

		drawn := ElevationWindow{
			ID: window.ID, Frame: window.Frame, Sensitive: window.Sensitive, ReviewStatus: window.ReviewStatus,
			Findings: findings[window.ID],
		}
		if drawn.Findings == nil {
			drawn.Findings = []modelgeojson.ValidationIssue{}
		}

		i, exists := onEdge[[3]int{window.Facade.Part, window.Facade.Ring, window.Facade.Edge}]
		if !exists || detached(drawn.Findings) {
			elevation.Detached = append(elevation.Detached, drawn)

			continue
		}

		drawn.OnPartyWall = onPartyWall(elevation.Facades[i].PartyWalls, window.Frame.CentreUM())
		elevation.Facades[i].Windows = append(elevation.Facades[i].Windows, drawn)
	}

	return elevation, nil
}

// buildingFeature finds a building by its feature id.
func buildingFeature(model modelgeojson.Model, ref string) (modelgeojson.Feature, bool) {
	for _, f := range model.Features {
		if f.Kind == modelgeojson.FeatureKindBuilding && strings.TrimSpace(f.ID) == ref {
			return f, true
		}
	}

	return modelgeojson.Feature{}, false
}

// windowFindings is the validation report's errors and warnings, by feature
// id.
func windowFindings(model modelgeojson.Model) map[string][]modelgeojson.ValidationIssue {
	report := modelgeojson.Validate(model)
	byID := make(map[string][]modelgeojson.ValidationIssue)

	for _, issue := range append(report.Errors, report.Warnings...) {
		if issue.FeatureID != "" {
			byID[issue.FeatureID] = append(byID[issue.FeatureID], issue)
		}
	}

	return byID
}

// detached is a window whose key names no edge of its building as it is now.
// Only moved can reach here: a key naming this building names one the model
// has, so it is never unresolved.
func detached(findings []modelgeojson.ValidationIssue) bool {
	return slices.ContainsFunc(findings, func(issue modelgeojson.ValidationIssue) bool {
		return issue.Code == modelgeojson.CodeWindowFacadeMoved
	})
}

// onPartyWall is a point u metres along a facade that stands inside one of its
// party walls. The ends are excluded, as the strict mask facade mode applies
// excludes them.
func onPartyWall(spans []geo.FacadeSpan, u float64) bool {
	for _, s := range spans {
		if s.FromUM < u && u < s.ToUM {
			return true
		}
	}

	return false
}
