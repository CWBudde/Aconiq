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
	if err := source.Validate(); err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.BrakingShare = 2
	if err := source.Validate(); err == nil {
		t.Fatal("expected invalid braking share")
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
		if _, err := os.Stat(path); err != nil {
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
	if err := json.Unmarshal(payload, &scenario); err != nil {
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
