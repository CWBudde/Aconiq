package cli

import (
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// The extraction functions decode feature property overrides in a fixed order,
// and the first decode error is the one the user sees. Nothing else in the suite
// pins that order: the TestExtract…UsesFeatureProperties tests all decode
// successfully, so a reordering would leave them green while changing which
// error a feature with two bad properties reports.
//
// Each case below carries two malformed properties and names the one that must
// win, so the pair of cases per extractor brackets the table from both ends.

func overrideModel(t *testing.T, kind, sourceType, geometry, properties string) modelgeojson.Model {
	t.Helper()

	return mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "f-1", "kind": "`+kind+`"`+sourceType+`, `+properties+`},
      "geometry": `+geometry+`
    }
  ]
}`)
}

const (
	lineGeometry  = `{"type": "LineString", "coordinates": [[0,0],[100,0]]}`
	pointGeometry = `{"type": "Point", "coordinates": [0,0]}`
	polyGeometry  = `{"type": "Polygon", "coordinates": [[[0,0],[10,0],[10,10],[0,10],[0,0]]]}`
)

func rls19PrecedenceOptions() rls19RoadRunOptions {
	return rls19RoadRunOptions{
		SurfaceType:      string(rls19road.SurfaceSMA),
		SpeedPkwKPH:      100,
		SpeedLkw1KPH:     100,
		SpeedLkw2KPH:     80,
		SpeedKradKPH:     100,
		TrafficDayPkw:    900,
		TrafficDayLkw1:   40,
		TrafficDayLkw2:   60,
		TrafficDayKrad:   10,
		TrafficNightPkw:  200,
		TrafficNightLkw1: 10,
		TrafficNightLkw2: 20,
		TrafficNightKrad: 2,
		SegmentLengthM:   1,
		MinDistanceM:     3,
	}
}

func TestExtractOverrideErrorPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       string
		sourceType string
		geometry   string
		properties string
		extract    func(modelgeojson.Model) error
		want       string
	}{
		{
			name:       "cnossos road first override wins",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"road_surface_type": 5, "road_speed_kph": "fast"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosRoadSources(m, cnossosRoadRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosRoadSources: feature "f-1": property "road_surface_type" must be a string`,
		},
		{
			name:       "cnossos road last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"traffic_night_ptw_vph": "many"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosRoadSources(m, cnossosRoadRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosRoadSources: feature "f-1": property "traffic_night_ptw_vph": must be a finite number`,
		},
		{
			name:       "cnossos rail first override wins",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"rail_traction_type": 5, "rail_on_bridge": 7`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosRailSources(m, cnossosRailRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosRailSources: feature "f-1": property "rail_traction_type" must be a string`,
		},
		{
			name:       "cnossos rail last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"traffic_night_trains_per_hour": "lots"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosRailSources(m, cnossosRailRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosRailSources: feature "f-1": property "traffic_night_trains_per_hour": must be a finite number`,
		},
		{
			name:       "cnossos aircraft per-feature override precedes per-track",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"track_start_height_m": "high", "airport_id": 5`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosAircraftSources(m, cnossosAircraftRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosAircraftSources: feature "f-1": property "track_start_height_m": must be a finite number`,
		},
		{
			name:       "cnossos aircraft last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"movement_night_per_hour": "many"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosAircraftSources(m, cnossosAircraftRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractCnossosAircraftSources: feature "f-1": property "movement_night_per_hour": must be a finite number`,
		},
		{
			name:       "buf aircraft per-feature override precedes per-track",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"track_start_height_m": "high", "airport_id": 5`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBUFAircraftSources(m, bufAircraftRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractBUFAircraftSources: feature "f-1": property "track_start_height_m": must be a finite number`,
		},
		{
			name:       "buf aircraft last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"movement_night_per_hour": "many"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBUFAircraftSources(m, bufAircraftRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractBUFAircraftSources: feature "f-1": property "movement_night_per_hour": must be a finite number`,
		},
		{
			name:       "bub road first override wins",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"road_surface_type": 5, "road_function_class": 7`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBUBRoadSources(m, bubRoadRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractBUBRoadSources: feature "f-1": property "road_surface_type" must be a string`,
		},
		{
			name:       "bub road last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"traffic_night_ptw_vph": "many"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBUBRoadSources(m, bubRoadRunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractBUBRoadSources: feature "f-1": property "traffic_night_ptw_vph": must be a finite number`,
		},
		{
			name:       "cnossos industry first override wins",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			properties: `"industry_source_height_m": "high", "industry_source_category": 5`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosIndustrySources(m, cnossosIndustryRunOptions{}, []string{"point", "area"})
				return err
			},
			want: `cli.extractCnossosIndustrySources: feature "f-1": property "industry_source_height_m": must be a finite number`,
		},
		{
			name:       "cnossos industry area branch decodes the same table",
			kind:       "source",
			sourceType: `, "source_type": "area"`,
			geometry:   polyGeometry,
			properties: `"operation_night_factor": "often"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractCnossosIndustrySources(m, cnossosIndustryRunOptions{}, []string{"point", "area"})
				return err
			},
			want: `cli.extractCnossosIndustrySources: feature "f-1": property "operation_night_factor": must be a finite number`,
		},
		{
			name:       "iso9613 first override wins",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			properties: `"iso9613_source_height_m": "high", "iso9613_sound_power_level_db": "loud"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractISO9613Sources(m, iso9613RunOptions{}, []string{"point"})
				return err
			},
			want: `cli.extractISO9613Sources: feature "f-1": property "iso9613_source_height_m": must be a finite number`,
		},
		{
			name:       "iso9613 last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			properties: `"iso9613_impulsivity_correction_db": "some"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractISO9613Sources(m, iso9613RunOptions{}, []string{"point"})
				return err
			},
			want: `cli.extractISO9613Sources: feature "f-1": property "iso9613_impulsivity_correction_db": must be a finite number`,
		},
		{
			name:       "schall03 first override wins",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"rail_train_class": 5, "rail_on_bridge": 7`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractSchall03Sources(m, schall03RunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractSchall03Sources: feature "f-1": property "rail_train_class" must be a string`,
		},
		{
			name:       "schall03 last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"traffic_night_trains_per_hour": "lots"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractSchall03Sources(m, schall03RunOptions{}, []string{"line"})
				return err
			},
			want: `cli.extractSchall03Sources: feature "f-1": property "traffic_night_trains_per_hour": must be a finite number`,
		},
		{
			name:       "beb buildings first override wins",
			kind:       "building",
			sourceType: `, "height_m": 10`,
			geometry:   polyGeometry,
			properties: `"building_usage_type": 5, "floor_count": "three"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBEBBuildings(m, bebExposureRunOptions{})
				return err
			},
			want: `cli.extractBEBBuildings: feature "f-1": property "building_usage_type" must be a string`,
		},
		{
			name:       "beb buildings last override is reached",
			kind:       "building",
			sourceType: `, "height_m": 10`,
			geometry:   polyGeometry,
			properties: `"floor_count": "three"`,
			extract: func(m modelgeojson.Model) error {
				_, err := extractBEBBuildings(m, bebExposureRunOptions{})
				return err
			},
			want: `cli.extractBEBBuildings: feature "f-1": property "floor_count": must be a finite number`,
		},
		{
			name:       "rls19 road first override wins",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"surface_type": 5, "road_speed_kph": "fast"`,
			extract: func(m modelgeojson.Model) error {
				_, _, err := extractRLS19RoadSources(m, rls19PrecedenceOptions(), []string{"line"})
				return err
			},
			want: `cli.extractRLS19RoadSources: feature "f-1": property "surface_type" must be a string`,
		},
		{
			name:       "rls19 road speed fan-out precedes the per-class keys",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"road_speed_kph": "fast", "speed_pkw_kph": "quick"`,
			extract: func(m modelgeojson.Model) error {
				_, _, err := extractRLS19RoadSources(m, rls19PrecedenceOptions(), []string{"line"})
				return err
			},
			want: `cli.extractRLS19RoadSources: feature "f-1": property "road_speed_kph": must be a finite number`,
		},
		{
			name:       "rls19 road last override is reached",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			properties: `"traffic_night_krad": "many"`,
			extract: func(m modelgeojson.Model) error {
				_, _, err := extractRLS19RoadSources(m, rls19PrecedenceOptions(), []string{"line"})
				return err
			},
			want: `cli.extractRLS19RoadSources: feature "f-1": property "traffic_night_krad": must be a finite number`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := overrideModel(t, test.kind, test.sourceType, test.geometry, test.properties)

			err := test.extract(model)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}

			if err.Error() != test.want {
				t.Fatalf("unexpected error\n got: %s\nwant: %s", err.Error(), test.want)
			}
		})
	}
}

// Every fixture under testdata/ gives each feature an explicit "id", so the
// generated-ID fallbacks in the extractors are reached by nothing else in the
// suite. Those IDs are written into receiver tables and provenance, so both the
// prefix and the %03d index are an output contract rather than an
// implementation detail. The index is the feature's position in the model, not
// a counter over the sources that were kept, which is why each case puts a
// skipped receiver feature first.

func idlessModel(t *testing.T, kind, sourceType, geometry string) modelgeojson.Model {
	t.Helper()

	return mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4},
      "geometry": `+pointGeometry+`
    },
    {
      "type": "Feature",
      "properties": {"kind": "`+kind+`"`+sourceType+`},
      "geometry": `+geometry+`
    }
  ]
}`)
}

func TestExtractGeneratedSourceIDFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       string
		sourceType string
		geometry   string
		extract    func(modelgeojson.Model) ([]string, error)
		want       string
	}{
		{
			name:       "dummy freefield",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractDummySources(m, 100, []string{"point"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "source-001",
		},
		{
			name:       "cnossos road",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractCnossosRoadSources(m, cnossosRoadRunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "road-source-001",
		},
		{
			name:       "cnossos rail",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractCnossosRailSources(m, cnossosRailRunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "rail-source-001",
		},
		{
			name:       "cnossos aircraft",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractCnossosAircraftSources(m, cnossosAircraftRunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "aircraft-source-001",
		},
		{
			name:       "cnossos industry",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractCnossosIndustrySources(m, cnossosIndustryRunOptions{}, []string{"point", "area"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "industry-source-001",
		},
		{
			name:       "bub road",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractBUBRoadSources(m, bubRoadRunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "bub-road-source-001",
		},
		{
			name:       "buf aircraft",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractBUFAircraftSources(m, bufAircraftRunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "buf-aircraft-source-001",
		},
		{
			name:       "iso9613",
			kind:       "source",
			sourceType: `, "source_type": "point"`,
			geometry:   pointGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractISO9613Sources(m, iso9613RunOptions{}, []string{"point"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "iso9613-source-001",
		},
		{
			name:       "schall03",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, err := extractSchall03Sources(m, schall03RunOptions{}, []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "schall03-source-001",
		},
		{
			name:       "rls19 road",
			kind:       "source",
			sourceType: `, "source_type": "line"`,
			geometry:   lineGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				sources, _, err := extractRLS19RoadSources(m, rls19PrecedenceOptions(), []string{"line"})

				ids := make([]string, 0, len(sources))
				for _, source := range sources {
					ids = append(ids, source.ID)
				}

				return ids, err
			},
			want: "rls19-road-source-001",
		},
		{
			name:       "beb buildings",
			kind:       "building",
			sourceType: `, "height_m": 10`,
			geometry:   polyGeometry,
			extract: func(m modelgeojson.Model) ([]string, error) {
				units, err := extractBEBBuildings(m, bebExposureRunOptions{})

				ids := make([]string, 0, len(units))
				for _, unit := range units {
					ids = append(ids, unit.ID)
				}

				return ids, err
			},
			want: "beb-building-001",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := idlessModel(t, test.kind, test.sourceType, test.geometry)

			ids, err := test.extract(model)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}

			if len(ids) != 1 {
				t.Fatalf("expected exactly 1 extracted value, got %d (%v)", len(ids), ids)
			}

			if ids[0] != test.want {
				t.Fatalf("generated id = %q, want %q", ids[0], test.want)
			}
		})
	}
}
