package modelgeojson_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func geographicModel() modelgeojson.Model {
	height := 12.0

	return modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:4326",
		Features: []modelgeojson.Feature{
			{
				ID:           "src-1",
				Kind:         modelgeojson.FeatureKindSource,
				SourceType:   "point",
				GeometryType: modelgeojson.GeometryTypePoint,
				Coordinates:  []any{10.0, 53.55},
				Properties:   map[string]any{"carried": "kept"},
			},
			{
				ID:           "bld-1",
				Kind:         modelgeojson.FeatureKindBuilding,
				HeightM:      &height,
				GeometryType: modelgeojson.GeometryTypePolygon,
				Coordinates: []any{[]any{
					[]any{10.0, 53.55, 31.5},
					[]any{10.001, 53.55, 31.5},
					[]any{10.001, 53.551, 31.5},
					[]any{10.0, 53.55, 31.5},
				}},
			},
		},
	}
}

func TestReprojectMovesXAndYIntoTheTargetCRS(t *testing.T) {
	t.Parallel()

	out, err := modelgeojson.Reproject(geographicModel(), "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	if out.ProjectCRS != "EPSG:25832" {
		t.Fatalf("ProjectCRS = %q, want EPSG:25832; a model that does not restate its CRS lies about its own coordinates", out.ProjectCRS)
	}

	point, ok := out.Features[0].Coordinates.([]any)
	if !ok || len(point) != 2 {
		t.Fatalf("source coordinates = %#v, want a 2-element pair", out.Features[0].Coordinates)
	}

	x, _ := point[0].(float64)
	y, _ := point[1].(float64)

	// Hamburg in EPSG:25832 is roughly 566 km easting, 5 934 km northing.
	// The assertion is loose on purpose: it pins the order of magnitude, which
	// is what catches a CRS mix-up, and leaves the metre-level accuracy of the
	// projection itself to internal/geo's own transform tests.
	if x < 560000 || x > 572000 || y < 5928000 || y > 5940000 {
		t.Fatalf("(10.0, 53.55) projected to (%.1f, %.1f), which is not in UTM zone 32 over Hamburg", x, y)
	}
}

// z is an absolute elevation in metres and means the same thing in both CRS,
// so a reprojection that touched it would silently move every building roof.
func TestReprojectLeavesElevationAlone(t *testing.T) {
	t.Parallel()

	out, err := modelgeojson.Reproject(geographicModel(), "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	rings, _ := out.Features[1].Coordinates.([]any)
	ring, _ := rings[0].([]any)

	for i, raw := range ring {
		vertex, ok := raw.([]any)
		if !ok || len(vertex) != 3 {
			t.Fatalf("vertex %d = %#v, want a 3-element coordinate", i, raw)
		}

		z, _ := vertex[2].(float64)
		if z != 31.5 {
			t.Fatalf("vertex %d elevation = %v, want 31.5", i, z)
		}
	}
}

func TestReprojectKeepsEverythingThatIsNotACoordinate(t *testing.T) {
	t.Parallel()

	in := geographicModel()

	out, err := modelgeojson.Reproject(in, "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	if len(out.Features) != len(in.Features) {
		t.Fatalf("feature count %d, want %d", len(out.Features), len(in.Features))
	}

	if out.Features[0].ID != "src-1" || out.Features[0].Kind != modelgeojson.FeatureKindSource {
		t.Fatalf("feature identity changed: %+v", out.Features[0])
	}

	if out.Features[0].Properties["carried"] != "kept" {
		t.Fatalf("properties were dropped: %#v", out.Features[0].Properties)
	}

	if out.Features[1].HeightM == nil || *out.Features[1].HeightM != 12.0 {
		t.Fatalf("height_m was dropped or changed: %#v", out.Features[1].HeightM)
	}
}

func TestReprojectIntoTheSameCRSIsANoOp(t *testing.T) {
	t.Parallel()

	in := geographicModel()

	out, err := modelgeojson.Reproject(in, "epsg:4326")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	point, _ := out.Features[0].Coordinates.([]any)

	x, _ := point[0].(float64)
	if x != 10.0 {
		t.Fatalf("x = %v after reprojecting into the same CRS, want 10.0", x)
	}
}

func TestReprojectRefusesWhatItCannotResolve(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		model  modelgeojson.Model
		target string
	}{
		{"no target", geographicModel(), "  "},
		{"model has no CRS", modelgeojson.Model{}, "EPSG:25832"},
		{"unparseable target", geographicModel(), "not-a-crs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := modelgeojson.Reproject(tc.model, tc.target)
			if err == nil {
				t.Fatal("Reproject returned no error")
			}
		})
	}
}

func TestBoundsSpansEveryCoordinate(t *testing.T) {
	t.Parallel()

	bbox, ok := geographicModel().Bounds()
	if !ok {
		t.Fatal("Bounds reported no extent for a model that has coordinates")
	}

	if math.Abs(bbox.MinX-10.0) > 1e-12 || math.Abs(bbox.MaxX-10.001) > 1e-12 {
		t.Fatalf("x span [%v, %v], want [10, 10.001]", bbox.MinX, bbox.MaxX)
	}

	if math.Abs(bbox.MinY-53.55) > 1e-12 || math.Abs(bbox.MaxY-53.551) > 1e-12 {
		t.Fatalf("y span [%v, %v], want [53.55, 53.551]", bbox.MinY, bbox.MaxY)
	}
}

func TestBoundsReportsAnEmptyModelAsHavingNoExtent(t *testing.T) {
	t.Parallel()

	_, ok := modelgeojson.Model{ProjectCRS: "EPSG:4326"}.Bounds()
	if ok {
		t.Fatal("an empty model reported an extent; a zero bbox at (0,0) is a place, not an absence")
	}
}

// modelWithEmbeddedGeometry carries the two vocabularies that attach
// coordinates to a feature through its properties rather than its geometry.
func modelWithEmbeddedGeometry() modelgeojson.Model {
	return modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:4326",
		Features: []modelgeojson.Feature{{
			ID:           "road-1",
			Kind:         modelgeojson.FeatureKindSource,
			SourceType:   "line",
			GeometryType: modelgeojson.GeometryTypeLineString,
			Coordinates:  []any{[]any{10.0, 53.55}, []any{10.001, 53.55}},
			Properties: map[string]any{
				"rls19_directional_sources": []any{
					map[string]any{
						"centerline":            []any{[]any{10.0, 53.55}, []any{10.001, 53.55}},
						"centerline_elevations": []any{12.0, 12.5},
					},
					map[string]any{
						"coordinates": []any{[]any{10.0, 53.551}, []any{10.001, 53.551}},
					},
				},
				"schall03_track_features": []any{
					map[string]any{"kind": "haltestelle", "x": 10.0, "y": 53.55},
				},
			},
		}},
	}
}

// Coordinates in properties are consumed as geometry by the RLS-19 and
// Schall 03 extractors, so leaving them in degrees while the feature geometry
// moves into metres would place directional sources millions of metres from
// their own receivers and push track features past their 25 m proximity check.
func TestReprojectMovesCoordinatesEmbeddedInProperties(t *testing.T) {
	t.Parallel()

	out, err := modelgeojson.Reproject(modelWithEmbeddedGeometry(), "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	props := out.Features[0].Properties

	sources, _ := props["rls19_directional_sources"].([]any)
	if len(sources) != 2 {
		t.Fatalf("rls19_directional_sources = %#v", props["rls19_directional_sources"])
	}

	for i, member := range []string{"centerline", "coordinates"} {
		entry, _ := sources[i].(map[string]any)

		line, _ := entry[member].([]any)
		if len(line) != 2 {
			t.Fatalf("source %d %s = %#v", i, member, entry[member])
		}

		first, _ := line[0].([]any)

		x, _ := first[0].(float64)
		if x < 560000 || x > 572000 {
			t.Fatalf("source %d %s still starts at x=%v; it was not projected", i, member, x)
		}
	}

	// Elevations are metres in both CRS and must not be touched.
	firstSource, _ := sources[0].(map[string]any)

	elevations, _ := firstSource["centerline_elevations"].([]any)
	if len(elevations) != 2 || elevations[0] != 12.0 || elevations[1] != 12.5 {
		t.Fatalf("centerline_elevations = %#v, want [12, 12.5]", firstSource["centerline_elevations"])
	}

	features, _ := props["schall03_track_features"].([]any)
	track, _ := features[0].(map[string]any)

	trackX, _ := track["x"].(float64)
	trackY, _ := track["y"].(float64)

	if trackX < 560000 || trackX > 572000 || trackY < 5928000 || trackY > 5940000 {
		t.Fatalf("track feature sits at (%v, %v); it was not projected", trackX, trackY)
	}

	if track["kind"] != "haltestelle" {
		t.Fatalf("track feature lost its kind: %#v", track)
	}
}

// Reproject must not write through the model it is handed: the caller still
// holds the loaded model, and a run that reprojects in place would leave the
// stored model's properties silently rewritten.
func TestReprojectLeavesTheInputModelAlone(t *testing.T) {
	t.Parallel()

	in := modelWithEmbeddedGeometry()

	_, err := modelgeojson.Reproject(in, "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	sources, _ := in.Features[0].Properties["rls19_directional_sources"].([]any)
	entry, _ := sources[0].(map[string]any)
	line, _ := entry["centerline"].([]any)
	first, _ := line[0].([]any)

	x, _ := first[0].(float64)
	if x != 10.0 {
		t.Fatalf("the input model's centerline moved to x=%v; Reproject wrote through its argument", x)
	}

	features, _ := in.Features[0].Properties["schall03_track_features"].([]any)
	track, _ := features[0].(map[string]any)

	if track["x"] != 10.0 {
		t.Fatalf("the input model's track feature moved to x=%v; Reproject wrote through its argument", track["x"])
	}
}

// modelWithFacadeReceiver carries a facade receiver: its facade property is a
// single object, not an array of them, holding the endpoints of the building
// edge the receiver was placed on. The receiver stands on that edge's first
// endpoint, so facade.from and the receiver point must stay identical.
func modelWithFacadeReceiver() modelgeojson.Model {
	height := 4.0

	return modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:4326",
		Features: []modelgeojson.Feature{{
			ID:           "rcv-1",
			Kind:         modelgeojson.FeatureKindReceiver,
			HeightM:      &height,
			GeometryType: modelgeojson.GeometryTypePoint,
			Coordinates:  []any{10.0, 53.55},
			Properties: map[string]any{
				"facade": map[string]any{
					"building_ref": "osm-way-123456",
					"part":         0.0,
					"ring":         0.0,
					"edge":         3.0,
					"from":         []any{10.0, 53.55},
					"to":           []any{10.0001, 53.55002},
				},
			},
		}},
	}
}

// project transforms one coordinate through the pipeline Reproject builds, so
// a test can say exactly where a coordinate should land rather than roughly.
func project(t *testing.T, sourceCRS, targetCRS string, x, y float64) geo.Point2D {
	t.Helper()

	from, err := geo.ParseCRS(sourceCRS)
	if err != nil {
		t.Fatalf("ParseCRS(%q): %v", sourceCRS, err)
	}

	to, err := geo.ParseCRS(targetCRS)
	if err != nil {
		t.Fatalf("ParseCRS(%q): %v", targetCRS, err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("BuildTransformPipeline: %v", err)
	}

	moved, err := pipeline.ApplyPoint(geo.Point2D{X: x, Y: y})
	if err != nil {
		t.Fatalf("ApplyPoint: %v", err)
	}

	return moved
}

func coordinatePair(t *testing.T, label string, raw any) (float64, float64) {
	t.Helper()

	pair, ok := raw.([]any)
	if !ok || len(pair) != 2 {
		t.Fatalf("%s = %#v, want an [x, y] pair", label, raw)
	}

	x, xOK := pair[0].(float64)
	y, yOK := pair[1].(float64)

	if !xOK || !yOK {
		t.Fatalf("%s = %#v, want two numbers", label, raw)
	}

	return x, y
}

// The facade endpoints are what the import re-check compares against the
// building edge. Left in the old CRS while the receiver moves, every
// re-import would report the receiver as having moved off its facade.
func TestReprojectMovesFacadeEndpointsWithTheReceiver(t *testing.T) {
	t.Parallel()

	out, err := modelgeojson.Reproject(modelWithFacadeReceiver(), "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	receiver := out.Features[0]
	receiverX, receiverY := coordinatePair(t, "receiver point", receiver.Coordinates)

	facade, ok := receiver.Properties["facade"].(map[string]any)
	if !ok {
		t.Fatalf("facade = %#v, want an object", receiver.Properties["facade"])
	}

	fromX, fromY := coordinatePair(t, "facade.from", facade["from"])
	if fromX != receiverX || fromY != receiverY {
		t.Fatalf("facade.from = (%v, %v), receiver = (%v, %v); the two started on the same coordinate and must move together",
			fromX, fromY, receiverX, receiverY)
	}

	wantTo := project(t, "EPSG:4326", "EPSG:25832", 10.0001, 53.55002)

	toX, toY := coordinatePair(t, "facade.to", facade["to"])
	if toX != wantTo.X || toY != wantTo.Y {
		t.Fatalf("facade.to = (%v, %v), want (%v, %v)", toX, toY, wantTo.X, wantTo.Y)
	}

	// The edge address is not a coordinate and must not be touched.
	for key, want := range map[string]any{"building_ref": "osm-way-123456", "part": 0.0, "ring": 0.0, "edge": 3.0} {
		if facade[key] != want {
			t.Fatalf("facade.%s = %#v, want %#v", key, facade[key], want)
		}
	}
}

func TestReprojectLeavesTheInputFacadeAlone(t *testing.T) {
	t.Parallel()

	in := modelWithFacadeReceiver()

	_, err := modelgeojson.Reproject(in, "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	facade, _ := in.Features[0].Properties["facade"].(map[string]any)

	fromX, _ := coordinatePair(t, "input facade.from", facade["from"])
	if fromX != 10.0 {
		t.Fatalf("the input model's facade.from moved to x=%v; Reproject wrote through its argument", fromX)
	}

	toX, _ := coordinatePair(t, "input facade.to", facade["to"])
	if toX != 10.0001 {
		t.Fatalf("the input model's facade.to moved to x=%v; Reproject wrote through its argument", toX)
	}
}

// A facade of the wrong shape is treated like a malformed directional source:
// carried through untouched for validation to report, not refused here and
// not repaired here. A well-formed member beside a malformed one still moves.
func TestReprojectCarriesAMalformedFacadeThrough(t *testing.T) {
	t.Parallel()

	wantTo := project(t, "EPSG:4326", "EPSG:25832", 10.0001, 53.55002)

	cases := []struct {
		name   string
		facade any
		// check inspects the reprojected facade value.
		check func(t *testing.T, got any)
	}{
		{
			name:   "an array of objects is not the singleton form",
			facade: []any{map[string]any{"from": []any{10.0, 53.55}, "to": []any{10.0001, 53.55002}}},
			check: func(t *testing.T, got any) {
				t.Helper()

				entries, _ := got.([]any)
				entry, _ := entries[0].(map[string]any)

				x, _ := coordinatePair(t, "facade[0].from", entry["from"])
				if x != 10.0 {
					t.Fatalf("facade[0].from moved to x=%v; an array-shaped facade should be carried through", x)
				}
			},
		},
		{
			name:   "from is not a pair",
			facade: map[string]any{"from": []any{10.0}, "to": []any{10.0001, 53.55002}},
			check: func(t *testing.T, got any) {
				t.Helper()

				facade, _ := got.(map[string]any)

				from, _ := facade["from"].([]any)
				if len(from) != 1 || from[0] != 10.0 {
					t.Fatalf("facade.from = %#v, want [10] carried through", facade["from"])
				}

				toX, toY := coordinatePair(t, "facade.to", facade["to"])
				if toX != wantTo.X || toY != wantTo.Y {
					t.Fatalf("facade.to = (%v, %v), want (%v, %v)", toX, toY, wantTo.X, wantTo.Y)
				}
			},
		},
		{
			name:   "to is not numeric",
			facade: map[string]any{"from": []any{10.0, 53.55}, "to": "edge-end"},
			check: func(t *testing.T, got any) {
				t.Helper()

				facade, _ := got.(map[string]any)
				if facade["to"] != "edge-end" {
					t.Fatalf("facade.to = %#v, want the string carried through", facade["to"])
				}

				fromX, _ := coordinatePair(t, "facade.from", facade["from"])
				if fromX < 560000 || fromX > 572000 {
					t.Fatalf("facade.from still at x=%v; it was not projected", fromX)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			in := modelWithFacadeReceiver()
			in.Features[0].Properties = map[string]any{"facade": tc.facade}

			out, err := modelgeojson.Reproject(in, "EPSG:25832")
			if err != nil {
				t.Fatalf("Reproject: %v", err)
			}

			tc.check(t, out.Features[0].Properties["facade"])
		})
	}
}
