package iso9613

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestComputeReceiverOutputsDeterministicPointScope(t *testing.T) {
	t.Parallel()

	sources := []PointSource{
		{
			ID:                      "s1",
			Point:                   geo.Point2D{X: 0, Y: 0},
			SourceHeightM:           10,
			SoundPowerLevelDB:       100,
			DirectivityCorrectionDB: 1,
		},
		{
			ID:                   "s2",
			Point:                geo.Point2D{X: 30, Y: 0},
			SourceHeightM:        5,
			SoundPowerLevelDB:    96,
			TonalityCorrectionDB: 2,
		},
	}

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 20, Y: 10}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}

	if outputs[0].Indicators.LpAeq <= outputs[1].Indicators.LpAeq {
		t.Fatalf("expected receiver r1 closer to dominant source than r2: %#v", outputs)
	}

	outputsAgain, err := ComputeReceiverOutputs(receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs again: %v", err)
	}

	if outputs[0].Indicators != outputsAgain[0].Indicators || outputs[1].Indicators != outputsAgain[1].Indicators {
		t.Fatalf("expected deterministic outputs, got %#v and %#v", outputs, outputsAgain)
	}
}

func TestExportResultBundleWritesExpectedFiles(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	outputs := []ReceiverOutput{
		{
			Receiver:   geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{LpAeq: 55.2},
		},
		{
			Receiver:   geo.PointReceiver{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{LpAeq: 49.8},
		},
	}

	exported, err := ExportResultBundle(baseDir, outputs, 2, 1)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{
		exported.ReceiverJSONPath,
		exported.ReceiverCSVPath,
		exported.RasterMetaPath,
		exported.RasterDataPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected output file %s: %v", filepath.Base(path), err)
		}
	}
}

func TestDescriptorRejectsInvalidParameterValues(t *testing.T) {
	t.Parallel()

	resolved, err := Descriptor().ResolveVersionProfile("", "")
	if err != nil {
		t.Fatalf("resolve descriptor: %v", err)
	}

	_, err = resolved.RunParameterSchema.NormalizeAndValidate(map[string]string{
		"ground_factor": "1.5",
	})
	if err == nil || !strings.Contains(err.Error(), "ground_factor") {
		t.Fatalf("expected ground_factor validation error, got %v", err)
	}

	_, err = resolved.RunParameterSchema.NormalizeAndValidate(map[string]string{
		"meteorology_assumption": "unsupported",
	})
	if err == nil || !strings.Contains(err.Error(), "meteorology_assumption") {
		t.Fatalf("expected meteorology_assumption validation error, got %v", err)
	}
}

func TestValidationRejectsInvalidTypedInputs(t *testing.T) {
	t.Parallel()

	err := (PointSource{}).Validate()
	if err == nil {
		t.Fatal("expected empty point source to fail validation")
	}

	err = (Receiver{}).Validate()
	if err == nil {
		t.Fatal("expected empty receiver to fail validation")
	}

	err = (GroundZone{ID: "g1", Polygon: [][]geo.Point2D{{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 0}}}, GroundFactor: 2}).Validate()
	if err == nil {
		t.Fatal("expected invalid ground zone to fail validation")
	}

	err = (Meteorology{Assumption: "bad", TemperatureC: 10, RelativeHumidityPercent: 70}).Validate()
	if err == nil {
		t.Fatal("expected invalid meteorology to fail validation")
	}

	err = (PropagationConfig{GroundFactor: -1, AirTemperatureC: 10, RelativeHumidityPercent: 70, MeteorologyAssumption: MeteorologyDownwind, MinDistanceM: 1}).Validate()
	if err == nil {
		t.Fatal("expected invalid propagation config to fail validation")
	}
}

func TestComputeReceiverLevelRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	source := PointSource{
		ID:                "s1",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     5,
		SoundPowerLevelDB: 100,
	}

	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}

	if _, err := ComputeReceiverLevel(receiver, nil, DefaultPropagationConfig()); err == nil {
		t.Fatal("expected no-source compute to fail")
	}

	if _, err := ComputeReceiverLevel(geo.PointReceiver{}, []PointSource{source}, DefaultPropagationConfig()); err == nil {
		t.Fatal("expected invalid receiver to fail")
	}

	badSource := source

	badSource.ID = ""
	if _, err := ComputeReceiverLevel(receiver, []PointSource{badSource}, DefaultPropagationConfig()); err == nil {
		t.Fatal("expected invalid source to fail")
	}
}

func TestBarrierAttenuationLowersLevel(t *testing.T) {
	t.Parallel()

	source := PointSource{
		ID:                "s1",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     10,
		SoundPowerLevelDB: 100,
	}
	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 50, Y: 0}, HeightM: 4}

	baseLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute base level: %v", err)
	}

	cfg := DefaultPropagationConfig()
	cfg.BarrierAttenuationDB = 5

	barrierLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute barrier level: %v", err)
	}

	if barrierLevel >= baseLevel {
		t.Fatalf("expected barrier attenuation to reduce level: base=%v barrier=%v", baseLevel, barrierLevel)
	}
}

func TestMinDistanceClampKeepsCloseReceiverFinite(t *testing.T) {
	t.Parallel()

	source := PointSource{
		ID:                "s1",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     4,
		SoundPowerLevelDB: 95,
	}
	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}

	level, err := ComputeReceiverLevel(receiver, []PointSource{source}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute level: %v", err)
	}

	if level <= -900 {
		t.Fatalf("expected finite near-field level, got %v", level)
	}
}

func TestExportResultBundleRejectsShapeMismatch(t *testing.T) {
	t.Parallel()

	_, err := ExportResultBundle(t.TempDir(), []ReceiverOutput{
		{
			Receiver:   geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{LpAeq: 55},
		},
	}, 2, 1)
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("expected grid shape error, got %v", err)
	}
}
