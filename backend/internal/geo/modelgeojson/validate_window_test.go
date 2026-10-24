package modelgeojson

import (
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// bedroomSpec is a window on edge 0 of simpleFacadeFootprint, which runs
// 20 m east from (facadeX0, facadeY0): its left edge at u = 2, 1.2 m wide,
// 1.5 m tall, its sill at 3.9 m.
func bedroomSpec(id string) WindowSpec {
	return WindowSpec{
		ID:          id,
		BuildingRef: "b1",
		Frame:       geo.FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Sensitive:   true,
	}
}

func mustWindowFeature(t *testing.T, model Model, spec WindowSpec) Feature {
	t.Helper()

	placer, err := NewWindowPlacer(model)
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	feature, err := placer.Feature(spec)
	if err != nil {
		t.Fatalf("Feature(%q): %v", spec.ID, err)
	}

	return feature
}

// oneWindow is a model of building b1 and one window on it, whose
// properties a test may then change.
func oneWindow(t *testing.T) (Model, Feature) {
	t.Helper()

	model := facadeModel(buildingFeature("b1", simpleFacadeFootprint()))
	window := mustWindowFeature(t, model, bedroomSpec("w1"))
	window.Properties = maps.Clone(window.Properties)
	model.Features = append(model.Features, window)

	return model, window
}

func TestWindowFeatureAsPlaced(t *testing.T) {
	t.Parallel()

	model, window := oneWindow(t)
	assertCleanReport(t, Validate(model))

	if window.Kind != FeatureKindWindow || window.GeometryType != GeometryTypePoint || window.HeightM != nil {
		t.Fatalf("window = %+v, want a window Point without height_m", window)
	}

	// The geometry is the window centre on the facade plane, for the map.
	coords, ok := window.Coordinates.([]any)
	if !ok || len(coords) != 2 || coords[0] != facadeX0+2.6 || coords[1] != facadeY0 {
		t.Errorf("coordinates = %#v, want [%v, %v]", window.Coordinates, facadeX0+2.6, facadeY0)
	}

	got, err := WindowOf(window)
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	want := geo.FacadeKey{
		BuildingRef: "b1", From: geo.Point2D{X: facadeX0, Y: facadeY0}, To: geo.Point2D{X: facadeX0 + 20, Y: facadeY0},
	}
	if got.Facade != want || got.Frame != bedroomSpec("").Frame || !got.Sensitive || got.ReviewStatus != geo.ReviewStatusProposed {
		t.Errorf("WindowOf = %+v, want the key of edge 0, the frame, sensitive and proposed", got)
	}
}

func TestWindowPlacerRefuses(t *testing.T) {
	t.Parallel()

	model := facadeModel(buildingFeature("b1", simpleFacadeFootprint()))

	placer, err := NewWindowPlacer(model)
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	with := func(edit func(*WindowSpec)) WindowSpec {
		spec := bedroomSpec("w1")
		edit(&spec)

		return spec
	}

	cases := map[string]WindowSpec{
		"no building":     with(func(s *WindowSpec) { s.BuildingRef = "b2" }),
		"no such edge":    with(func(s *WindowSpec) { s.Edge = 4 }),
		"no such ring":    with(func(s *WindowSpec) { s.Ring = 1 }),
		"zero width":      with(func(s *WindowSpec) { s.Frame.WidthM = 0 }),
		"past the corner": with(func(s *WindowSpec) { s.Frame.UM = 19.5 }),
		"blank id":        with(func(s *WindowSpec) { s.ID = " " }),
		"slash in id":     with(func(s *WindowSpec) { s.ID = "a/b" }),
		"unknown review":  with(func(s *WindowSpec) { s.ReviewStatus = "maybe" }),
	}

	for name, spec := range cases {
		if _, err := placer.Feature(spec); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateWindowInvalid(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*Feature){
		"no facade":          func(w *Feature) { delete(w.Properties, PropertyFacade) },
		"no frame":           func(w *Feature) { delete(w.Properties, PropertyWindow) },
		"frame not object":   func(w *Feature) { w.Properties[PropertyWindow] = "1x1" },
		"zero width":         func(w *Feature) { w.Properties[PropertyWindow] = frameWith("width_m", 0.0) },
		"string sill":        func(w *Feature) { w.Properties[PropertyWindow] = frameWith("sill_z_m", "1") },
		"sensitive string":   func(w *Feature) { w.Properties[PropertySensitive] = "yes" },
		"unknown review":     func(w *Feature) { w.Properties[PropertyReviewStatus] = "maybe" },
		"evidence an object": func(w *Feature) { w.Properties[PropertyEvidence] = map[string]any{"kind": "plan"} },
		"evidence of string": func(w *Feature) { w.Properties[PropertyEvidence] = []any{"photo.jpg"} },
		"slash in id":        func(w *Feature) { w.ID = "w/1" },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, window := oneWindow(t)
			mutate(&window)
			model.Features[1] = window

			report := Validate(model)
			if report.Valid || len(issuesWithCode(report.Errors, CodeWindowInvalid)) != 1 {
				t.Fatalf("errors = %#v, want one %s", report.Errors, CodeWindowInvalid)
			}
		})
	}

	model, window := oneWindow(t)
	window.GeometryType = GeometryTypeLineString
	window.Coordinates = []any{[]any{facadeX0, facadeY0}, []any{facadeX0 + 1, facadeY0}}
	model.Features[1] = window

	if report := Validate(model); len(issuesWithCode(report.Errors, "window.geometry.invalid")) != 1 {
		t.Errorf("errors = %#v, want window.geometry.invalid", report.Errors)
	}
}

func frameWith(member string, value any) map[string]any {
	frame := map[string]any{"u_m": 2.0, "sill_z_m": 3.9, "width_m": 1.2, "height_m": 1.5}
	frame[member] = value

	return frame
}

// A window whose building changed under it is reported as a receiver is,
// with its own codes, and so is one the edge no longer has room for. All of
// them are warnings: the import that changed the building has to succeed.
func TestValidateWindowAgainstItsBuilding(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*Feature)
		code   string
	}{
		"unresolved": {func(w *Feature) {
			facade := facadeOf(t, *w)
			facade["building_ref"] = "b2"
			w.Properties[PropertyFacade] = facade
		}, CodeWindowFacadeUnresolved},
		"moved": {func(w *Feature) {
			facade := facadeOf(t, *w)
			facade["from"] = []any{facadeX0 + 0.002, facadeY0}
			w.Properties[PropertyFacade] = facade
		}, CodeWindowFacadeMoved},
		"outside its facade": {func(w *Feature) {
			w.Properties[PropertyWindow] = frameWith("u_m", 19.5)
		}, CodeWindowOutsideFacade},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, window := oneWindow(t)
			tc.mutate(&window)
			model.Features[1] = window

			report := Validate(model)
			if !report.Valid || len(report.Warnings) != 1 || report.Warnings[0].Code != tc.code ||
				report.Warnings[0].FeatureID != "w1" || !strings.Contains(report.Warnings[0].Message, `window "w1"`) {
				t.Fatalf("report = %#v, want one %s warning on w1", report, tc.code)
			}
		})
	}
}

// On a geographic project the window is placed, and checked, in the metric
// CRS a run computes in: its frame is metres, never degrees.
func TestWindowOnAGeographicProject(t *testing.T) {
	t.Parallel()

	projected := Model{ProjectCRS: "EPSG:25832", Features: []Feature{buildingFeature("b1", geoFacadeFootprint())}}

	model, err := Reproject(projected, "EPSG:4326")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	// Edge 0 of the canonical footprint runs from (x0 - 1, y0 + 12) to
	// (x0, y0), about 12.04 m.
	spec := bedroomSpec("w1")
	model.Features = append(model.Features, mustWindowFeature(t, model, spec))
	assertCleanReport(t, Validate(model))

	back, err := Reproject(model, "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject back: %v", err)
	}

	coords, _ := back.Features[1].Coordinates.([]any)
	x, _ := coords[0].(float64)
	y, _ := coords[1].(float64)
	// u = 2.6 along the edge from (x0 - 1, y0 + 12) towards (x0, y0).
	length := math.Hypot(1, 12)
	wantX, wantY := geoFacadeX0-1+2.6/length, geoFacadeY0+12-2.6*12/length

	if math.Hypot(x-wantX, y-wantY) > 0.01 {
		t.Errorf("centre = (%v, %v), want (%v, %v) within 1 cm", x, y, wantX, wantY)
	}

	placer, err := NewWindowPlacer(model)
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	spec.Frame.UM = 11.5
	if _, err := placer.Feature(spec); err == nil {
		t.Error("a window past the corner of the 12 m edge was placed")
	}
}

// A window re-attaches as a receiver does: by its stored endpoints, and back
// to proposed.
func TestReattachWindowAfterARenumbering(t *testing.T) {
	t.Parallel()

	model, window := oneWindow(t)

	// The stored edge index one off, as a CRS round trip that breaks a tie
	// leaves it; the endpoints still name edge 0.
	facade := facadeOf(t, window)
	facade["edge"] = 1.0
	window.Properties[PropertyFacade] = facade
	window.Properties[PropertyReviewStatus] = geo.ReviewStatusConfirmed
	model.Features[1] = window

	report := Validate(model)
	if len(issuesWithCode(report.Warnings, CodeWindowFacadeMoved)) != 1 {
		t.Fatalf("warnings = %#v, want the window moved", report.Warnings)
	}

	reattachments := mustPropose(t, model, FacadeMoveToleranceM)
	if len(reattachments) != 1 || reattachments[0].Outcome != ReattachProposed || reattachments[0].ReceiverID != "w1" {
		t.Fatalf("reattachments = %+v, want w1 proposed", reattachments)
	}

	fixed, applied := ApplyFacadeReattachments(model, reattachments)
	if applied != 1 {
		t.Fatalf("applied = %d, want 1", applied)
	}

	assertCleanReport(t, Validate(fixed))

	if status := fixed.Features[1].Properties[PropertyReviewStatus]; status != geo.ReviewStatusProposed {
		t.Errorf("review_status = %v, want proposed after a re-attach", status)
	}
}

// A window's Point is its centre on the facade, so re-attaching it to an
// edge that moved moves the Point with it: onto the new edge, at the frame's
// centre u. The frame itself stays as it was.
func TestReattachWindowMovesItsCentreOntoTheNewEdge(t *testing.T) {
	t.Parallel()

	model, window := oneWindow(t)
	model = withBuilding(model, "b1", [][][]geo.Point2D{{facadeRect(facadeX0+0.3, facadeY0+0.2, facadeX0+20.3, facadeY0+12.2)}})

	reattachments := mustPropose(t, model, 0.5)
	if len(reattachments) != 1 || reattachments[0].Outcome != ReattachProposed || reattachments[0].ReceiverID != "w1" {
		t.Fatalf("reattachments = %+v, want w1 proposed", reattachments)
	}

	fixed, applied := ApplyFacadeReattachments(model, reattachments)
	if applied != 1 {
		t.Fatalf("applied = %d, want 1", applied)
	}

	assertCleanReport(t, Validate(fixed))

	got, err := WindowOf(fixed.Features[1])
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	if got.Frame != bedroomSpec("").Frame {
		t.Errorf("frame = %+v, want %+v unchanged", got.Frame, bedroomSpec("").Frame)
	}

	coords, ok := fixed.Features[1].Coordinates.([]any)
	if !ok || len(coords) != 2 {
		t.Fatalf("coordinates = %#v, want a Point", fixed.Features[1].Coordinates)
	}

	x, _ := coords[0].(float64)
	y, _ := coords[1].(float64)
	centre := geo.Point2D{X: x, Y: y}
	want := geo.FacadePoint(got.Facade.From, got.Facade.To, got.Frame.CentreUM(), 0)

	if d := geo.Distance(centre, want); d > 1e-9 {
		t.Errorf("centre = %+v, want %+v on the new edge (%g m off)", centre, want, d)
	}

	if d := geo.DistancePointToSegment(centre, got.Facade.From, got.Facade.To); d > 1e-9 {
		t.Errorf("centre is %g m off the new edge", d)
	}

	if old, _ := window.Coordinates.([]any); slices.Equal(coords, old) {
		t.Error("the window Point stayed where the old edge put it")
	}
}
