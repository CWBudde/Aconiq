package cli

import (
	"fmt"
	"math"
	"strings"
	"testing"

	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// parkingFeature renders one area source feature with the given extra
// properties, over a 20 m x 10 m rectangle centred on (10, 5).
func parkingFeature(id, properties string) string {
	return fmt.Sprintf(`{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {"id": %q, "kind": "source", "source_type": "area"%s},
	      "geometry": {
	        "type": "Polygon",
	        "coordinates": [[[0,0],[20,0],[20,10],[0,10],[0,0]]]
	      }
	    }
	  ]
	}`, id, properties)
}

const parkingComplete = `,
	        "rls19_parking_num_spaces": 40,
	        "rls19_parking_type": "lkw-omnibus",
	        "rls19_parking_movements_per_space_day": 1.5,
	        "rls19_parking_movements_per_space_night": 0.8`

func TestExtractRLS19ParkingSources(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, parkingFeature("lot-1", parkingComplete))

	sources, extent, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 parking source, got %d", len(sources))
	}

	source := sources[0]

	if source.ID != "lot-1" {
		t.Errorf("id = %q, want lot-1", source.ID)
	}

	// Centroid and area come from the polygon, so neither has to be asserted
	// by hand in the model.
	if math.Abs(source.Center.X-10) > 1e-9 || math.Abs(source.Center.Y-5) > 1e-9 {
		t.Errorf("center = %+v, want (10, 5)", source.Center)
	}

	if math.Abs(source.AreaM2-200) > 1e-9 {
		t.Errorf("area_m2 = %g, want 200", source.AreaM2)
	}

	if source.NumSpaces != 40 {
		t.Errorf("num_spaces = %d, want 40", source.NumSpaces)
	}

	if source.LotType != rls19road.ParkingLotLkwOmnibus {
		t.Errorf("parking_type = %q, want lkw-omnibus", source.LotType)
	}

	if source.MovementsPerSpaceDay == nil || *source.MovementsPerSpaceDay != 1.5 {
		t.Errorf("day rate = %v, want 1.5", source.MovementsPerSpaceDay)
	}

	// The grid extent must be the footprint, not the centroid.
	if len(extent) != 5 {
		t.Errorf("extent has %d points, want the 5 polygon vertices", len(extent))
	}
}

// TestExtractRLS19ParkingSourcesAreaSubtractsHoles pins that the area comes
// from the whole polygon rather than its outer ring alone.
func TestExtractRLS19ParkingSourcesAreaSubtractsHoles(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {
	        "id": "lot-hole", "kind": "source", "source_type": "area",
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1
	      },
	      "geometry": {
	        "type": "Polygon",
	        "coordinates": [
	          [[0,0],[20,0],[20,10],[0,10],[0,0]],
	          [[5,2],[7,2],[7,4],[5,4],[5,2]]
	        ]
	      }
	    }
	  ]
	}`)

	sources, _, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if math.Abs(sources[0].AreaM2-196) > 1e-9 {
		t.Errorf("area_m2 = %g, want 200 - 4 = 196", sources[0].AreaM2)
	}
}

// TestExtractRLS19ParkingSourcesFacilityDefaults is the first non-test caller
// of DefaultMovementsPerHour, and so the thing that makes Tabelle 7 reachable.
func TestExtractRLS19ParkingSourcesFacilityDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		facility  string
		wantDay   float64
		wantNight float64
	}{
		{facility: "park-and-ride", wantDay: 0.3, wantNight: 0.06},
		{facility: "tank-rastanlage", wantDay: 1.5, wantNight: 0.8},
	}

	for _, tt := range tests {
		t.Run(tt.facility, func(t *testing.T) {
			t.Parallel()

			model := mustNormalizeModel(t, parkingFeature("lot", fmt.Sprintf(`,
	        "rls19_parking_num_spaces": 100,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_facility_type": %q`, tt.facility)))

			sources, _, err := extractRLS19ParkingSources(model)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}

			if got := *sources[0].MovementsPerSpaceDay; math.Abs(got-tt.wantDay) > 1e-9 {
				t.Errorf("day rate = %g, want %g", got, tt.wantDay)
			}

			if got := *sources[0].MovementsPerSpaceNight; math.Abs(got-tt.wantNight) > 1e-9 {
				t.Errorf("night rate = %g, want %g", got, tt.wantNight)
			}
		})
	}
}

// TestExtractRLS19ParkingSourcesExplicitRateOverridesItsOwnPeriod pins the
// precedence: a facility type seeds both periods, and an explicit rate replaces
// only the one it names.
func TestExtractRLS19ParkingSourcesExplicitRateOverridesItsOwnPeriod(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, parkingFeature("lot", `,
	        "rls19_parking_num_spaces": 100,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_facility_type": "park-and-ride",
	        "rls19_parking_movements_per_space_day": 0.9`))

	sources, _, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if got := *sources[0].MovementsPerSpaceDay; math.Abs(got-0.9) > 1e-9 {
		t.Errorf("day rate = %g, want the stated 0.9", got)
	}

	if got := *sources[0].MovementsPerSpaceNight; math.Abs(got-0.06) > 1e-9 {
		t.Errorf("night rate = %g, want the Tabelle 7 value 0.06", got)
	}
}

// TestExtractRLS19ParkingSourcesAcceptsExplicitZero guards the half of the rule
// that must stay legal all the way out to the GeoJSON layer.
func TestExtractRLS19ParkingSourcesAcceptsExplicitZero(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, parkingFeature("lot", `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 0.4,
	        "rls19_parking_movements_per_space_night": 0`))

	sources, _, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("an explicitly stated zero rate is legal: %v", err)
	}

	if got := *sources[0].MovementsPerSpaceNight; got != 0 {
		t.Errorf("night rate = %g, want 0", got)
	}
}

func TestExtractRLS19ParkingSourcesRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		properties string
		want       string
	}{
		{
			name: "parking_type omitted",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: propRLS19ParkingType,
		},
		{
			name: "parking_type unknown",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "lastwagen",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: "lastwagen",
		},
		{
			name: "parking_type as an ordinal",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": 2,
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: "must be a string",
		},
		{
			name: "day rate omitted with no facility type",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_night": 1`,
			want: propRLS19ParkingMovementsDay,
		},
		{
			name: "night rate omitted with no facility type",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1`,
			want: propRLS19ParkingMovementsNight,
		},
		{
			name: "facility type unknown",
			properties: `,
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_facility_type": "tiefgarage"`,
			want: "Tabelle 7",
		},
		{
			name: "num_spaces omitted",
			properties: `,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: propRLS19ParkingNumSpaces,
		},
		{
			name: "num_spaces zero",
			properties: `,
	        "rls19_parking_num_spaces": 0,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: propRLS19ParkingNumSpaces,
		},
		{
			name: "num_spaces fractional",
			properties: `,
	        "rls19_parking_num_spaces": 2.5,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1`,
			want: "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := mustNormalizeModel(t, parkingFeature("lot", tt.properties))

			_, _, err := extractRLS19ParkingSources(model)
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

// TestExtractRLS19ParkingSourcesRejectsMultiPolygon: n spaces can be neither
// split across parts nor duplicated into one lot per part, so the modeller has
// to say which Teilfläche carries which n.
func TestExtractRLS19ParkingSourcesRejectsMultiPolygon(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {
	        "id": "lot", "kind": "source", "source_type": "area",
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1
	      },
	      "geometry": {
	        "type": "MultiPolygon",
	        "coordinates": [
	          [[[0,0],[10,0],[10,10],[0,10],[0,0]]],
	          [[[20,0],[30,0],[30,10],[20,10],[20,0]]]
	        ]
	      }
	    }
	  ]
	}`)

	_, _, err := extractRLS19ParkingSources(model)
	if err == nil {
		t.Fatal("expected a MultiPolygon to be refused")
	}

	if !strings.Contains(err.Error(), "single Polygon") {
		t.Errorf("error %q should explain the one-feature-one-lot rule", err)
	}
}

func TestExtractRLS19ParkingSourcesRejectsDegeneratePolygon(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {
	        "id": "lot", "kind": "source", "source_type": "area",
	        "rls19_parking_num_spaces": 10,
	        "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1
	      },
	      "geometry": {
	        "type": "Polygon",
	        "coordinates": [[[0,0],[10,0],[20,0],[0,0]]]
	      }
	    }
	  ]
	}`)

	_, _, err := extractRLS19ParkingSources(model)
	if err == nil {
		t.Fatal("expected a collinear polygon to be refused")
	}

	if !strings.Contains(err.Error(), "centroid") {
		t.Errorf("error %q should say why the geometry cannot be used", err)
	}
}

// TestExtractRLS19ParkingSourcesAreDeterministic pins model feature order and
// the fallback ID's use of the model index, so an ID cannot collide with a road
// or barrier feature's and cannot shift when an unrelated feature is inserted.
func TestExtractRLS19ParkingSourcesAreDeterministic(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {"id": "road-a", "kind": "source", "source_type": "line"},
	      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
	    },
	    {
	      "type": "Feature",
	      "properties": {
	        "kind": "source", "source_type": "area",
	        "rls19_parking_num_spaces": 10, "rls19_parking_type": "pkw",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1
	      },
	      "geometry": {"type": "Polygon", "coordinates": [[[0,0],[10,0],[10,10],[0,10],[0,0]]]}
	    },
	    {
	      "type": "Feature",
	      "properties": {
	        "id": "named-lot", "kind": "source", "source_type": "area",
	        "rls19_parking_num_spaces": 20, "rls19_parking_type": "motorrad",
	        "rls19_parking_movements_per_space_day": 1,
	        "rls19_parking_movements_per_space_night": 1
	      },
	      "geometry": {"type": "Polygon", "coordinates": [[[50,0],[60,0],[60,10],[50,10],[50,0]]]}
	    }
	  ]
	}`)

	sources, _, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	want := []string{"rls19-parking-001", "named-lot"}
	if len(sources) != len(want) {
		t.Fatalf("expected %d parking sources, got %d", len(want), len(sources))
	}

	for i, id := range want {
		if sources[i].ID != id {
			t.Errorf("source[%d] id = %q, want %q", i, sources[i].ID, id)
		}
	}
}

// TestExtractRLS19RoadSourcesSkipsAreaFeatures: an area feature is a Parkplatz
// and must pass through the road extractor untouched rather than be refused.
func TestExtractRLS19RoadSourcesSkipsAreaFeatures(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, parkingFeature("lot", parkingComplete))

	options := rls19RoadRunOptions{
		SurfaceType:  string(rls19road.SurfaceSMA),
		SpeedPkwKPH:  100,
		SpeedLkw1KPH: 100,
		SpeedLkw2KPH: 80,
		SpeedKradKPH: 100,
	}

	sources, overrideCount, err := extractRLS19RoadSources(model, options, []string{"line", "area"})
	if err != nil {
		t.Fatalf("an area feature must be skipped, not refused: %v", err)
	}

	if len(sources) != 0 || overrideCount != 0 {
		t.Errorf("expected no road sources from a parking-only model, got %d", len(sources))
	}
}

func TestExtractRLS19RoadSourcesStillRejectsPointFeatures(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
	  "type": "FeatureCollection",
	  "features": [
	    {
	      "type": "Feature",
	      "properties": {"id": "p", "kind": "source", "source_type": "point"},
	      "geometry": {"type": "Point", "coordinates": [0,0]}
	    }
	  ]
	}`)

	_, _, err := extractRLS19RoadSources(model, rls19RoadRunOptions{}, []string{"line", "area"})
	if err == nil {
		t.Fatal("a point source is still unsupported by rls19-road")
	}
}
