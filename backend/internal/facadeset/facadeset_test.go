package facadeset

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// The building mapping reads levels and the height source as the importers
// write them. A building with levels and no height_source is `levels`: the
// projects imported before the importer wrote it carry none. One with neither
// (an OSM height tag alone, CityGML, LGLN) is `assumed`, and the generator
// places points on every mapped building rather than refusing its source.
func TestBuildingFromFeature(t *testing.T) {
	t.Parallel()

	height := 9.0

	for _, tc := range []struct {
		name       string
		properties map[string]any
		levels     int
		source     string
	}{
		{"levels, no source", map[string]any{"building:levels": "3"}, 3, geo.SlabHeightLevels},
		{"levels as a number", map[string]any{"building:levels": float64(3)}, 3, geo.SlabHeightLevels},
		{"levels and measured", map[string]any{"building:levels": "3", "height_source": "measured"}, 3, geo.SlabHeightMeasured},
		{"assumed", map[string]any{"height_source": "assumed"}, 0, geo.SlabHeightAssumed},
		{"nothing", map[string]any{}, 0, geo.SlabHeightAssumed},
		{"fractional levels are unknown", map[string]any{"building:levels": "2.5"}, 0, geo.SlabHeightAssumed},
		{"zero levels are unknown", map[string]any{"building:levels": "0"}, 0, geo.SlabHeightAssumed},
		{"negative levels are unknown", map[string]any{"building:levels": "-2"}, 0, geo.SlabHeightAssumed},
	} {
		feature := modelgeojson.Feature{
			ID:           "a",
			Kind:         modelgeojson.FeatureKindBuilding,
			GeometryType: modelgeojson.GeometryTypePolygon,
			HeightM:      &height,
			Properties:   tc.properties,
			Coordinates:  []any{[]any{[]any{0.0, 0.0}, []any{10.0, 0.0}, []any{10.0, 8.0}, []any{0.0, 0.0}}},
		}

		building, err := BuildingFromFeature(feature)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if building.Levels != tc.levels || building.HeightSource != tc.source || building.HeightM != 9 || building.GroundFloorM != 0 {
			t.Fatalf("%s: building = %+v", tc.name, building)
		}

		_, err = geo.FacadeReceiverSet{
			ID: "facade", BuildingIDs: []string{"a"}, PlacementRule: geo.PlacementRLS19Facade, SpacingM: 5,
		}.Generate([]geo.FacadeBuilding{building})
		if err != nil {
			t.Fatalf("%s: generate: %v", tc.name, err)
		}
	}
}

// The IDs this package spells out are the modules' own: a module renamed
// without this table would offer, and gate, nothing.
func TestStandardIDsAreTheModules(t *testing.T) {
	t.Parallel()

	for got, want := range map[string]string{
		rls19RoadID: rls19road.StandardID,
		schall03ID:  schall03.StandardID,
		iso9613ID:   iso9613.StandardID,
	} {
		if got != want {
			t.Errorf("facadeset spells %q, the module's StandardID is %q", got, want)
		}
	}
}
