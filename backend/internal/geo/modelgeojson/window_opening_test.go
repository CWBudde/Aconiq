package modelgeojson

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// openingBaseZ is the ground under b1 in the openings below, a height above
// the datum as a CityGML document states it.
const openingBaseZ = 50.0

// rectangleOutline is an opening's outline: a vertical rectangle from (x0, y0)
// to (x1, y1) on the ground plan, from z0 to z1 above the datum.
func rectangleOutline(x0, y0, x1, y1, z0, z1 float64) [][3]float64 {
	return [][3]float64{{x0, y0, z0}, {x1, y1, z0}, {x1, y1, z1}, {x0, y0, z1}, {x0, y0, z0}}
}

func openingPlacer(t *testing.T) WindowPlacer {
	t.Helper()

	placer, err := NewWindowPlacer(facadeModel(buildingFeature("b1", simpleFacadeFootprint())))
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	return placer
}

// A LoD3 window recessed 0.2 m into the south facade lands on the facade's
// edge with the frame bedroomSpec states by hand: the same key, the same
// rectangle, as a proposed window that is not sensitive until a person says
// so.
func TestOpeningFeatureOnItsFacade(t *testing.T) {
	t.Parallel()

	placer := openingPlacer(t)
	evidence := []any{map[string]any{"kind": "citygml-lod3", "source_id": "win-1"}}

	feature, err := placer.OpeningFeature(OpeningSpec{
		ID: "win-1", BuildingRef: "b1",
		Vertices: rectangleOutline(facadeX0+2, facadeY0+0.2, facadeX0+3.2, facadeY0+0.2, openingBaseZ+3.9, openingBaseZ+5.4),
		BaseZ:    openingBaseZ,
		Evidence: evidence,
	})
	if err != nil {
		t.Fatalf("OpeningFeature: %v", err)
	}

	got, err := WindowOf(feature)
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	want, err := WindowOf(mustWindowFeature(t, facadeModel(buildingFeature("b1", simpleFacadeFootprint())), bedroomSpec("win-1")))
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	if got.Facade != want.Facade || !framesClose(got.Frame, want.Frame) {
		t.Errorf("window = %+v, want the key and frame %+v", got, want)
	}

	if got.Sensitive || got.ReviewStatus != geo.ReviewStatusProposed {
		t.Errorf("window = %+v, want proposed and not sensitive", got)
	}

	if ev, ok := feature.Properties[PropertyEvidence].([]any); !ok || len(ev) != 1 {
		t.Errorf("evidence = %#v, want the opening's", feature.Properties[PropertyEvidence])
	}
}

// An outline that overhangs both ends of its edge by less than the fit
// tolerance is clamped to the edge at both, so it fits the edge it was put on.
func TestOpeningFeatureClampsToItsEdge(t *testing.T) {
	t.Parallel()

	overhang := geo.FacadeWindowFitToleranceM / 2

	feature, err := openingPlacer(t).OpeningFeature(OpeningSpec{
		ID: "wide", BuildingRef: "b1",
		Vertices: rectangleOutline(facadeX0-overhang, facadeY0, facadeX0+20+overhang, facadeY0, openingBaseZ+1, openingBaseZ+2),
		BaseZ:    openingBaseZ,
	})
	if err != nil {
		t.Fatalf("OpeningFeature: %v", err)
	}

	got, err := WindowOf(feature)
	if err != nil || got.Frame.UM != 0 || math.Abs(got.Frame.WidthM-20) > 1e-9 {
		t.Errorf("window = %+v, %v, want the whole 20 m edge", got, err)
	}
}

// On the east facade the frame is measured along that edge, from whichever
// end the canonical footprint starts it at.
func TestOpeningFeatureOnASideFacade(t *testing.T) {
	t.Parallel()

	feature, err := openingPlacer(t).OpeningFeature(OpeningSpec{
		ID: "win-east", BuildingRef: "b1",
		Vertices: rectangleOutline(facadeX0+20, facadeY0+3, facadeX0+20, facadeY0+4, openingBaseZ+1, openingBaseZ+2.2),
		BaseZ:    openingBaseZ,
	})
	if err != nil {
		t.Fatalf("OpeningFeature: %v", err)
	}

	got, err := WindowOf(feature)
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	south, north := geo.Point2D{X: facadeX0 + 20, Y: facadeY0}, geo.Point2D{X: facadeX0 + 20, Y: facadeY0 + 12}

	var wantU float64

	switch {
	case got.Facade.From == south && got.Facade.To == north:
		wantU = 3
	case got.Facade.From == north && got.Facade.To == south:
		wantU = 8
	default:
		t.Fatalf("facade = %+v, want the east edge", got.Facade)
	}

	want := geo.FacadeWindow{UM: wantU, SillZM: 1, WidthM: 1, HeightM: 1.2}
	if !framesClose(got.Frame, want) {
		t.Errorf("frame = %+v, want %+v", got.Frame, want)
	}
}

func TestOpeningFeatureRefuses(t *testing.T) {
	t.Parallel()

	placer := openingPlacer(t)
	south := func(x0, x1, y float64) [][3]float64 {
		return rectangleOutline(x0, y, x1, y, openingBaseZ+1, openingBaseZ+2)
	}

	cases := map[string]struct {
		spec OpeningSpec
		want string
	}{
		"no building": {OpeningSpec{ID: "w", BuildingRef: "b2", Vertices: south(facadeX0+2, facadeX0+3, facadeY0)}, "not in the model"},
		"no outline":  {OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: south(facadeX0+2, facadeX0+3, facadeY0)[:2]}, "outline"},
		"off every facade": {
			OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: south(facadeX0+2, facadeX0+3, facadeY0+3)},
			"no facade plane",
		},
		"past the corner": {
			OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: south(facadeX0+19.5, facadeX0+20.7, facadeY0)},
			"no facade plane",
		},
		"across two facades": {
			OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: [][3]float64{
				{facadeX0 + 19.8, facadeY0, 51}, {facadeX0 + 20, facadeY0 + 0.2, 51}, {facadeX0 + 20, facadeY0 + 0.2, 52},
			}},
			"plane of 2 facades",
		},
		"below the ground": {
			OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: rectangleOutline(facadeX0+2, facadeY0, facadeX0+3, facadeY0, openingBaseZ-0.5, openingBaseZ+0.5)},
			"below its building's base",
		},
		"not finite": {
			OpeningSpec{ID: "w", BuildingRef: "b1", Vertices: [][3]float64{{facadeX0 + 2, facadeY0, math.NaN()}, {facadeX0 + 3, facadeY0, 51}, {facadeX0 + 3, facadeY0, 52}}},
			"finite",
		},
	}

	for name, tc := range cases {
		tc.spec.BaseZ = openingBaseZ

		_, err := placer.OpeningFeature(tc.spec)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), `"w"`) {
			t.Errorf("%s: err = %v, want it to name the opening and mention %q", name, err, tc.want)
		}
	}
}

func framesClose(a, b geo.FacadeWindow) bool {
	const eps = 1e-6

	return math.Abs(a.UM-b.UM) < eps && math.Abs(a.SillZM-b.SillZM) < eps &&
		math.Abs(a.WidthM-b.WidthM) < eps && math.Abs(a.HeightM-b.HeightM) < eps
}

// An outline read in another CRS moves with the footprints it was read with,
// and keeps its heights.
func TestOpeningsInProjectCRS(t *testing.T) {
	t.Parallel()

	specs := []OpeningSpec{{ID: "w", Vertices: [][3]float64{{9.7320, 52.3759, 57.5}}}}

	same, err := OpeningsInProjectCRS(specs, "EPSG:25832", "EPSG:25832")
	if err != nil || &same[0] != &specs[0] {
		t.Fatalf("same CRS = %v, %v, want the specs as given", same, err)
	}

	moved, err := OpeningsInProjectCRS(specs, "EPSG:25832", "EPSG:4326")
	if err != nil {
		t.Fatalf("OpeningsInProjectCRS: %v", err)
	}

	v := moved[0].Vertices[0]
	if math.Abs(v[0]-549000) > 2000 || math.Abs(v[1]-5803000) > 2000 || v[2] != 57.5 || specs[0].Vertices[0][0] != 9.7320 {
		t.Errorf("moved = %v, want Hanover in UTM 32 at the same height, and the input left alone", v)
	}
}
