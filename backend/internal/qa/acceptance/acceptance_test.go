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

			if _, err := os.Stat(filepath.Join(".", fixture.ScenarioPath)); err != nil {
				t.Fatalf("expected fixture file %s: %v", fixture.ScenarioPath, err)
			}

			if !golden.UpdateEnabled() {
				if _, err := os.Stat(filepath.Join(".", fixture.ExpectedJSONPath)); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
	case cnossosaircraft.StandardID:
		var scenario struct {
			Sources    []cnossosaircraft.AircraftSource `json:"sources"`
			Receivers  []geo.PointReceiver              `json:"receivers"`
			GridWidth  int                              `json:"grid_width"`
			GridHeight int                              `json:"grid_height"`
		}
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
		if err := decodeFixtureJSON(fixture.ScenarioPath, &scenario); err != nil {
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
