package acceptance

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/qa/golden"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestCatalogProvidesDeterministicFixtures(t *testing.T) {
	t.Parallel()

	fixtures := Catalog()
	if len(fixtures) == 0 {
		t.Fatal("expected acceptance fixtures")
	}

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			if fixture.StandardID == "" || fixture.ScenarioPath == "" || fixture.ExpectedJSONPath == "" {
				t.Fatalf("fixture is incomplete: %#v", fixture)
			}

			_, err := os.Stat(filepath.Join(".", fixture.ScenarioPath))
			if err != nil {
				t.Fatalf("expected fixture file %s: %v", fixture.ScenarioPath, err)
			}

			if !golden.UpdateEnabled() {
				_, err = os.Stat(filepath.Join(".", fixture.ExpectedJSONPath))
				if err != nil {
					t.Fatalf("expected fixture file %s: %v", fixture.ExpectedJSONPath, err)
				}
			}
		})
	}
}

func TestAcceptanceFixtures(t *testing.T) {
	t.Parallel()

	for _, fixture := range Catalog() {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			snapshot, err := computeFixtureSnapshot(fixture)
			if err != nil {
				t.Fatalf("compute acceptance snapshot: %v", err)
			}

			golden.AssertJSONSnapshot(t, fixture.ExpectedJSONPath, snapshot)
		})
	}
}

func computeFixtureSnapshot(fixture Fixture) (map[string]any, error) {
	switch fixture.StandardID {
	case cnossosroad.StandardID:
		var scenario struct {
			Sources    []cnossosroad.RoadSource `json:"sources"`
			Receivers  []geo.PointReceiver      `json:"receivers"`
			GridWidth  int                      `json:"grid_width"`
			GridHeight int                      `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := cnossosroad.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, cnossosroad.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundRoadOutputs(outputs),
		}, nil
	case cnossosrail.StandardID:
		var scenario struct {
			Sources    []cnossosrail.RailSource `json:"sources"`
			Receivers  []geo.PointReceiver      `json:"receivers"`
			GridWidth  int                      `json:"grid_width"`
			GridHeight int                      `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := cnossosrail.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, cnossosrail.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundRailOutputs(outputs),
		}, nil
	case cnossosindustry.StandardID:
		var scenario struct {
			Sources    []cnossosindustry.IndustrySource `json:"sources"`
			Receivers  []geo.PointReceiver              `json:"receivers"`
			GridWidth  int                              `json:"grid_width"`
			GridHeight int                              `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := cnossosindustry.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, cnossosindustry.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundIndustryOutputs(outputs),
		}, nil
	case iso9613.StandardID:
		var scenario struct {
			Sources           []iso9613.PointSource `json:"sources"`
			Receivers         []geo.PointReceiver   `json:"receivers"`
			GridWidth         int                   `json:"grid_width"`
			GridHeight        int                   `json:"grid_height"`
			PropagationConfig struct {
				GroundFactor            float64 `json:"ground_factor"`
				AirTemperatureC         float64 `json:"air_temperature_c"`
				RelativeHumidityPercent float64 `json:"relative_humidity_percent"`
				MeteorologyAssumption   string  `json:"meteorology_assumption"`
				C0Met                   float64 `json:"c0_met"`
				MinDistanceM            float64 `json:"min_distance_m"`
			} `json:"propagation_config"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := iso9613.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, iso9613.PropagationConfig{
			GroundFactor:            scenario.PropagationConfig.GroundFactor,
			AirTemperatureC:         scenario.PropagationConfig.AirTemperatureC,
			RelativeHumidityPercent: scenario.PropagationConfig.RelativeHumidityPercent,
			MeteorologyAssumption:   scenario.PropagationConfig.MeteorologyAssumption,
			MinDistanceM:            scenario.PropagationConfig.MinDistanceM,
		})
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundISO9613Outputs(outputs),
		}, nil
	case cnossosaircraft.StandardID:
		var scenario struct {
			Sources    []cnossosaircraft.AircraftSource `json:"sources"`
			Receivers  []geo.PointReceiver              `json:"receivers"`
			GridWidth  int                              `json:"grid_width"`
			GridHeight int                              `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := cnossosaircraft.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, cnossosaircraft.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundAircraftOutputs(outputs),
		}, nil
	case bubroad.StandardID:
		var scenario struct {
			Sources    []bubroad.RoadSource `json:"sources"`
			Receivers  []geo.PointReceiver  `json:"receivers"`
			GridWidth  int                  `json:"grid_width"`
			GridHeight int                  `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := bubroad.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, bubroad.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundBUBRoadOutputs(outputs),
		}, nil
	case bufaircraft.StandardID:
		var scenario struct {
			Sources    []bufaircraft.AircraftSource `json:"sources"`
			Receivers  []geo.PointReceiver          `json:"receivers"`
			GridWidth  int                          `json:"grid_width"`
			GridHeight int                          `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := bufaircraft.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, bufaircraft.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundBUFAircraftOutputs(outputs),
		}, nil
	case bebexposure.StandardID:
		var scenario struct {
			Buildings []bebexposure.BuildingUnit `json:"buildings"`
			Roads     []bubroad.RoadSource       `json:"roads"`
			Config    struct {
				FloorHeightM         float64 `json:"floor_height_m"`
				DwellingsPerFloor    float64 `json:"dwellings_per_floor"`
				PersonsPerDwelling   float64 `json:"persons_per_dwelling"`
				ThresholdLdenDB      float64 `json:"threshold_lden_db"`
				ThresholdLnightDB    float64 `json:"threshold_lnight_db"`
				OccupancyMode        string  `json:"occupancy_mode"`
				FacadeEvaluationMode string  `json:"facade_evaluation_mode"`
			} `json:"config"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, summary, err := bebexposure.ComputeOutputs(scenario.Buildings, scenario.Roads, defaultBEBExposureConfig(scenario.Config), bubroad.DefaultPropagationConfig(), 4)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"summary":   roundBEBSummary(summary),
			"buildings": roundBEBOutputs(outputs),
		}, nil
	case rls19road.StandardID:
		var scenario struct {
			Sources           []rls19road.RoadSource `json:"sources"`
			Barriers          []rls19road.Barrier    `json:"barriers"`
			Buildings         []rls19road.Building   `json:"buildings"`
			Receivers         []geo.PointReceiver    `json:"receivers"`
			GridWidth         int                    `json:"grid_width"`
			GridHeight        int                    `json:"grid_height"`
			PropagationConfig struct {
				SegmentLengthM   float64                    `json:"segment_length_m"`
				MinDistanceM     float64                    `json:"min_distance_m"`
				ReceiverHeightM  float64                    `json:"receiver_height_m"`
				ReceiverTerrainZ float64                    `json:"receiver_terrain_z"`
				Terrain          []rls19road.TerrainProfile `json:"terrain"`
				Reflectors       []rls19road.Reflector      `json:"reflectors"`
			} `json:"propagation_config"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := rls19road.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, scenario.Barriers, rls19road.PropagationConfig{
			SegmentLengthM:   scenario.PropagationConfig.SegmentLengthM,
			MinDistanceM:     scenario.PropagationConfig.MinDistanceM,
			ReceiverHeightM:  scenario.PropagationConfig.ReceiverHeightM,
			ReceiverTerrainZ: scenario.PropagationConfig.ReceiverTerrainZ,
			Terrain:          scenario.PropagationConfig.Terrain,
			Reflectors:       scenario.PropagationConfig.Reflectors,
			Buildings:        scenario.Buildings,
		})
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundRLS19RoadOutputs(outputs),
		}, nil
	case schall03.StandardID:
		var scenario struct {
			Sources    []schall03.RailSource `json:"sources"`
			Receivers  []geo.PointReceiver   `json:"receivers"`
			GridWidth  int                   `json:"grid_width"`
			GridHeight int                   `json:"grid_height"`
		}

		err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
		if err != nil {
			return nil, err
		}

		outputs, err := schall03.ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, schall03.DefaultPropagationConfig())
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"receiver_count": len(outputs),
			"grid_width":     scenario.GridWidth,
			"grid_height":    scenario.GridHeight,
			"receivers":      roundSchall03Outputs(outputs),
		}, nil
	default:
		return nil, os.ErrNotExist
	}
}

func decodeFixtureJSON(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(payload, target)
}

func roundRoadOutputs(outputs []cnossosroad.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundRailOutputs(outputs []cnossosrail.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundIndustryOutputs(outputs []cnossosindustry.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundISO9613Outputs(outputs []iso9613.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"LpAeq_DW": round6(output.Indicators.LpAeqDW),
			"LpAeq_LT": round6(output.Indicators.LpAeqLT),
		})
	}

	return out
}

func roundAircraftOutputs(outputs []cnossosaircraft.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundBUBRoadOutputs(outputs []bubroad.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundBUFAircraftOutputs(outputs []bufaircraft.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func roundRLS19RoadOutputs(outputs []rls19road.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"LrDay":    round6(output.Indicators.LrDay),
			"LrNight":  round6(output.Indicators.LrNight),
		})
	}

	return out
}

func roundSchall03Outputs(outputs []schall03.ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"LrDay":    round6(output.Indicators.LrDay),
			"LrNight":  round6(output.Indicators.LrNight),
		})
	}

	return out
}

func roundBEBOutputs(outputs []bebexposure.BuildingExposureOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":                        output.Building.ID,
			"x":                         round6(output.RepresentativeReceiver.Point.X),
			"y":                         round6(output.RepresentativeReceiver.Point.Y),
			"Lden":                      round6(output.Indicators.Lden),
			"Lnight":                    round6(output.Indicators.Lnight),
			"estimated_dwellings":       round6(output.Indicators.EstimatedDwellings),
			"estimated_persons":         round6(output.Indicators.EstimatedPersons),
			"affected_dwellings_lden":   round6(output.Indicators.AffectedDwellingsLden),
			"affected_persons_lden":     round6(output.Indicators.AffectedPersonsLden),
			"affected_dwellings_lnight": round6(output.Indicators.AffectedDwellingsLnight),
			"affected_persons_lnight":   round6(output.Indicators.AffectedPersonsLnight),
		})
	}

	return out
}

func roundBEBSummary(summary bebexposure.Summary) map[string]any {
	roundBands := func(bands []bebexposure.ExposureBandSummary) []map[string]any {
		out := make([]map[string]any, 0, len(bands))
		for _, band := range bands {
			item := map[string]any{
				"label":               band.Label,
				"lower_db":            round6(band.LowerDB),
				"estimated_dwellings": round6(band.EstimatedDwellings),
				"estimated_persons":   round6(band.EstimatedPersons),
			}
			if band.UpperDBExclusive != nil {
				item["upper_db_exclusive"] = round6(*band.UpperDBExclusive)
			}

			out = append(out, item)
		}

		return out
	}

	return map[string]any{
		"building_count":            summary.BuildingCount,
		"estimated_dwellings":       round6(summary.EstimatedDwellings),
		"estimated_persons":         round6(summary.EstimatedPersons),
		"affected_dwellings_lden":   round6(summary.AffectedDwellingsLden),
		"affected_persons_lden":     round6(summary.AffectedPersonsLden),
		"affected_dwellings_lnight": round6(summary.AffectedDwellingsLnight),
		"affected_persons_lnight":   round6(summary.AffectedPersonsLnight),
		"threshold_lden_db":         round6(summary.ThresholdLdenDB),
		"threshold_lnight_db":       round6(summary.ThresholdLnightDB),
		"occupancy_mode":            summary.OccupancyMode,
		"facade_evaluation_mode":    summary.FacadeEvaluationMode,
		"upstream_mapping_standard": summary.UpstreamMappingStandard,
		"lden_bands":                roundBands(summary.LdenBands),
		"lnight_bands":              roundBands(summary.LnightBands),
	}
}

func defaultBEBExposureConfig(raw struct {
	FloorHeightM         float64 `json:"floor_height_m"`
	DwellingsPerFloor    float64 `json:"dwellings_per_floor"`
	PersonsPerDwelling   float64 `json:"persons_per_dwelling"`
	ThresholdLdenDB      float64 `json:"threshold_lden_db"`
	ThresholdLnightDB    float64 `json:"threshold_lnight_db"`
	OccupancyMode        string  `json:"occupancy_mode"`
	FacadeEvaluationMode string  `json:"facade_evaluation_mode"`
},
) bebexposure.ExposureConfig {
	cfg := bebexposure.ExposureConfig{
		FloorHeightM:            3,
		DwellingsPerFloor:       1,
		PersonsPerDwelling:      2.2,
		ThresholdLdenDB:         55,
		ThresholdLnightDB:       50,
		OccupancyMode:           bebexposure.OccupancyModePreferFeatureOverrides,
		FacadeEvaluationMode:    bebexposure.FacadeEvaluationCentroid,
		UpstreamMappingStandard: bebexposure.UpstreamStandardBUBRoad,
	}

	if raw.FloorHeightM > 0 {
		cfg.FloorHeightM = raw.FloorHeightM
	}

	if raw.DwellingsPerFloor > 0 {
		cfg.DwellingsPerFloor = raw.DwellingsPerFloor
	}

	if raw.PersonsPerDwelling > 0 {
		cfg.PersonsPerDwelling = raw.PersonsPerDwelling
	}

	if raw.ThresholdLdenDB != 0 {
		cfg.ThresholdLdenDB = raw.ThresholdLdenDB
	}

	if raw.ThresholdLnightDB != 0 {
		cfg.ThresholdLnightDB = raw.ThresholdLnightDB
	}

	if raw.OccupancyMode != "" {
		cfg.OccupancyMode = raw.OccupancyMode
	}

	if raw.FacadeEvaluationMode != "" {
		cfg.FacadeEvaluationMode = raw.FacadeEvaluationMode
	}

	return cfg
}

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

// iso9613Tolerance returns the Table 5 tolerance (dB) from ISO 9613-2:1996
// Section 9 for broadband A-weighted levels under downwind conditions without
// screening or reflection.
//
// Parameters:
//   - meanHeight: mean height of source and receiver (m)
//   - distance:   horizontal source-receiver distance (m)
//
// Outside the stated applicability (h > 30 m or d > 1000 m) the function
// returns +Inf so that no tolerance assertion can pass silently.
func iso9613Tolerance(meanHeight, distance float64) float64 {
	switch {
	case distance > 1000 || meanHeight > 30:
		return math.Inf(1)
	case meanHeight >= 5 && distance < 100:
		return 1
	default:
		return 3
	}
}

func TestISO9613ToleranceLookup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		meanHeight float64
		distance   float64
		want       float64
	}{
		// h < 5 m, d < 100 m  → ±3 dB
		{name: "low-height-short-range", meanHeight: 2, distance: 50, want: 3},
		{name: "low-height-at-100m-boundary", meanHeight: 2, distance: 100, want: 3},
		// h < 5 m, d = 100–1000 m  → ±3 dB
		{name: "low-height-mid-range", meanHeight: 4.9, distance: 500, want: 3},
		{name: "low-height-at-1000m", meanHeight: 2, distance: 1000, want: 3},
		// h = 5–30 m, d < 100 m  → ±1 dB
		{name: "mid-height-short-range", meanHeight: 5, distance: 50, want: 1},
		{name: "mid-height-just-below-100m", meanHeight: 15, distance: 99, want: 1},
		// h = 5–30 m, d = 100–1000 m  → ±3 dB
		{name: "mid-height-mid-range", meanHeight: 10, distance: 500, want: 3},
		{name: "mid-height-at-1000m", meanHeight: 30, distance: 1000, want: 3},
		// Outside applicability → +Inf
		{name: "over-height", meanHeight: 31, distance: 100, want: math.Inf(1)},
		{name: "over-distance", meanHeight: 10, distance: 1001, want: math.Inf(1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := iso9613Tolerance(tc.meanHeight, tc.distance)
			if got != tc.want {
				t.Errorf("iso9613Tolerance(%.1f, %.1f) = %v, want %v",
					tc.meanHeight, tc.distance, got, tc.want)
			}
		})
	}
}

func TestISO9613ToleranceCompliance(t *testing.T) {
	t.Parallel()

	for _, fixture := range Catalog() {
		if fixture.StandardID != iso9613.StandardID {
			continue
		}

		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			// Decode the scenario to get source/receiver geometry.
			var scenario struct {
				Sources   []iso9613.PointSource `json:"sources"`
				Receivers []geo.PointReceiver   `json:"receivers"`
			}

			err := decodeFixtureJSON(fixture.ScenarioPath, &scenario)
			if err != nil {
				t.Fatalf("decode scenario: %v", err)
			}

			// Compute the snapshot (our current output).
			snapshot, err := computeFixtureSnapshot(fixture)
			if err != nil {
				t.Fatalf("compute snapshot: %v", err)
			}

			// Load the golden expected output.
			goldenBytes, err := os.ReadFile(fixture.ExpectedJSONPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			var golden map[string]any
			if err := json.Unmarshal(goldenBytes, &golden); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}

			// Parse computed and expected receiver slices.
			computedReceivers := snapshotReceivers(t, snapshot)
			expectedReceivers := snapshotReceivers(t, golden)

			if len(computedReceivers) != len(expectedReceivers) {
				t.Fatalf("receiver count mismatch: computed %d, expected %d",
					len(computedReceivers), len(expectedReceivers))
			}

			// Build a source lookup by ID for geometry.
			sourceByID := make(map[string]iso9613.PointSource, len(scenario.Sources))
			for _, s := range scenario.Sources {
				sourceByID[s.ID] = s
			}

			// Build a receiver lookup by ID for geometry.
			receiverByID := make(map[string]geo.PointReceiver, len(scenario.Receivers))
			for _, r := range scenario.Receivers {
				receiverByID[r.ID] = r
			}

			for i, comp := range computedReceivers {
				exp := expectedReceivers[i]
				rid := comp.id

				recv, ok := receiverByID[rid]
				if !ok {
					t.Fatalf("receiver %q not found in scenario", rid)
				}

				// Determine the most conservative tolerance across all
				// source-receiver pairs contributing to this receiver.
				maxTol := 0.0
				for _, src := range scenario.Sources {
					dx := src.Point.X - recv.Point.X
					dy := src.Point.Y - recv.Point.Y
					dist := math.Sqrt(dx*dx + dy*dy)
					meanH := (src.SourceHeightM + recv.HeightM) / 2

					tol := iso9613Tolerance(meanH, dist)
					if tol > maxTol {
						maxTol = tol
					}
				}

				// Check LpAeq_DW.
				diffDW := math.Abs(comp.lpAeqDW - exp.lpAeqDW)
				if diffDW > maxTol {
					t.Errorf("receiver %q LpAeq_DW: |%.6f - %.6f| = %.6f > tolerance %.0f dB",
						rid, comp.lpAeqDW, exp.lpAeqDW, diffDW, maxTol)
				}

				// Check LpAeq_LT.
				diffLT := math.Abs(comp.lpAeqLT - exp.lpAeqLT)
				if diffLT > maxTol {
					t.Errorf("receiver %q LpAeq_LT: |%.6f - %.6f| = %.6f > tolerance %.0f dB",
						rid, comp.lpAeqLT, exp.lpAeqLT, diffLT, maxTol)
				}

				t.Logf("receiver %q: DW diff=%.6f dB, LT diff=%.6f dB, tolerance=%.0f dB",
					rid, diffDW, diffLT, maxTol)
			}
		})
	}
}

// iso9613ReceiverRow holds parsed indicator values for one receiver.
type iso9613ReceiverRow struct {
	id      string
	lpAeqDW float64
	lpAeqLT float64
}

// snapshotReceivers extracts the receiver rows from a snapshot map.
func snapshotReceivers(t *testing.T, snap map[string]any) []iso9613ReceiverRow {
	t.Helper()

	raw, ok := snap["receivers"]
	if !ok {
		t.Fatal("snapshot missing 'receivers' key")
	}

	slice, ok := raw.([]any)
	if !ok {
		// Handle the typed []map[string]any produced by computeFixtureSnapshot.
		if typed, ok2 := raw.([]map[string]any); ok2 {
			slice = make([]any, len(typed))
			for i, m := range typed {
				slice[i] = m
			}
		} else {
			t.Fatalf("unexpected receivers type: %T", raw)
		}
	}

	rows := make([]iso9613ReceiverRow, 0, len(slice))

	for _, entry := range slice {
		m, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("receiver entry is %T, want map[string]any", entry)
		}

		rows = append(rows, iso9613ReceiverRow{
			id:      m["id"].(string),
			lpAeqDW: m["LpAeq_DW"].(float64),
			lpAeqLT: m["LpAeq_LT"].(float64),
		})
	}

	return rows
}
