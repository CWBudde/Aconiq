package road

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
)

func TestRoadSourceValidate(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.RoadFunctionClass = "nope"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid function class")
	}

	source = sampleSource()
	source.JunctionType = "nope"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid junction type")
	}
}

func TestEmissionIncreasesWithTraffic(t *testing.T) {
	t.Parallel()

	low := sampleSource()
	low.TrafficDay = TrafficPeriod{LightVehiclesPerHour: 200, HeavyVehiclesPerHour: 20}

	high := sampleSource()
	high.TrafficDay = TrafficPeriod{LightVehiclesPerHour: 1200, HeavyVehiclesPerHour: 120}

	lowEmission, err := ComputeEmission(low)
	if err != nil {
		t.Fatalf("compute low emission: %v", err)
	}

	highEmission, err := ComputeEmission(high)
	if err != nil {
		t.Fatalf("compute high emission: %v", err)
	}

	if highEmission.Lday <= lowEmission.Lday {
		t.Fatalf("expected higher traffic to increase Lday")
	}
}

func TestEmissionUsesRoadContext(t *testing.T) {
	t.Parallel()

	base := sampleSource()
	contextual := sampleSource()
	contextual.SurfaceType = SurfaceCobblestone
	contextual.RoadFunctionClass = FunctionRuralMain
	contextual.JunctionType = JunctionTrafficLight
	contextual.JunctionDistanceM = 15
	contextual.TemperatureC = -5
	contextual.StuddedTyreShare = 0.3
	contextual.TrafficDay.MediumVehiclesPerHour = 80
	contextual.TrafficDay.PoweredTwoWheelersPerHour = 20

	baseEmission, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base emission: %v", err)
	}

	contextualEmission, err := ComputeEmission(contextual)
	if err != nil {
		t.Fatalf("compute contextual emission: %v", err)
	}

	if contextualEmission.Lday <= baseEmission.Lday {
		t.Fatalf("expected contextual source to increase Lday: base=%f contextual=%f", baseEmission.Lday, contextualEmission.Lday)
	}
}

func TestPropagationDecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 10}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.Lday <= far.Lday {
		t.Fatalf("expected near level > far level")
	}
}

func TestPropagationIncreasesWithSourceLength(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	short := sampleSource()
	short.Centerline = []geo.Point2D{{X: -10, Y: 0}, {X: 10, Y: 0}}

	long := sampleSource()
	long.Centerline = []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}}

	receiver := geo.Point2D{X: 0, Y: 20}

	shortLevels, err := ComputeReceiverPeriodLevels(receiver, []RoadSource{short}, cfg)
	if err != nil {
		t.Fatalf("compute short source levels: %v", err)
	}

	longLevels, err := ComputeReceiverPeriodLevels(receiver, []RoadSource{long}, cfg)
	if err != nil {
		t.Fatalf("compute long source levels: %v", err)
	}

	if longLevels.Lday <= shortLevels.Lday {
		t.Fatalf("expected longer source to increase level: short=%f long=%f", shortLevels.Lday, longLevels.Lday)
	}
}

func TestAttenuationTermsExposeMappingComponents(t *testing.T) {
	t.Parallel()

	// Expected values are derived by hand, not by calling the helpers under
	// test:
	//
	//   Adiv   = 20 lg(d) + 11 = 20 lg(50) + 11 = 44.979400... dB
	//   Aatm   = 0.7 * 50 / 1000 = 0.035 dB
	//   Aground = 1.2 dB                     (DefaultPropagationConfig)
	//   Acanyon = 1.5 dB                     (set on cfg below)
	//   Aintersection = min(3.0, 30 / 20) = 1.5 dB
	//
	// BUB has no published emission/propagation coefficient set in this
	// module yet (PLAN.md Priority 4), so these are scaffold constants
	// pinned as a regression guard, not normative values.
	cfg := DefaultPropagationConfig()
	cfg.UrbanCanyonDB = 1.5
	cfg.IntersectionDensityPerKM = 30

	wantGeometricDB := 20*math.Log10(50) + 11.0

	terms := attenuationTerms(50, cfg)

	if math.Abs(terms.DistanceM-50) > 1e-9 {
		t.Fatalf("distance term = %v, want 50", terms.DistanceM)
	}

	if math.Abs(terms.GeometricDB-wantGeometricDB) > 1e-9 {
		t.Fatalf("geometric term = %v, want %v", terms.GeometricDB, wantGeometricDB)
	}

	if math.Abs(terms.AirDB-0.035) > 1e-9 {
		t.Fatalf("air term = %v, want 0.035", terms.AirDB)
	}

	if math.Abs(terms.GroundDB-1.2) > 1e-9 {
		t.Fatalf("ground term = %v, want 1.2", terms.GroundDB)
	}

	if math.Abs(terms.UrbanCanyonDB-1.5) > 1e-9 {
		t.Fatalf("urban canyon term = %v, want 1.5", terms.UrbanCanyonDB)
	}

	if math.Abs(terms.IntersectionDB-1.5) > 1e-9 {
		t.Fatalf("intersection term = %v, want 1.5", terms.IntersectionDB)
	}
}

func TestIntersectionEffectSaturatesAtThreeDecibels(t *testing.T) {
	t.Parallel()

	// min(3.0, density / 20): 60 /km is the saturation point, anything above
	// it must stay clamped at 3.0 dB.
	cfg := DefaultPropagationConfig()
	cfg.IntersectionDensityPerKM = 60

	if got := attenuationTerms(50, cfg).IntersectionDB; math.Abs(got-3.0) > 1e-9 {
		t.Fatalf("intersection term at 60 /km = %v, want 3.0", got)
	}

	cfg.IntersectionDensityPerKM = 500

	if got := attenuationTerms(50, cfg).IntersectionDB; math.Abs(got-3.0) > 1e-9 {
		t.Fatalf("intersection term at 500 /km = %v, want 3.0 (clamped)", got)
	}
}

func TestSubsegmentContributionUsesMinimumDistance(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.MinDistanceM = 5
	subsegment := lineSubsegment{Midpoint: geo.Point2D{X: 0, Y: 0}, LengthM: 10}

	near := subsegmentContribution(70, geo.Point2D{X: 0, Y: 0.5}, subsegment, cfg)
	clamped := subsegmentContribution(70, geo.Point2D{X: 0, Y: 5}, subsegment, cfg)

	if math.Abs(near-clamped) > 1e-9 {
		t.Fatalf("expected min-distance clamping to stabilize contribution: near=%f clamped=%f", near, clamped)
	}
}

func TestMappingContextImprovesReceiverLevels(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 20}

	baseCfg := DefaultPropagationConfig()
	contextualCfg := DefaultPropagationConfig()
	contextualCfg.UrbanCanyonDB = 2.0
	contextualCfg.IntersectionDensityPerKM = 40

	baseLevels, err := ComputeReceiverPeriodLevels(receiver, []RoadSource{source}, baseCfg)
	if err != nil {
		t.Fatalf("compute base receiver levels: %v", err)
	}

	contextualLevels, err := ComputeReceiverPeriodLevels(receiver, []RoadSource{source}, contextualCfg)
	if err != nil {
		t.Fatalf("compute contextual receiver levels: %v", err)
	}

	if contextualLevels.Lday <= baseLevels.Lday {
		t.Fatalf("expected mapping context to increase Lday: base=%f contextual=%f", baseLevels.Lday, contextualLevels.Lday)
	}
}

func TestExportResultBundle(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
		{ID: "r4", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []RoadSource{sampleSource()}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, 2, 2)
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected exported file %s: %v", path, err)
		}
	}

	raster, err := results.LoadRaster(exported.RasterMetaPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	if raster.Metadata().Bands != 2 {
		t.Fatalf("expected 2 raster bands")
	}
}

func TestDescriptorValidates(t *testing.T) {
	t.Parallel()

	err := Descriptor().Validate()
	if err != nil {
		t.Fatalf("descriptor should validate: %v", err)
	}
}

func TestBUBRoadProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"road_function_class":         FunctionUrbanMain,
		"road_junction_type":          JunctionTrafficLight,
		"road_temperature_c":          "5",
		"traffic_day_medium_vph":      "80",
		"traffic_day_ptw_vph":         "20",
		"urban_canyon_db":             "1.5",
		"intersection_density_per_km": "30",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["compliance_boundary"] != "baseline-preview-expanded-bub-road-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", metadata)
	}

	if metadata["key_parameter.road_junction_type"] != JunctionTrafficLight || metadata["key_parameter.traffic_day_medium_vph"] != "80" {
		t.Fatalf("expected expanded key parameters in metadata: %#v", metadata)
	}
}

func TestGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Sources    []RoadSource        `json:"sources"`
		Receivers  []geo.PointReceiver `json:"receivers"`
		GridWidth  int                 `json:"grid_width"`
		GridHeight int                 `json:"grid_height"`
	}

	payload, err := os.ReadFile(testdataPath(t, "road_scenario.json"))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}

	err = json.Unmarshal(payload, &scenario)
	if err != nil {
		t.Fatalf("decode scenario: %v", err)
	}

	outputs, err := ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	snapshot := map[string]any{
		"receiver_count": len(outputs),
		"grid_width":     scenario.GridWidth,
		"grid_height":    scenario.GridHeight,
		"receivers":      roundedOutputs(outputs),
	}

	golden.AssertJSONSnapshot(t, testdataPath(t, "road_scenario.golden.json"), snapshot)
}

func roundedOutputs(outputs []ReceiverOutput) []map[string]any {
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

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve road test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func sampleSource() RoadSource {
	return RoadSource{
		ID:                "bub-road-1",
		SurfaceType:       SurfaceDenseAsphalt,
		RoadFunctionClass: FunctionUrbanMain,
		SpeedKPH:          60,
		JunctionType:      JunctionNone,
		JunctionDistanceM: 0,
		TemperatureC:      15,
		StuddedTyreShare:  0,
		Centerline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		TrafficDay: TrafficPeriod{
			LightVehiclesPerHour:      900,
			MediumVehiclesPerHour:     120,
			HeavyVehiclesPerHour:      90,
			PoweredTwoWheelersPerHour: 30,
		},
		TrafficEvening: TrafficPeriod{
			LightVehiclesPerHour:      500,
			MediumVehiclesPerHour:     60,
			HeavyVehiclesPerHour:      45,
			PoweredTwoWheelersPerHour: 15,
		},
		TrafficNight: TrafficPeriod{
			LightVehiclesPerHour:      250,
			MediumVehiclesPerHour:     30,
			HeavyVehiclesPerHour:      30,
			PoweredTwoWheelersPerHour: 5,
		},
	}
}

func TestEmissionFlowTermScalesByDecade(t *testing.T) {
	t.Parallel()

	// Scaling every vehicle class by ten must raise the period level by
	// exactly 10 lg(10) = 10.0 dB, because the flow term is the only thing
	// that changes and it enters each class emission additively before the
	// energy sum.
	//
	// The old 10 lg(Q + 1) form failed this: it gave 7.404 dB for the 1 -> 10
	// step and a spurious +3.0 dB at Q = 1 veh/h.
	low := sampleSource()
	low.TrafficDay = TrafficPeriod{
		LightVehiclesPerHour:      90,
		MediumVehiclesPerHour:     12,
		HeavyVehiclesPerHour:      9,
		PoweredTwoWheelersPerHour: 3,
	}

	high := sampleSource()
	high.TrafficDay = TrafficPeriod{
		LightVehiclesPerHour:      900,
		MediumVehiclesPerHour:     120,
		HeavyVehiclesPerHour:      90,
		PoweredTwoWheelersPerHour: 30,
	}

	lowEmission, err := ComputeEmission(low)
	if err != nil {
		t.Fatalf("compute low-flow emission: %v", err)
	}

	highEmission, err := ComputeEmission(high)
	if err != nil {
		t.Fatalf("compute high-flow emission: %v", err)
	}

	if delta := highEmission.Lday - lowEmission.Lday; math.Abs(delta-10.0) > 1e-9 {
		t.Fatalf("decade flow delta = %v dB, want 10.0", delta)
	}
}

func TestEmissionDropsSilentVehicleClasses(t *testing.T) {
	t.Parallel()

	// A vehicle class with zero flow must contribute nothing. Under the old
	// 10 lg(Q + 1) form it still contributed its full base level, so an
	// empty class raised the period level.
	lightOnly := sampleSource()
	lightOnly.TrafficDay = TrafficPeriod{LightVehiclesPerHour: 900}

	withEmptyClasses := sampleSource()
	withEmptyClasses.TrafficDay = TrafficPeriod{
		LightVehiclesPerHour:      900,
		MediumVehiclesPerHour:     0,
		HeavyVehiclesPerHour:      0,
		PoweredTwoWheelersPerHour: 0,
	}

	lightEmission, err := ComputeEmission(lightOnly)
	if err != nil {
		t.Fatalf("compute light-only emission: %v", err)
	}

	emptyEmission, err := ComputeEmission(withEmptyClasses)
	if err != nil {
		t.Fatalf("compute emission with empty classes: %v", err)
	}

	if math.Abs(lightEmission.Lday-emptyEmission.Lday) > 1e-12 {
		t.Fatalf("empty classes changed the level: %v vs %v", lightEmission.Lday, emptyEmission.Lday)
	}

	silent := sampleSource()
	silent.TrafficDay = TrafficPeriod{}

	silentEmission, err := ComputeEmission(silent)
	if err != nil {
		t.Fatalf("compute silent emission: %v", err)
	}

	if silentEmission.Lday != silenceDB {
		t.Fatalf("Lday without traffic = %v, want the silence sentinel %v", silentEmission.Lday, silenceDB)
	}
}
