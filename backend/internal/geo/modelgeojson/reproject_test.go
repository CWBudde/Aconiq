package modelgeojson_test

import (
	"math"
	"testing"

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
