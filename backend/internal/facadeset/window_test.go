package facadeset

import (
	"maps"
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// windowModel is a synthetic 10 m × 8 m house of three storeys in
// EPSG:25832, with the windows given on its south facade (canonical edge 0,
// from (547000, 5795000) east).
func windowModel(t *testing.T, windows ...modelgeojson.WindowSpec) modelgeojson.Model {
	t.Helper()

	height := 9.0
	model := modelgeojson.Model{ProjectCRS: "EPSG:25832", Features: []modelgeojson.Feature{{
		ID: "house", Kind: modelgeojson.FeatureKindBuilding, GeometryType: modelgeojson.GeometryTypePolygon,
		HeightM:    &height,
		Properties: map[string]any{"building:levels": "3", "height_source": "measured"},
		Coordinates: []any{[]any{
			[]any{547000.0, 5795000.0},
			[]any{547010.0, 5795000.0},
			[]any{547010.0, 5795008.0},
			[]any{547000.0, 5795008.0},
			[]any{547000.0, 5795000.0},
		}},
	}}}

	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	for _, spec := range windows {
		feature, err := placer.Feature(spec)
		if err != nil {
			t.Fatalf("window %q: %v", spec.ID, err)
		}

		model.Features = append(model.Features, feature)
	}

	return model
}

func windowSpec(id string, uM float64, sensitive bool, status string) modelgeojson.WindowSpec {
	return modelgeojson.WindowSpec{
		ID: id, BuildingRef: "house",
		Frame:     geo.FacadeWindow{UM: uM, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Sensitive: sensitive, ReviewStatus: status,
	}
}

var houseRequest = Request{Buildings: []string{"house"}, SpacingM: 4}

func TestGenerateAtSensitiveWindows(t *testing.T) {
	t.Parallel()

	model := windowModel(
		t,
		windowSpec("bedroom", 2, true, geo.ReviewStatusConfirmed),
		windowSpec("stairs", 5, false, geo.ReviewStatusConfirmed),
		windowSpec("gone", 7, true, geo.ReviewStatusRejected),
	)

	listed, metadata, err := Generate(houseRequest, rls19RoadID, model)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var atWindow []string

	for _, receiver := range listed.Receivers {
		attrs := listed.Facade[receiver.ID]
		if attrs.Facade.Edge != 0 {
			continue
		}

		atWindow = append(atWindow, receiver.ID)

		if attrs.WindowID != "bedroom" || attrs.Window == nil || attrs.HeightBasis != geo.HeightBasisSchall03WindowHead ||
			attrs.ReviewStatus != geo.ReviewStatusConfirmed || attrs.SlabHeightSource != "" {
			t.Errorf("attrs = %+v, want the bedroom window's, confirmed, with the borrowed basis", attrs)
		}

		if receiver.HeightM < 5.6-1e-9 || receiver.HeightM > 5.6+1e-9 {
			t.Errorf("height = %v, want the window head + 0.2 m", receiver.HeightM)
		}
	}

	// Only the sensitive window that is not rejected carries a point, and
	// it replaces the station grid on its facade.
	if strings.Join(atWindow, ",") != "facade/house/p0r0e0/w/bedroom" {
		t.Errorf("south facade = %v, want the bedroom window's point alone", atWindow)
	}

	if len(listed.Receivers) == len(atWindow) {
		t.Error("the facades without windows lost their stations")
	}

	if metadata["facade_window_count"] != "1" || metadata["facade_window_height_basis"] != geo.HeightBasisSchall03WindowHead {
		t.Errorf("metadata = %v, want one window point, its height borrowed from Schall 03", metadata)
	}
}

// A sensitive window on a party wall cannot carry a point. Dropping it
// quietly would report fewer Immissionsorte than the window list holds.
func TestGenerateRefusesAWindowOnAPartyWall(t *testing.T) {
	t.Parallel()

	model := windowModel(t, modelgeojson.WindowSpec{
		ID: "behind-the-wall", BuildingRef: "house", Edge: 1,
		Frame:     geo.FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Sensitive: true,
	})

	// The neighbour shares the east wall, canonical edge 1 of the house.
	height := 9.0
	model.Features = append(model.Features, modelgeojson.Feature{
		ID: "neighbour", Kind: modelgeojson.FeatureKindBuilding, GeometryType: modelgeojson.GeometryTypePolygon,
		HeightM: &height,
		Coordinates: []any{[]any{
			[]any{547010.0, 5795000.0},
			[]any{547020.0, 5795000.0},
			[]any{547020.0, 5795008.0},
			[]any{547010.0, 5795008.0},
			[]any{547010.0, 5795000.0},
		}},
	})

	for _, standard := range []string{rls19RoadID, iso9613ID} {
		_, _, err := Generate(houseRequest, standard, model)
		if err == nil || !strings.Contains(err.Error(), "behind-the-wall") || !strings.Contains(err.Error(), "party wall") {
			t.Errorf("%s: err = %v, want the window named", standard, err)
		}
	}
}

// TA Lärm places at windows only: ISO 9613-2, which computes it, has a facade
// mode that needs them, and says so when the buildings have none.
func TestGenerateTALaermAtWindowsOnly(t *testing.T) {
	t.Parallel()

	listed, metadata, err := Generate(houseRequest, iso9613ID, windowModel(t, windowSpec("bedroom", 2, true, "")))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(listed.Receivers) != 1 || listed.Receivers[0].ID != "facade/house/p0r0e0/w/bedroom" {
		t.Fatalf("receivers = %+v, want the bedroom window's point alone", listed.Receivers)
	}

	if r := listed.Receivers[0]; math.Abs(r.HeightM-4.65) > 1e-9 || r.Point.Y != 5795000-0.5 {
		t.Errorf("point = %+v, want 0.5 m out at the window centre", r)
	}

	if listed.Facade[listed.Receivers[0].ID].ReviewStatus != geo.ReviewStatusProposed {
		t.Error("a window without a review status is proposed")
	}

	if metadata["facade_placement_rule"] != geo.PlacementTALaermWindow {
		t.Errorf("metadata = %v", metadata)
	}

	for name, model := range map[string]modelgeojson.Model{
		"no windows":         windowModel(t),
		"no sensitive":       windowModel(t, windowSpec("stairs", 2, false, "")),
		"only rejected ones": windowModel(t, windowSpec("gone", 2, true, geo.ReviewStatusRejected)),
	} {
		_, _, err := Generate(houseRequest, iso9613ID, model)
		if err == nil || !strings.Contains(err.Error(), "talaerm-window") || !strings.Contains(err.Error(), "--windows") {
			t.Errorf("%s: err = %v, want the window-only rule explained", name, err)
		}
	}
}

// A window whose facade link broke is not placed at, for any standard: its
// frame is measured along an edge that is no longer the one it names.
func TestGenerateRefusesABrokenWindowLink(t *testing.T) {
	t.Parallel()

	model := windowModel(t, windowSpec("bedroom", 2, true, geo.ReviewStatusConfirmed))
	window := model.Features[1]
	window.Properties = maps.Clone(window.Properties)
	facade := maps.Clone(window.Properties[modelgeojson.PropertyFacade].(map[string]any))
	facade["from"] = []any{547000.5, 5795000.0}
	window.Properties[modelgeojson.PropertyFacade] = facade
	model.Features[1] = window

	for _, standard := range []string{rls19RoadID, iso9613ID} {
		_, _, err := Generate(houseRequest, standard, model)
		if err == nil || !strings.Contains(err.Error(), modelgeojson.CodeWindowFacadeMoved) || !strings.Contains(err.Error(), "bedroom") {
			t.Errorf("%s: err = %v, want the moved window named", standard, err)
		}
	}
}

// A window above its building's roof stands in no storey. Placing a point
// there would put an Immissionsort above the building the model describes.
func TestGenerateRefusesAWindowAboveTheRoof(t *testing.T) {
	t.Parallel()

	high := windowSpec("attic", 2, true, geo.ReviewStatusConfirmed)
	high.Frame.SillZM = 8.5 // a 1.5 m window, its head at 10 m on a 9 m house

	model := windowModel(t, high)

	for _, standard := range []string{rls19RoadID, iso9613ID} {
		_, _, err := Generate(houseRequest, standard, model)
		if err == nil || !strings.Contains(err.Error(), modelgeojson.CodeWindowAboveBuilding) || !strings.Contains(err.Error(), "attic") {
			t.Errorf("%s: err = %v, want the window above the roof named", standard, err)
		}
	}

	// The same window, not sensitive, carries no point and refuses nothing.
	high.Sensitive = false

	_, _, err := Generate(houseRequest, rls19RoadID, windowModel(t, high))
	if err != nil {
		t.Errorf("a non-sensitive window above the roof: %v", err)
	}
}

func TestGenerateNamesTheStandardsWithAFacadeMode(t *testing.T) {
	t.Parallel()

	_, _, err := Generate(houseRequest, "cnossos-road", windowModel(t))
	if err == nil || !strings.Contains(err.Error(), "iso9613, rls19-road and schall03") {
		t.Errorf("err = %v, want the three standards listed", err)
	}
}
