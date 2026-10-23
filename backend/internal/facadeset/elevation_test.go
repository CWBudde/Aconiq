package facadeset

import (
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// The house unfolds into its four facades in canonical order, with the
// storeys facade mode places at and each window on the facade it names: one
// that rises above the roof is drawn and says so, and one whose key no longer
// names its edge is listed apart rather than drawn on the wrong facade.
func TestElevation(t *testing.T) {
	t.Parallel()

	roof := windowSpec("attic", 5, true, geo.ReviewStatusConfirmed)
	roof.Frame.SillZM = 8.5

	model := windowModel(t, windowSpec("a-bed", 2, true, geo.ReviewStatusConfirmed), roof, windowSpec("moved", 7, false, ""))

	for i, f := range model.Features {
		if f.ID == "moved" {
			facade := maps.Clone(f.Properties[modelgeojson.PropertyFacade].(map[string]any))
			facade["edge"] = 2.0
			model.Features[i].Properties = maps.Clone(f.Properties)
			model.Features[i].Properties[modelgeojson.PropertyFacade] = facade
		}
	}

	elevation, err := Elevation(model, "house")
	if err != nil {
		t.Fatalf("Elevation: %v", err)
	}

	if elevation.HeightM != 9 || elevation.Levels != 3 || elevation.SlabHeightSource != geo.SlabHeightMeasured ||
		!slices.Equal(elevation.SlabHeightsM, []float64{3, 6, 9}) {
		t.Errorf("building = %+v, want 9 m in three measured storeys", elevation)
	}

	if len(elevation.Facades) != 4 {
		t.Fatalf("got %d facades, want 4", len(elevation.Facades))
	}

	south := elevation.Facades[0]
	if south.Edge != 0 || south.From != [2]float64{547000, 5795000} || south.To != [2]float64{547010, 5795000} ||
		math.Abs(south.LengthM-10) > 1e-9 || math.Abs(south.FacingDeg-180) > 1e-9 || south.Courtyard {
		t.Errorf("facade 0 = %+v, want the 10 m south edge", south)
	}

	lengths := make([]float64, 0, len(elevation.Facades))
	for _, f := range elevation.Facades {
		lengths = append(lengths, f.LengthM)
	}

	if !slices.Equal(lengths, []float64{10, 8, 10, 8}) {
		t.Errorf("lengths = %v, want 10, 8, 10, 8", lengths)
	}

	ids := map[string]ElevationWindow{}
	for _, w := range south.Windows {
		ids[w.ID] = w
	}

	if bed, ok := ids["a-bed"]; !ok || len(bed.Findings) != 0 || !bed.Sensitive || bed.Frame.UM != 2 {
		t.Errorf("a-bed = %+v, want it on the south facade with no findings", bed)
	}

	if attic, ok := ids["attic"]; !ok || len(attic.Findings) != 1 || attic.Findings[0].Code != modelgeojson.CodeWindowAboveBuilding {
		t.Errorf("attic = %+v, want it on the south facade, above the building", attic)
	}

	if len(elevation.Detached) != 1 || elevation.Detached[0].ID != "moved" ||
		elevation.Detached[0].Findings[0].Code != modelgeojson.CodeWindowFacadeMoved {
		t.Errorf("detached = %+v, want the moved window", elevation.Detached)
	}

	for _, f := range elevation.Facades[1:] {
		if len(f.Windows) != 0 {
			t.Errorf("facade %d carries %+v, want no window", f.Edge, f.Windows)
		}
	}
}

func TestElevationRefuses(t *testing.T) {
	t.Parallel()

	model := windowModel(t)

	if _, err := Elevation(model, "shed"); err == nil || !strings.Contains(err.Error(), `"shed" is not in the model`) {
		t.Errorf("unknown building: err = %v", err)
	}

	model.Features[0].HeightM = nil
	if _, err := Elevation(model, "house"); err == nil || !strings.Contains(err.Error(), "height_m") {
		t.Errorf("no height: err = %v", err)
	}
}
