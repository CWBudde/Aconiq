package schall03

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

	source := RailSource{
		ID: "track-1",
		TrackCenterline: []geo.Point2D{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
		},
		ElevationM:      1.5,
		AverageSpeedKPH: 120,
		Infrastructure: RailInfrastructure{
			TractionType:        TractionElectric,
			TrackType:           TrackTypeBallasted,
			TrackRoughnessClass: RoughnessStandard,
			CurveRadiusM:        400,
		},
		TrafficDay:   TrafficPeriod{TrainsPerHour: 8},
		TrafficNight: TrafficPeriod{TrainsPerHour: 3},
	}

	err := source.Validate()
	if err != nil {
		t.Fatalf("validate source: %v", err)
	}
}

func TestRailSourceRejectsUnknownInfrastructureValue(t *testing.T) {
	t.Parallel()

	source := RailSource{
		ID: "track-1",
		TrackCenterline: []geo.Point2D{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
		},
		AverageSpeedKPH: 120,
		Infrastructure: RailInfrastructure{
			TractionType:        "steam",
			TrackType:           TrackTypeBallasted,
			TrackRoughnessClass: RoughnessStandard,
		},
		TrafficDay:   TrafficPeriod{TrainsPerHour: 8},
		TrafficNight: TrafficPeriod{TrainsPerHour: 3},
	}

	err := source.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestReceiverInputValidate(t *testing.T) {
	t.Parallel()

	receiver := ReceiverInput{
		ID:      "r-1",
		Point:   geo.Point2D{X: 10, Y: 20},
		HeightM: 4,
	}

	err := receiver.Validate()
	if err != nil {
		t.Fatalf("validate receiver: %v", err)
	}
}

func TestEnergeticSumLevels(t *testing.T) {
	t.Parallel()

	got := EnergeticSumLevels(50, 50)

	want := 53.01029995663981
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected energetic sum: got %.12f want %.12f", got, want)
	}
}

func TestSumSpectra(t *testing.T) {
	t.Parallel()

	got := SumSpectra([]OctaveSpectrum{
		{50, 40, 30, 20, 10, 0, -10, -20},
		{50, 40, 30, 20, 10, 0, -10, -20},
	})

	if math.Abs(got[0]-53.01029995663981) > 1e-9 {
		t.Fatalf("unexpected first band sum: %.12f", got[0])
	}

	if math.Abs(got[7]-(-16.989700043360187)) > 1e-9 {
		t.Fatalf("unexpected last band sum: %.12f", got[7])
	}
}

func TestDescriptorValidates(t *testing.T) {
	t.Parallel()

	err := Descriptor().Validate()
	if err != nil {
		t.Fatalf("validate descriptor: %v", err)
	}
}

func TestComputeReceiverOutputs(t *testing.T) {
	t.Parallel()

	outputs, err := ComputeReceiverOutputs([]geo.PointReceiver{
		{ID: "r-1", Point: geo.Point2D{X: 10, Y: 25}, HeightM: 4},
	}, []RailSource{
		{
			ID: "track-1",
			TrackCenterline: []geo.Point2D{
				{X: 0, Y: 0},
				{X: 100, Y: 0},
			},
			AverageSpeedKPH: 100,
			Infrastructure: RailInfrastructure{
				TractionType:        TractionElectric,
				TrackType:           TrackTypeSlab,
				TrackRoughnessClass: RoughnessStandard,
				OnBridge:            true,
				CurveRadiusM:        300,
			},
			TrafficDay:   TrafficPeriod{TrainsPerHour: 8},
			TrafficNight: TrafficPeriod{TrainsPerHour: 4},
		},
	}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	if len(outputs) != 1 {
		t.Fatalf("expected one output, got %d", len(outputs))
	}

	if outputs[0].Indicators.LrDay <= outputs[0].Indicators.LrNight {
		t.Fatalf("expected day > night, got %#v", outputs[0].Indicators)
	}
}

func TestComputeEmissionAndPropagation(t *testing.T) {
	t.Parallel()

	source := RailSource{
		ID: "track-1",
		TrackCenterline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		AverageSpeedKPH: 100,
		Infrastructure: RailInfrastructure{
			TractionType:        TractionElectric,
			TrackType:           TrackTypeSlab,
			TrackRoughnessClass: RoughnessStandard,
			OnBridge:            false,
			CurveRadiusM:        400,
		},
		TrafficDay:   TrafficPeriod{TrainsPerHour: 12},
		TrafficNight: TrafficPeriod{TrainsPerHour: 5},
	}

	emission, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("compute emission: %v", err)
	}

	if emission.DaySpectrum.EnergeticTotal() <= emission.NightSpectrum.EnergeticTotal() {
		t.Fatal("expected day emission > night emission")
	}

	near, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 10}, []RailSource{source}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 200}, []RailSource{source}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.LrDay <= far.LrDay {
		t.Fatalf("expected near level > far level: near=%f far=%f", near.LrDay, far.LrDay)
	}
}

func TestGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Sources    []RailSource        `json:"sources"`
		Receivers  []geo.PointReceiver `json:"receivers"`
		GridWidth  int                 `json:"grid_width"`
		GridHeight int                 `json:"grid_height"`
	}

	payload, err := os.ReadFile(testdataPath(t, "schall03_scenario.json"))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}

	if err := json.Unmarshal(payload, &scenario); err != nil {
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

	golden.AssertJSONSnapshot(t, testdataPath(t, "schall03_scenario.golden.json"), snapshot)
}

func TestExportResultBundle(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	exported, err := ExportResultBundle(baseDir, []ReceiverOutput{
		{
			Receiver: geo.PointReceiver{ID: "r-1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{
				LrDay:   50,
				LrNight: 45,
			},
		},
	}, 1, 1)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	if _, err := os.Stat(exported.ReceiverJSONPath); err != nil {
		t.Fatalf("receiver json missing: %v", err)
	}

	payload, err := os.ReadFile(exported.ReceiverJSONPath)
	if err != nil {
		t.Fatalf("read receiver json: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	if len(table.IndicatorOrder) != 2 || table.IndicatorOrder[0] != IndicatorLrDay {
		t.Fatalf("unexpected indicator order: %#v", table.IndicatorOrder)
	}

	if filepath.Base(exported.RasterMetaPath) != StandardID+".json" {
		t.Fatalf("unexpected raster meta path: %s", exported.RasterMetaPath)
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
			"LrDay":    round6(output.Indicators.LrDay),
			"LrNight":  round6(output.Indicators.LrNight),
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
		t.Fatal("resolve schall03 test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}
