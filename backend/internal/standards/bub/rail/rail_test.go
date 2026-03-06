package rail

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestDescriptorValidates(t *testing.T) {
	t.Parallel()

	err := Descriptor().Validate()
	if err != nil {
		t.Fatalf("descriptor should validate: %v", err)
	}
}

func TestComputeReceiverOutputs(t *testing.T) {
	t.Parallel()

	outputs, err := ComputeReceiverOutputs(
		[]geo.PointReceiver{{ID: "r-1", Point: geo.Point2D{X: 0, Y: 20}, HeightM: 4}},
		[]RailSource{sampleSource()},
		DefaultPropagationConfig(),
	)
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	if len(outputs) != 1 || outputs[0].Indicators.Lden <= 0 {
		t.Fatalf("unexpected outputs: %#v", outputs)
	}
}

func TestExportResultBundle(t *testing.T) {
	t.Parallel()

	outputs, err := ComputeReceiverOutputs(
		[]geo.PointReceiver{
			{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
			{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
			{ID: "r3", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
			{ID: "r4", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4},
		},
		[]RailSource{sampleSource()},
		DefaultPropagationConfig(),
	)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	exported, err := ExportResultBundle(t.TempDir(), outputs, 2, 2)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected exported file %s: %v", path, err)
		}
	}

	if filepath.Base(exported.RasterMetaPath) != StandardID+".json" {
		t.Fatalf("expected raster metadata name %s.json, got %s", StandardID, filepath.Base(exported.RasterMetaPath))
	}
}

func sampleSource() RailSource {
	return RailSource{
		ID:                   "rail-1",
		TrackCenterline:      []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		TractionType:         TractionElectric,
		TrackRoughnessClass:  RoughnessStandard,
		AverageTrainSpeedKPH: 90,
		BrakingShare:         0.1,
		CurveRadiusM:         500,
		TrafficDay:           TrafficPeriod{TrainsPerHour: 12},
		TrafficEvening:       TrafficPeriod{TrainsPerHour: 6},
		TrafficNight:         TrafficPeriod{TrainsPerHour: 4},
	}
}
