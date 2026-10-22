package modelgeojson

import (
	"maps"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Synthetic coordinates in EPSG:25832, near lon 9.7, lat 52.3; no real
// address. The footprint is a quadrilateral whose vertices all differ in x,
// so a CRS round trip cannot change which vertex its canonical rings start
// at.
const geoFacadeX0, geoFacadeY0 = 547000.0, 5795000.0

func geoFacadeFootprint() [][][]geo.Point2D {
	x0, y0 := geoFacadeX0, geoFacadeY0

	return [][][]geo.Point2D{{{
		{X: x0, Y: y0}, {X: x0 + 20, Y: y0 + 1}, {X: x0 + 19, Y: y0 + 13}, {X: x0 - 1, Y: y0 + 12}, {X: x0, Y: y0},
	}}}
}

// geographicFacadeModel places receivers on a building in EPSG:25832, lets
// shift move the first receiver's stored facade from by dx metres east, and
// returns the model reprojected into crs, as a geographic project holds it,
// together with that receiver's id.
func geographicFacadeModel(t *testing.T, crs string, dx float64) (Model, string) {
	t.Helper()

	footprint := geoFacadeFootprint()
	receivers := facadeReceiverFeatures(t, facadeTestBuilding(footprint), false)

	first := receivers[0]
	first.Properties = maps.Clone(first.Properties)
	facade := facadeOf(t, first)

	from, ok := facade["from"].([]any)
	if !ok || len(from) != 2 {
		t.Fatalf("facade.from = %#v, want [x, y]", facade["from"])
	}

	x, _ := from[0].(float64)
	facade["from"] = []any{x + dx, from[1]}
	first.Properties[PropertyFacade] = facade
	receivers[0] = first

	projected := Model{ProjectCRS: "EPSG:25832", Features: append([]Feature{buildingFeature("b1", footprint)}, receivers...)}

	model, err := Reproject(projected, crs)
	if err != nil {
		t.Fatalf("Reproject into %s: %v", crs, err)
	}

	return model, first.ID
}

func movedReceiverIDs(report ValidationReport) []string {
	moved := issuesWithCode(report.Warnings, CodeReceiverFacadeMoved)

	ids := make([]string, 0, len(moved))
	for _, issue := range moved {
		ids = append(ids, issue.FeatureID)
	}

	return ids
}

// On a geographic project the 1 mm tolerance is metres, not degrees: the
// stored from / to are measured against the edge in the metric CRS aconiq run
// would compute in, so validation at import and the run's refusal agree.
func TestValidateFacadeMovedOnAGeographicProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		dx    float64
		moved bool
	}{
		{"as placed, after a CRS round trip", 0, false},
		{"from moved 0.5 mm", 0.0005, false},
		{"from moved 2 mm", 0.002, true},
		// 0.5 m is about 7e-6 degrees, which a comparison in degrees passes.
		{"from moved 0.5 m", 0.5, true},
	}

	for _, crs := range []string{"EPSG:4326", "EPSG:4258"} {
		for _, tc := range cases {
			t.Run(crs+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				model, receiverID := geographicFacadeModel(t, crs, tc.dx)
				report := Validate(model)

				// What `aconiq run` sees: the model in its compute CRS.
				compute, err := Reproject(model, "EPSG:25832")
				if err != nil {
					t.Fatalf("Reproject into the compute CRS: %v", err)
				}

				if got, want := movedReceiverIDs(report), movedReceiverIDs(Validate(compute)); !equalStrings(got, want) {
					t.Fatalf("moved receivers = %v in %s, %v in the compute CRS; validation and the run must agree", got, crs, want)
				}

				if !tc.moved {
					assertCleanReport(t, report)

					return
				}

				if !report.Valid || len(report.Warnings) != 1 || report.Warnings[0].Code != CodeReceiverFacadeMoved {
					t.Fatalf("report = valid %v, errors %#v, warnings %#v; want exactly one %s",
						report.Valid, report.Errors, report.Warnings, CodeReceiverFacadeMoved)
				}

				issue := report.Warnings[0]
				if issue.FeatureID != receiverID || !strings.Contains(issue.Message, "EPSG:25832") {
					t.Fatalf("issue = %#v, want it on %q and naming the CRS it measured in", issue, receiverID)
				}

				if tc.dx == 0.5 && !strings.Contains(issue.Message, "0.5000 m") {
					t.Fatalf("message = %q, want the offset in metres (0.5000 m)", issue.Message)
				}
			})
		}
	}
}

// degreeFacadeModel is a geographic model built in degrees directly, around
// (lon, lat): one building, a receiver keyed to its canonical edge 0, one
// keyed to a building that is not there, and one without a facade.
//
// The footprint is a trapezoid whose westernmost vertex is so by 13 m, so its
// canonical ring starts there in degrees and in UTM alike. An axis-aligned
// rectangle would not do: its two western vertices tie on x in degrees, and
// the grid convergence of the projection breaks the tie the other way, which
// moves edge 0.
func degreeFacadeModel(t *testing.T, lon, lat float64) Model {
	t.Helper()

	const d = 0.0002

	footprint := [][][]geo.Point2D{{{
		{X: lon, Y: lat}, {X: lon + 2*d, Y: lat}, {X: lon + 2*d, Y: lat + d}, {X: lon + d, Y: lat + d}, {X: lon, Y: lat},
	}}}

	from, to, ok := geo.FacadeEdge(footprint, 0, 0, 0)
	if !ok {
		t.Fatal("FacadeEdge: no edge 0")
	}

	height := 4.0
	receiver := func(id string, props map[string]any) Feature {
		return Feature{
			ID: id, Kind: FeatureKindReceiver, HeightM: &height, GeometryType: GeometryTypePoint,
			Coordinates: []any{lon + d/2, lat - d/10}, Properties: props,
		}
	}

	facade := func(ref string) map[string]any {
		return map[string]any{
			PropertyFacade: map[string]any{
				"building_ref": ref, "part": 0.0, "ring": 0.0, "edge": 0.0,
				"from": []any{from.X, from.Y}, "to": []any{to.X, to.Y},
			},
		}
	}

	return Model{ProjectCRS: "EPSG:4326", Features: []Feature{
		buildingFeature("b-deg", footprint),
		receiver("r-keyed", facade("b-deg")),
		receiver("r-orphan", facade("b-gone")),
		receiver("r-plain", nil),
	}}
}

// A geographic site with no metric CRS to measure in cannot be checked, and
// says so on each keyed receiver rather than passing it in degrees. A key
// naming no building is unresolved wherever the site is.
func TestValidateFacadeOnAGeographicSiteWithNoComputeCRS(t *testing.T) {
	t.Parallel()

	for name, site := range map[string][2]float64{
		"outside UTM zones 31-34": {-100, 40},
		"southern hemisphere":     {9.7, -30},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := Validate(degreeFacadeModel(t, site[0], site[1]))
			if !report.Valid || len(report.Errors) != 0 {
				t.Fatalf("errors = %#v, want none", report.Errors)
			}

			moved := issuesWithCode(report.Warnings, CodeReceiverFacadeMoved)
			if len(moved) != 1 || moved[0].FeatureID != "r-keyed" || !strings.Contains(moved[0].Message, "cannot be checked") {
				t.Fatalf("moved = %#v, want one on r-keyed that cannot be checked", moved)
			}

			unresolved := issuesWithCode(report.Warnings, CodeReceiverFacadeUnresolved)
			if len(unresolved) != 1 || unresolved[0].FeatureID != "r-orphan" {
				t.Fatalf("unresolved = %#v, want one on r-orphan", unresolved)
			}

			if len(report.Warnings) != 2 {
				t.Fatalf("warnings = %#v, want only those two", report.Warnings)
			}
		})
	}
}

// The same model at a site that has a compute CRS checks clean, so the
// finding above is the projection's and not the fixture's.
func TestValidateFacadeDegreeModelInsideTheComputeCRSRange(t *testing.T) {
	t.Parallel()

	report := Validate(degreeFacadeModel(t, 9.7, 52.3))
	if len(movedReceiverIDs(report)) != 0 {
		t.Fatalf("warnings = %#v, want no %s", report.Warnings, CodeReceiverFacadeMoved)
	}
}

// A CRS whose kind cannot be determined is compared in its stored units, as
// aconiq run leaves it unprojected too.
func TestValidateFacadeMovedInACRSOfUnknownKind(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		dx    float64
		moved bool
	}{
		"0.5 units": {0.0005, false},
		"2 units":   {0.002, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, receiver := oneFacadeReceiver(t)
			model.ProjectCRS = "WKT:LOCAL_CS[\"site grid\"]"
			facade := facadeOf(t, receiver)
			facade["from"] = []any{facadeX0 + tc.dx, facadeY0}
			receiver.Properties[PropertyFacade] = facade
			model.Features[1] = receiver

			report := Validate(model)
			if got := movedReceiverIDs(report); (len(got) == 1) != tc.moved {
				t.Fatalf("warnings = %#v, want moved=%v", report.Warnings, tc.moved)
			}

			for _, issue := range report.Warnings {
				if strings.Contains(issue.Message, "cannot be checked") {
					t.Fatalf("issue = %#v; a CRS of unknown kind is compared as stored, not refused", issue)
				}
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
