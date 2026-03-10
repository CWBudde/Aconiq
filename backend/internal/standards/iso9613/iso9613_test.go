package iso9613

import (
	"os"
	"path/filepath"
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
