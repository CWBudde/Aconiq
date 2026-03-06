package exposure

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
)

func TestBuildingValidate(t *testing.T) {
	t.Parallel()

	building := sampleBuilding()

	err := building.Validate()
	if err != nil {
		t.Fatalf("valid building failed validation: %v", err)
	}

	building.HeightM = 0

	err = building.Validate()
	if err == nil {
		t.Fatal("expected invalid building height")
	}

	building = sampleBuilding()
	floors := -1.0
	building.FloorCount = &floors

	err = building.Validate()
	if err == nil {
		t.Fatal("expected invalid floor count")
	}
}

func TestComputeOutputsThresholds(t *testing.T) {
	t.Parallel()

	outputs, summary, err := ComputeOutputs(
		[]BuildingUnit{sampleBuilding()},
		[]road.RoadSource{sampleRoad()},
		DefaultExposureConfig(),
		road.DefaultPropagationConfig(),
		4,
	)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	if len(outputs) != 1 {
		t.Fatalf("expected one output, got %d", len(outputs))
	}

	if outputs[0].Indicators.AffectedPersonsLden <= 0 {
		t.Fatal("expected Lden affected persons > 0")
	}

	if summary.AffectedPersonsLden != outputs[0].Indicators.AffectedPersonsLden {
		t.Fatal("expected summary and output affected persons to match")
	}
}

func TestComputeOutputsFromAircraftThresholds(t *testing.T) {
	t.Parallel()

	cfg := DefaultExposureConfig()
	cfg.UpstreamMappingStandard = UpstreamStandardBUFAircraft

	outputs, summary, err := ComputeOutputsFromAircraft(
		[]BuildingUnit{sampleBuilding()},
		[]bufaircraft.AircraftSource{sampleAircraft()},
		cfg,
		bufaircraft.DefaultPropagationConfig(),
		4,
	)
	if err != nil {
		t.Fatalf("compute outputs from aircraft: %v", err)
	}

	if len(outputs) != 1 {
		t.Fatalf("expected one output, got %d", len(outputs))
	}

	if summary.UpstreamMappingStandard != UpstreamStandardBUFAircraft {
		t.Fatalf("expected upstream_mapping_standard=%q, got %q", UpstreamStandardBUFAircraft, summary.UpstreamMappingStandard)
	}
}

func TestOccupancyModeUsesFeatureOverrides(t *testing.T) {
	t.Parallel()

	building := sampleBuilding()
	floors := 5.0
	dwellings := 12.0
	persons := 20.0
	building.FloorCount = &floors
	building.EstimatedDwellings = &dwellings
	building.EstimatedPersons = &persons

	occupancy := evaluateOccupancy(building, DefaultExposureConfig())
	if occupancy.Dwellings != dwellings || occupancy.Persons != persons {
		t.Fatalf("expected feature overrides to win: %#v", occupancy)
	}
}

func TestOccupancyModeHeightDerivedIgnoresFeatureOverrides(t *testing.T) {
	t.Parallel()

	building := sampleBuilding()
	dwellings := 12.0
	persons := 20.0
	building.EstimatedDwellings = &dwellings
	building.EstimatedPersons = &persons

	cfg := DefaultExposureConfig()
	cfg.OccupancyMode = OccupancyModeHeightDerived

	occupancy := evaluateOccupancy(building, cfg)
	if occupancy.Dwellings == dwellings || occupancy.Persons == persons {
		t.Fatalf("expected height-derived mode to ignore overrides: %#v", occupancy)
	}
}

func TestFacadeEvaluationUsesMaximumFacadeLevels(t *testing.T) {
	t.Parallel()

	cfg := DefaultExposureConfig()
	cfg.FacadeEvaluationMode = FacadeEvaluationMaxFacade

	prepared, _, err := prepareBuildings([]BuildingUnit{sampleBuilding()}, 4, cfg.FacadeEvaluationMode)
	if err != nil {
		t.Fatalf("prepare buildings: %v", err)
	}

	item := prepared[0]
	levelByID := map[string]levelIndicators{
		item.candidateReceivers[0].ID: {Lden: 55, Lnight: 45},
		item.candidateReceivers[1].ID: {Lden: 60, Lnight: 47},
		item.candidateReceivers[2].ID: {Lden: 58, Lnight: 51},
		item.candidateReceivers[3].ID: {Lden: 57, Lnight: 49},
		item.candidateReceivers[4].ID: {Lden: 56, Lnight: 46},
	}

	receiver, levels, err := selectBuildingLevels(item, levelByID, cfg.FacadeEvaluationMode)
	if err != nil {
		t.Fatalf("select building levels: %v", err)
	}

	if levels.Lden != 60 || levels.Lnight != 51 {
		t.Fatalf("expected max-facade levels, got %#v", levels)
	}

	if receiver.ID != item.candidateReceivers[1].ID {
		t.Fatalf("expected representative receiver to follow max Lden candidate, got %#v", receiver)
	}
}

func TestExportResultBundle(t *testing.T) {
	t.Parallel()

	outputs, summary, err := ComputeOutputs(
		[]BuildingUnit{sampleBuilding()},
		[]road.RoadSource{sampleRoad()},
		DefaultExposureConfig(),
		road.DefaultPropagationConfig(),
		4,
	)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, summary)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath, exported.SummaryPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected exported file %s: %v", path, err)
		}
	}

	raster, err := results.LoadRaster(exported.RasterMetaPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	if raster.Metadata().Bands != 4 {
		t.Fatalf("expected 4 raster bands")
	}
}

func TestGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Buildings []BuildingUnit    `json:"buildings"`
		Roads     []road.RoadSource `json:"roads"`
	}

	payload, err := os.ReadFile(testdataPath(t, "beb_scenario.json"))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}

	if err := json.Unmarshal(payload, &scenario); err != nil {
		t.Fatalf("decode scenario: %v", err)
	}

	outputs, summary, err := ComputeOutputs(scenario.Buildings, scenario.Roads, DefaultExposureConfig(), road.DefaultPropagationConfig(), 4)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	snapshot := map[string]any{
		"summary":   roundedSummary(summary),
		"buildings": roundedOutputs(outputs),
	}

	golden.AssertJSONSnapshot(t, testdataPath(t, "beb_scenario.golden.json"), snapshot)
}

func TestBEBProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"upstream_mapping_standard": "buf-aircraft",
		"occupancy_mode":            "height_derived",
		"facade_evaluation_mode":    "max_facade",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["compliance_boundary"] != "baseline-preview-expanded-beb-exposure-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", metadata)
	}

	if metadata["key_parameter.occupancy_mode"] != "height_derived" || metadata["key_parameter.facade_evaluation_mode"] != "max_facade" {
		t.Fatalf("expected expanded BEB key parameters in metadata: %#v", metadata)
	}
}

func roundedOutputs(outputs []BuildingExposureOutput) []map[string]any {
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

func roundedSummary(summary Summary) map[string]any {
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

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func DefaultExposureConfig() ExposureConfig {
	return ExposureConfig{
		FloorHeightM:            3,
		DwellingsPerFloor:       1,
		PersonsPerDwelling:      2.2,
		ThresholdLdenDB:         55,
		ThresholdLnightDB:       50,
		OccupancyMode:           OccupancyModePreferFeatureOverrides,
		FacadeEvaluationMode:    FacadeEvaluationCentroid,
		UpstreamMappingStandard: UpstreamStandardBUBRoad,
	}
}

func sampleBuilding() BuildingUnit {
	return BuildingUnit{
		ID:        "b-1",
		UsageType: UsageResidential,
		HeightM:   9,
		Footprint: [][]geo.Point2D{
			{
				{X: 30, Y: -10},
				{X: 45, Y: -10},
				{X: 45, Y: 5},
				{X: 30, Y: 5},
				{X: 30, Y: -10},
			},
		},
	}
}

func sampleRoad() road.RoadSource {
	return road.RoadSource{
		ID:                "r-1",
		Centerline:        []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		SurfaceType:       road.SurfaceDenseAsphalt,
		RoadFunctionClass: road.FunctionUrbanMain,
		SpeedKPH:          60,
		JunctionType:      road.JunctionNone,
		JunctionDistanceM: 0,
		TemperatureC:      15,
		StuddedTyreShare:  0,
		TrafficDay: road.TrafficPeriod{
			LightVehiclesPerHour:      900,
			MediumVehiclesPerHour:     120,
			HeavyVehiclesPerHour:      90,
			PoweredTwoWheelersPerHour: 30,
		},
		TrafficEvening: road.TrafficPeriod{
			LightVehiclesPerHour:      500,
			MediumVehiclesPerHour:     60,
			HeavyVehiclesPerHour:      45,
			PoweredTwoWheelersPerHour: 15,
		},
		TrafficNight: road.TrafficPeriod{
			LightVehiclesPerHour:      250,
			MediumVehiclesPerHour:     30,
			HeavyVehiclesPerHour:      30,
			PoweredTwoWheelersPerHour: 5,
		},
	}
}

func sampleAircraft() bufaircraft.AircraftSource {
	return bufaircraft.AircraftSource{
		ID:            "f-1",
		SourceType:    bufaircraft.SourceTypeLine,
		Airport:       bufaircraft.AirportRef{AirportID: "DE-APT", RunwayID: "RWY"},
		OperationType: bufaircraft.OperationDeparture,
		AircraftClass: bufaircraft.AircraftClassNarrow,
		ProcedureType: bufaircraft.ProcedureStandardSID,
		ThrustMode:    bufaircraft.ThrustTakeoff,
		FlightTrack: []geo.Point3D{
			{X: -100, Y: 0, Z: 30},
			{X: 0, Y: 0, Z: 90},
			{X: 100, Y: 0, Z: 180},
		},
		LateralOffsetM:        0,
		ReferencePowerLevelDB: 110,
		EngineStateFactor:     1,
		MovementDay:           bufaircraft.MovementPeriod{MovementsPerHour: 12},
		MovementEvening:       bufaircraft.MovementPeriod{MovementsPerHour: 6},
		MovementNight:         bufaircraft.MovementPeriod{MovementsPerHour: 2},
	}
}
