package modelgeojson

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Synthetic coordinates around a UTM-like origin; no real address.
const facadeX0, facadeY0 = 500000.0, 5800000.0

// facadeRect is a closed counter-clockwise rectangle ring.
func facadeRect(x0, y0, x1, y1 float64) []geo.Point2D {
	return []geo.Point2D{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}, {X: x0, Y: y0}}
}

// rewriteRing returns a closed ring walked the other way and starting k
// vertices later: the same ring, written differently.
func rewriteRing(ring []geo.Point2D, k int) []geo.Point2D {
	open := slices.Clone(ring[:len(ring)-1])
	slices.Reverse(open)
	open = append(open[k:], open[:k]...)

	return append(open, open[0])
}

func ringCoordinates(ring []geo.Point2D) []any {
	out := make([]any, len(ring))
	for i, p := range ring {
		out[i] = []any{p.X, p.Y}
	}

	return out
}

func polygonCoordinates(rings [][]geo.Point2D) []any {
	out := make([]any, len(rings))
	for i, ring := range rings {
		out[i] = ringCoordinates(ring)
	}

	return out
}

// buildingFeature writes a footprint as a Polygon when it has one part and as
// a MultiPolygon otherwise.
func buildingFeature(id string, polygons [][][]geo.Point2D) Feature {
	height := 9.0
	feature := Feature{ID: id, Kind: FeatureKindBuilding, HeightM: &height}

	if len(polygons) == 1 {
		feature.GeometryType = GeometryTypePolygon
		feature.Coordinates = polygonCoordinates(polygons[0])

		return feature
	}

	parts := make([]any, len(polygons))
	for i, rings := range polygons {
		parts[i] = polygonCoordinates(rings)
	}

	feature.GeometryType = GeometryTypeMultiPolygon
	feature.Coordinates = parts

	return feature
}

// jsonValue round-trips v through JSON, so a property holds exactly what a
// model read from disk would hold.
func jsonValue(t *testing.T, v any) any {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out any

	err = json.Unmarshal(data, &out)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return out
}

// facadeReceiverFeatures generates a building's facade receivers with
// geo.FacadeReceiverSet and writes them as model features, the way a run's
// receiver set would reach the model.
func facadeReceiverFeatures(t *testing.T, building geo.FacadeBuilding, includeCourtyard bool) []Feature {
	t.Helper()

	set := geo.FacadeReceiverSet{
		ID: "fs", BuildingIDs: []string{building.ID}, PlacementRule: geo.PlacementRLS19Facade,
		SpacingM: 5, IncludeCourtyard: includeCourtyard,
	}

	receivers, err := set.Generate([]geo.FacadeBuilding{building})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	features := make([]Feature, len(receivers))
	for i, r := range receivers {
		height := r.HeightM
		features[i] = Feature{
			ID:           r.ID,
			Kind:         FeatureKindReceiver,
			HeightM:      &height,
			GeometryType: GeometryTypePoint,
			Coordinates:  []any{r.Point.X, r.Point.Y},
			Properties: map[string]any{
				PropertyPlacementRule:    r.PlacementRule,
				PropertyFacade:           jsonValue(t, r.Facade),
				PropertyStorey:           float64(r.Storey),
				PropertyStationUM:        r.StationUM,
				PropertySlabHeightSource: r.SlabHeightSource,
				PropertyReviewStatus:     r.ReviewStatus,
			},
		}
	}

	return features
}

func facadeTestBuilding(polygons [][][]geo.Point2D) geo.FacadeBuilding {
	return geo.FacadeBuilding{ID: "b1", Polygons: polygons, HeightM: 9, Levels: 3, HeightSource: geo.SlabHeightMeasured}
}

func simpleFacadeFootprint() [][][]geo.Point2D {
	return [][][]geo.Point2D{{facadeRect(facadeX0, facadeY0, facadeX0+20, facadeY0+12)}}
}

func facadeModel(features ...Feature) Model {
	return Model{ProjectCRS: "EPSG:25832", Features: features}
}

// oneFacadeReceiver is a model of one building and the first receiver
// generated on it, whose properties a test may then change.
func oneFacadeReceiver(t *testing.T) (Model, Feature) {
	t.Helper()

	footprint := simpleFacadeFootprint()
	receiver := facadeReceiverFeatures(t, facadeTestBuilding(footprint), false)[0]
	receiver.Properties = maps.Clone(receiver.Properties)

	return facadeModel(buildingFeature("b1", footprint), receiver), receiver
}

func facadeOf(t *testing.T, receiver Feature) map[string]any {
	t.Helper()

	facade, ok := receiver.Properties[PropertyFacade].(map[string]any)
	if !ok {
		t.Fatalf("facade = %#v, want an object", receiver.Properties[PropertyFacade])
	}

	return maps.Clone(facade)
}

func issuesWithCode(issues []ValidationIssue, code string) []ValidationIssue {
	var out []ValidationIssue

	for _, issue := range issues {
		if issue.Code == code {
			out = append(out, issue)
		}
	}

	return out
}

func assertCleanReport(t *testing.T, report ValidationReport) {
	t.Helper()

	if !report.Valid || len(report.Errors) != 0 || len(report.Warnings) != 0 {
		t.Fatalf("want a clean report, got valid=%v errors=%#v warnings=%#v", report.Valid, report.Errors, report.Warnings)
	}
}

// The receivers Generate places validate clean on the building they were
// placed on, exterior and courtyard, Polygon and MultiPolygon alike.
func TestValidateFacadeReceiversAsGenerated(t *testing.T) {
	t.Parallel()

	courtyard := [][][]geo.Point2D{
		{
			facadeRect(facadeX0, facadeY0, facadeX0+20, facadeY0+12),
			rewriteRing(facadeRect(facadeX0+2, facadeY0+2, facadeX0+6, facadeY0+5), 0),
			rewriteRing(facadeRect(facadeX0+10, facadeY0+3, facadeX0+16, facadeY0+9), 0),
		},
		{facadeRect(facadeX0+30, facadeY0, facadeX0+36, facadeY0+4)},
	}

	for name, footprint := range map[string][][][]geo.Point2D{
		"polygon":                simpleFacadeFootprint(),
		"multipolygon courtyard": courtyard,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			receivers := facadeReceiverFeatures(t, facadeTestBuilding(footprint), true)
			assertCleanReport(t, Validate(facadeModel(append([]Feature{buildingFeature("b1", footprint)}, receivers...)...)))
		})
	}
}

// A building written with another start vertex, the other orientation and
// its parts and holes in another order still matches the keys its receivers
// were placed with.
func TestValidateFacadeReceiversSurviveARewrittenBuilding(t *testing.T) {
	t.Parallel()

	outer := facadeRect(facadeX0, facadeY0, facadeX0+20, facadeY0+12)
	holeA := rewriteRing(facadeRect(facadeX0+2, facadeY0+2, facadeX0+6, facadeY0+5), 0)
	holeB := rewriteRing(facadeRect(facadeX0+10, facadeY0+3, facadeX0+16, facadeY0+9), 0)
	annex := facadeRect(facadeX0+30, facadeY0, facadeX0+36, facadeY0+4)

	placed := [][][]geo.Point2D{{outer, holeA, holeB}, {annex}}
	rewritten := [][][]geo.Point2D{
		{rewriteRing(annex, 2)},
		{rewriteRing(outer, 3), rewriteRing(holeB, 1), rewriteRing(holeA, 2)},
	}

	receivers := facadeReceiverFeatures(t, facadeTestBuilding(placed), true)
	assertCleanReport(t, Validate(facadeModel(append([]Feature{buildingFeature("b1", rewritten)}, receivers...)...)))
}

// The properties survive the import path: GeoJSON text through Normalize.
func TestValidateFacadeReceiverThroughNormalize(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "b1", "kind": "building", "height_m": 9},
      "geometry": {"type": "Polygon", "coordinates": [[[500000,5800000],[500000,5800012],[500020,5800012],[500020,5800000],[500000,5800000]]]}
    },
    {
      "type": "Feature",
      "properties": {
        "id": "fs/b1/p0r0e0/s0/f0", "kind": "receiver", "height_m": 3,
        "placement_rule": "rls19-facade",
        "facade": {"building_ref": "b1", "part": 0, "ring": 0, "edge": 0, "from": [500000, 5800000], "to": [500020, 5800000]},
        "storey": 0, "station_u_m": 2.5, "slab_height_source": "measured", "review_status": "confirmed"
      },
      "geometry": {"type": "Point", "coordinates": [500002.5, 5799999.95]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	assertCleanReport(t, Validate(model))
}

// A property written as null is absent, as an exporter of a nullable column
// writes it, so a plain receiver carrying one is checked as before.
func TestValidateFacadeNullPropertiesAreAbsent(t *testing.T) {
	t.Parallel()

	model, receiver := oneFacadeReceiver(t)
	for _, key := range []string{
		PropertyPlacementRule, PropertyFacade, PropertyStorey,
		PropertyStationUM, PropertySlabHeightSource, PropertyReviewStatus,
	} {
		receiver.Properties[key] = nil
	}

	model.Features[1] = receiver
	assertCleanReport(t, Validate(model))
}

func TestValidateFacadeReceiverInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		property string
		mutate   func(props, facade map[string]any)
	}{
		{"facade not an object", PropertyFacade, func(p, _ map[string]any) { p[PropertyFacade] = []any{} }},
		{"facade a string", PropertyFacade, func(p, _ map[string]any) { p[PropertyFacade] = "b1/0/0/0" }},
		{"building_ref missing", PropertyFacade, func(_, f map[string]any) { delete(f, "building_ref") }},
		{"building_ref not a string", PropertyFacade, func(_, f map[string]any) { f["building_ref"] = 7.0 }},
		{"building_ref empty", PropertyFacade, func(_, f map[string]any) { f["building_ref"] = "  " }},
		{"building_ref citygml fallback", PropertyFacade, func(_, f map[string]any) { f["building_ref"] = "citygml-building-007" }},
		{"part missing", PropertyFacade, func(_, f map[string]any) { delete(f, "part") }},
		{"part fractional", PropertyFacade, func(_, f map[string]any) { f["part"] = 0.5 }},
		{"ring negative", PropertyFacade, func(_, f map[string]any) { f["ring"] = -1.0 }},
		{"edge a string", PropertyFacade, func(_, f map[string]any) { f["edge"] = "0" }},
		{"edge huge", PropertyFacade, func(_, f map[string]any) { f["edge"] = 1e300 }},
		{"from missing", PropertyFacade, func(_, f map[string]any) { delete(f, "from") }},
		{"from one number", PropertyFacade, func(_, f map[string]any) { f["from"] = []any{facadeX0} }},
		{"from three numbers", PropertyFacade, func(_, f map[string]any) { f["from"] = []any{facadeX0, facadeY0, 0.0} }},
		{"to not numbers", PropertyFacade, func(_, f map[string]any) { f["to"] = []any{"x", "y"} }},
		{"to an object", PropertyFacade, func(_, f map[string]any) { f["to"] = map[string]any{"x": facadeX0, "y": facadeY0} }},
		{"placement_rule unknown", PropertyPlacementRule, func(p, _ map[string]any) { p[PropertyPlacementRule] = "rls19-window" }},
		{"placement_rule not a string", PropertyPlacementRule, func(p, _ map[string]any) { p[PropertyPlacementRule] = 1.0 }},
		{"storey negative", PropertyStorey, func(p, _ map[string]any) { p[PropertyStorey] = -1.0 }},
		{"storey fractional", PropertyStorey, func(p, _ map[string]any) { p[PropertyStorey] = 1.5 }},
		{"storey a string", PropertyStorey, func(p, _ map[string]any) { p[PropertyStorey] = "EG" }},
		{"station_u_m a string", PropertyStationUM, func(p, _ map[string]any) { p[PropertyStationUM] = "2.5" }},
		{"station_u_m negative", PropertyStationUM, func(p, _ map[string]any) { p[PropertyStationUM] = -0.5 }},
		{"slab_height_source unknown", PropertySlabHeightSource, func(p, _ map[string]any) { p[PropertySlabHeightSource] = "guessed" }},
		{"review_status unknown", PropertyReviewStatus, func(p, _ map[string]any) { p[PropertyReviewStatus] = "approved" }},
		{"review_status not a string", PropertyReviewStatus, func(p, _ map[string]any) { p[PropertyReviewStatus] = true }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			model, receiver := oneFacadeReceiver(t)
			facade := facadeOf(t, receiver)
			receiver.Properties[PropertyFacade] = facade
			tc.mutate(receiver.Properties, facade)
			model.Features[1] = receiver

			report := Validate(model)
			if report.Valid {
				t.Fatal("want an invalid model")
			}

			if len(report.Errors) != 1 || report.Errors[0].Code != CodeReceiverFacadeInvalid {
				t.Fatalf("errors = %#v, want exactly one %s", report.Errors, CodeReceiverFacadeInvalid)
			}

			issue := report.Errors[0]
			if issue.FeatureID != receiver.ID || !strings.Contains(issue.Message, receiver.ID) || !strings.Contains(issue.Message, tc.property) {
				t.Fatalf("issue = %#v, want it to name receiver %q and property %q", issue, receiver.ID, tc.property)
			}

			// A key that does not parse is not looked up as well.
			if len(report.Warnings) != 0 {
				t.Fatalf("warnings = %#v, want none beside the invalid key", report.Warnings)
			}
		})
	}
}

// One issue per bad property.
func TestValidateFacadeReceiverReportsEachBadProperty(t *testing.T) {
	t.Parallel()

	model, receiver := oneFacadeReceiver(t)
	receiver.Properties[PropertyStorey] = -1.0
	receiver.Properties[PropertyReviewStatus] = "approved"
	model.Features[1] = receiver

	report := Validate(model)
	if got := issuesWithCode(report.Errors, CodeReceiverFacadeInvalid); len(got) != 2 {
		t.Fatalf("invalid issues = %#v, want two", got)
	}
}

// Integer properties written by code rather than read from JSON are accepted.
func TestValidateFacadeReceiverAcceptsGoIntegers(t *testing.T) {
	t.Parallel()

	model, receiver := oneFacadeReceiver(t)
	facade := facadeOf(t, receiver)
	facade["part"], facade["ring"], facade["edge"] = 0, int64(0), 0
	receiver.Properties[PropertyFacade] = facade
	receiver.Properties[PropertyStorey] = 0
	model.Features[1] = receiver

	assertCleanReport(t, Validate(model))
}

func TestValidateFacadeReceiverUnresolved(t *testing.T) {
	t.Parallel()

	for name, ref := range map[string]string{
		"no such feature":       "b-gone",
		"names a receiver":      "rcv-other",
		"names a source":        "src-1",
		"differs by whitespace": " b1",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, receiver := oneFacadeReceiver(t)
			facade := facadeOf(t, receiver)
			facade["building_ref"] = ref
			receiver.Properties[PropertyFacade] = facade
			model.Features[1] = receiver

			height := 4.0
			model.Features = append(
				model.Features,
				Feature{ID: "rcv-other", Kind: FeatureKindReceiver, HeightM: &height, GeometryType: GeometryTypePoint, Coordinates: []any{facadeX0 - 5, facadeY0}},
				Feature{ID: "src-1", Kind: FeatureKindSource, SourceType: SourceTypePoint, GeometryType: GeometryTypePoint, Coordinates: []any{facadeX0 - 9, facadeY0}},
			)

			report := Validate(model)

			// A warning, not an error: an import of changed buildings succeeds.
			if !report.Valid || len(report.Errors) != 0 {
				t.Fatalf("errors = %#v, want none", report.Errors)
			}

			if len(report.Warnings) != 1 || report.Warnings[0].Code != CodeReceiverFacadeUnresolved {
				t.Fatalf("warnings = %#v, want exactly one %s", report.Warnings, CodeReceiverFacadeUnresolved)
			}

			issue := report.Warnings[0]
			if issue.FeatureID != receiver.ID || !strings.Contains(issue.Message, receiver.ID) || !strings.Contains(issue.Message, ref) {
				t.Fatalf("issue = %#v, want it to name receiver %q and building_ref %q", issue, receiver.ID, ref)
			}
		})
	}
}

func TestValidateFacadeReceiverMoved(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(facade map[string]any)
		moved  bool
	}{
		{"from moved 2 mm", func(f map[string]any) { f["from"] = []any{facadeX0 + 0.002, facadeY0} }, true},
		{"to moved 2 mm diagonally", func(f map[string]any) { f["to"] = []any{facadeX0 + 20 + 0.0015, facadeY0 + 0.0015} }, true},
		{"from moved 0.5 mm", func(f map[string]any) { f["from"] = []any{facadeX0 + 0.0005, facadeY0} }, false},
		{"to moved 0.5 mm", func(f map[string]any) { f["to"] = []any{facadeX0 + 20, facadeY0 - 0.0005} }, false},
		{"edge out of range", func(f map[string]any) { f["edge"] = 4.0 }, true},
		{"ring out of range", func(f map[string]any) { f["ring"] = 1.0 }, true},
		{"part out of range", func(f map[string]any) { f["part"] = 1.0 }, true},
		{"another edge's endpoints", func(f map[string]any) { f["edge"] = 1.0 }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			model, receiver := oneFacadeReceiver(t)
			facade := facadeOf(t, receiver)
			tc.mutate(facade)
			receiver.Properties[PropertyFacade] = facade
			model.Features[1] = receiver

			report := Validate(model)
			if !report.Valid || len(report.Errors) != 0 {
				t.Fatalf("errors = %#v, want none", report.Errors)
			}

			if !tc.moved {
				assertCleanReport(t, report)

				return
			}

			if len(report.Warnings) != 1 || report.Warnings[0].Code != CodeReceiverFacadeMoved {
				t.Fatalf("warnings = %#v, want exactly one %s", report.Warnings, CodeReceiverFacadeMoved)
			}

			issue := report.Warnings[0]
			if issue.FeatureID != receiver.ID || !strings.Contains(issue.Message, receiver.ID) || !strings.Contains(issue.Message, "b1") {
				t.Fatalf("issue = %#v, want it to name receiver %q and building b1", issue, receiver.ID)
			}
		})
	}
}

// A building whose footprint cannot be read has no edge to compare with: the
// building's own geometry error is reported, and the receiver is moved.
func TestValidateFacadeReceiverOnUnreadableBuilding(t *testing.T) {
	t.Parallel()

	model, receiver := oneFacadeReceiver(t)
	building := model.Features[0]
	building.Coordinates = []any{[]any{[]any{facadeX0, facadeY0}, []any{facadeX0 + 20, facadeY0}}}
	model.Features[0] = building

	report := Validate(model)

	if len(issuesWithCode(report.Errors, "geometry.polygon.invalid")) != 1 {
		t.Fatalf("errors = %#v, want the building's geometry error", report.Errors)
	}

	moved := issuesWithCode(report.Warnings, CodeReceiverFacadeMoved)
	if len(moved) != 1 || moved[0].FeatureID != receiver.ID {
		t.Fatalf("warnings = %#v, want %s on %q", report.Warnings, CodeReceiverFacadeMoved, receiver.ID)
	}
}
