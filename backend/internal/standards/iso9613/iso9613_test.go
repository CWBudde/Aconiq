package iso9613

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/results"
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

	if outputs[0].Indicators.LpAeqDW <= outputs[1].Indicators.LpAeqDW {
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

// A cancelled context stops the walk before the first receiver, and the error
// still names the receiver and carries context.Canceled for errors.Is.
func TestComputeReceiverOutputsContextStopsOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	sources := []PointSource{{ID: "s1", Point: geo.Point2D{X: 0, Y: 0}, SourceHeightM: 10, SoundPowerLevelDB: 100}}
	receivers := []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4}}

	outputs, err := ComputeReceiverOutputsContext(ctx, receivers, sources, DefaultPropagationConfig())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if outputs != nil {
		t.Fatalf("expected no outputs from a cancelled walk, got %d", len(outputs))
	}
}

// Under a live context the variant returns exactly what the context-free entry
// point returns: the check reads no number the walk computes.
func TestComputeReceiverOutputsContextMatchesWithoutContext(t *testing.T) {
	t.Parallel()

	sources := []PointSource{
		{ID: "s1", Point: geo.Point2D{X: 0, Y: 0}, SourceHeightM: 10, SoundPowerLevelDB: 100},
		{ID: "s2", Point: geo.Point2D{X: 30, Y: 0}, SourceHeightM: 5, SoundPowerLevelDB: 96},
	}
	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 20, Y: 10}, HeightM: 4},
	}

	want, err := ComputeReceiverOutputs(receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	got, err := ComputeReceiverOutputsContext(t.Context(), receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs under context: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d outputs, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("output %d: got %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestExportResultBundleWritesExpectedFiles(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	outputs := []ReceiverOutput{
		{
			Receiver:   geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{LpAeqDW: 55.2, LpAeqLT: 55.2},
		},
		{
			Receiver:   geo.PointReceiver{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
			Indicators: ReceiverIndicators{LpAeqDW: 49.8, LpAeqLT: 49.8},
		},
	}

	exported, err := ExportResultBundle(baseDir, outputs, results.GridLayout{Width: 2, Height: 1})
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{
		exported.ReceiverJSONPath,
		exported.ReceiverCSVPath,
		exported.RasterMetaPath,
		exported.RasterDataPath,
	} {
		_, err := os.Stat(path)
		if err != nil {
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

	_, err := ComputeReceiverLevel(receiver, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected no-source compute to fail")
	}

	_, err = ComputeReceiverLevel(geo.PointReceiver{}, []PointSource{source}, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected invalid receiver to fail")
	}

	badSource := source

	badSource.ID = ""

	_, err = ComputeReceiverLevel(receiver, []PointSource{badSource}, DefaultPropagationConfig())
	if err == nil {
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
	// Diffracted path 2*27 = 54 m over a direct distance of 50 m, i.e. z = 4 m.
	cfg.Barrier = &BarrierGeometry{
		Dss: 27,
		Dsr: 27,
		E:   0,
		A:   0,
		D:   50,
	}

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
			Indicators: ReceiverIndicators{LpAeqDW: 55, LpAeqLT: 55},
		},
	}, results.GridLayout{Width: 2, Height: 1})
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("expected grid shape error, got %v", err)
	}
}
