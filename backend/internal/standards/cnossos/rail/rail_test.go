package rail

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

func TestRailSourceValidate(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.BrakingShare = 2

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid braking share")
	}

	source = sampleSource()
	source.TrackType = "unknown"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid track type")
	}
}

func TestRailEmissionAndPropagation(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	emission, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("compute emission: %v", err)
	}

	if emission.Lday <= emission.Lnight {
		t.Fatalf("expected day emission > night emission")
	}

	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 5}, []RailSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 250}, []RailSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.Lday <= far.Lday {
		t.Fatalf("expected near level > far level")
	}
}

func TestRailEmissionUsesTrackTypeAndInfrastructure(t *testing.T) {
	t.Parallel()

	ballasted := sampleSource()
	ballasted.TrackType = TrackTypeBallasted
	ballasted.OnBridge = false
	ballasted.CurveRadiusM = 900

	ballastedEmission, err := ComputeEmission(ballasted)
	if err != nil {
		t.Fatalf("compute ballasted emission: %v", err)
	}

	slab := sampleSource()
	slab.TrackType = TrackTypeSlab
	slab.OnBridge = true
	slab.CurveRadiusM = 250

	slabEmission, err := ComputeEmission(slab)
	if err != nil {
		t.Fatalf("compute slab emission: %v", err)
	}

	if slabEmission.Lday <= ballastedEmission.Lday {
		t.Fatalf("expected slab/bridge/curve case to increase Lday: ballasted=%f slab=%f", ballastedEmission.Lday, slabEmission.Lday)
	}
}

func TestRailPropagationIncreasesWithSourceLength(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	short := sampleSource()
	short.TrackCenterline = []geo.Point2D{{X: -10, Y: 0}, {X: 10, Y: 0}}

	long := sampleSource()
	long.TrackCenterline = []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}}

	receiver := geo.Point2D{X: 0, Y: 20}

	shortLevels, err := ComputeReceiverPeriodLevels(receiver, []RailSource{short}, cfg)
	if err != nil {
		t.Fatalf("compute short source levels: %v", err)
	}

	longLevels, err := ComputeReceiverPeriodLevels(receiver, []RailSource{long}, cfg)
	if err != nil {
		t.Fatalf("compute long source levels: %v", err)
	}

	if longLevels.Lday <= shortLevels.Lday {
		t.Fatalf("expected longer source to increase level: short=%f long=%f", shortLevels.Lday, longLevels.Lday)
	}
}

func TestRailAttenuationTermsExposePropagationComponents(t *testing.T) {
	t.Parallel()

	// Expected values are derived by hand, not by calling the helpers under
	// test:
	//
	//   Adiv = 20 lg(d) + 11        (CNOSSOS-EU, Directive (EU) 2015/996,
	//                                Annex II, geometrical divergence)
	//        = 20 lg(100) + 11 = 51.0 dB
	//   Aatm = alpha * d / 1000     (scaffold linear air-absorption model)
	//        = 0.7 * 100 / 1000 = 0.07 dB
	//
	// The ground, bridge, and curve terms come from the scaffold's own
	// defaults and from sampleSource (no bridge, curve radius 400 m):
	//
	//   Aground = 1.2 dB            (DefaultPropagationConfig)
	//   Abridge = 0 dB              (OnBridge == false)
	//   Acurve  = (500 - 400) / 500 * 5.0 = 1.0 dB
	//
	// These are scaffold constants, not CNOSSOS coefficients (PLAN.md
	// Priority 4); they are pinned as a regression guard only.
	const (
		wantDistanceM   = 100.0
		wantGeometricDB = 51.0
		wantAirDB       = 0.07
		wantGroundDB    = 1.2
		wantBridgeDB    = 0.0
		wantCurveDB     = 1.0
	)

	terms := attenuationTerms(sampleSource(), wantDistanceM, DefaultPropagationConfig())

	if math.Abs(terms.DistanceM-wantDistanceM) > 1e-9 {
		t.Fatalf("effective distance = %v, want %v", terms.DistanceM, wantDistanceM)
	}

	if math.Abs(terms.GeometricDB-wantGeometricDB) > 1e-9 {
		t.Fatalf("geometric term = %v, want %v", terms.GeometricDB, wantGeometricDB)
	}

	if math.Abs(terms.AirDB-wantAirDB) > 1e-9 {
		t.Fatalf("air term = %v, want %v", terms.AirDB, wantAirDB)
	}

	if math.Abs(terms.GroundDB-wantGroundDB) > 1e-9 {
		t.Fatalf("ground term = %v, want %v", terms.GroundDB, wantGroundDB)
	}

	if math.Abs(terms.BridgeDB-wantBridgeDB) > 1e-9 {
		t.Fatalf("bridge term = %v, want %v", terms.BridgeDB, wantBridgeDB)
	}

	if math.Abs(terms.CurveDB-wantCurveDB) > 1e-9 {
		t.Fatalf("curve term = %v, want %v", terms.CurveDB, wantCurveDB)
	}
}

func TestRailPropagationUsesMinimumDistanceClamp(t *testing.T) {
	t.Parallel()

	terms := attenuationTerms(sampleSource(), 0.5, DefaultPropagationConfig())
	if math.Abs(terms.DistanceM-DefaultPropagationConfig().MinDistanceM) > 1e-9 {
		t.Fatalf("expected min distance clamp: %#v", terms)
	}
}

func TestRailBridgeAndCurveCorrectionsIncreaseLevel(t *testing.T) {
	t.Parallel()

	baseSource := sampleSource()
	baseSource.OnBridge = false
	baseSource.CurveRadiusM = 1000

	contextSource := sampleSource()
	contextSource.OnBridge = true
	contextSource.CurveRadiusM = 200

	cfg := DefaultPropagationConfig()
	baseTerms := attenuationTerms(baseSource, 50, cfg)
	contextTerms := attenuationTerms(contextSource, 50, cfg)

	if totalAttenuation(contextTerms) >= totalAttenuation(baseTerms) {
		t.Fatalf("expected bridge/curve terms to reduce attenuation: base=%f contextual=%f", totalAttenuation(baseTerms), totalAttenuation(contextTerms))
	}
}

func TestRailDiscretizeLineSegmentUsesDeterministicMidpoints(t *testing.T) {
	t.Parallel()

	segments := discretizeLineSegment(geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 25, Y: 0})
	if len(segments) != 3 {
		t.Fatalf("expected 3 subsegments, got %d", len(segments))
	}

	if math.Abs(segments[0].Midpoint.X-(25.0/6.0)) > 1e-9 {
		t.Fatalf("unexpected first midpoint: %#v", segments[0])
	}

	if math.Abs(segments[0].LengthM-(25.0/3.0)) > 1e-9 {
		t.Fatalf("unexpected subsegment length: %#v", segments[0])
	}
}

func TestRailLdenAggregation(t *testing.T) {
	t.Parallel()

	levels := PeriodLevels{
		Lday:     58,
		Levening: 58,
		Lnight:   58,
	}
	lden := ComputeLden(levels)

	expected := 64.39524300131856
	if math.Abs(lden-expected) > 1e-9 {
		t.Fatalf("unexpected Lden: got %.12f expected %.12f", lden, expected)
	}
}

func TestRailExportResultBundle(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
		{ID: "r4", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []RailSource{sampleSource()}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, 2, 2)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		{
			_, err := os.Stat(path)
			if err != nil {
				t.Fatalf("expected exported file %s: %v", path, err)
			}
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

func TestRailGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Sources    []RailSource        `json:"sources"`
		Receivers  []geo.PointReceiver `json:"receivers"`
		GridWidth  int                 `json:"grid_width"`
		GridHeight int                 `json:"grid_height"`
	}

	payload, err := os.ReadFile(testdataPath(t, "rail_scenario.json"))
	if err != nil {
		t.Fatalf("read rail scenario: %v", err)
	}

	err = json.Unmarshal(payload, &scenario)
	if err != nil {
		t.Fatalf("decode rail scenario: %v", err)
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

	golden.AssertJSONSnapshot(t, testdataPath(t, "rail_scenario.golden.json"), snapshot)
}

func TestRailProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"rail_track_type":                 TrackTypeSlab,
		"rail_track_roughness_class":      RoughnessRough,
		"traffic_evening_trains_per_hour": "9",
		"rail_on_bridge":                  "true",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["key_parameter.rail_track_type"] != TrackTypeSlab {
		t.Fatalf("expected track type in provenance: %#v", metadata)
	}

	if metadata["key_parameter.traffic_evening_trains_per_hour"] != "9" {
		t.Fatalf("expected evening traffic in provenance: %#v", metadata)
	}
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
		t.Fatal("resolve rail test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func sampleSource() RailSource {
	return RailSource{
		ID:                   "rail-1",
		TractionType:         TractionElectric,
		TrackType:            TrackTypeBallasted,
		TrackRoughnessClass:  RoughnessStandard,
		AverageTrainSpeedKPH: 100,
		BrakingShare:         0.1,
		CurveRadiusM:         400,
		OnBridge:             false,
		TrackCenterline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		TrafficDay:     TrafficPeriod{TrainsPerHour: 12},
		TrafficEvening: TrafficPeriod{TrainsPerHour: 8},
		TrafficNight:   TrafficPeriod{TrainsPerHour: 5},
	}
}

func TestRailEmissionFlowTermScalesByDecade(t *testing.T) {
	t.Parallel()

	// The flow term is the only thing that varies between these two sources,
	// and it enters every component additively before the energy sum, so a
	// tenfold flow must raise the level by exactly 10 lg(10) = 10.0 dB.
	//
	// The old 10 lg(Q + 1) form failed this: it gave
	// 10 lg(11) - 10 lg(2) = 7.404 dB and a spurious +3.0 dB at Q = 1.
	low := sampleSource()
	low.TrafficDay = TrafficPeriod{TrainsPerHour: 1}

	high := sampleSource()
	high.TrafficDay = TrafficPeriod{TrainsPerHour: 10}

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

func TestRailEmissionIsSilentWithoutTrains(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.TrafficDay = TrafficPeriod{}

	emission, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("compute emission: %v", err)
	}

	if emission.Lday != silenceDB {
		t.Fatalf("Lday without trains = %v, want the silence sentinel %v", emission.Lday, silenceDB)
	}

	if emission.Lnight <= 0 {
		t.Fatalf("expected the remaining periods to stay audible: %#v", emission)
	}
}
