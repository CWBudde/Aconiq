package road

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
)

func TestRoadSourceValidate(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.SpeedKPH = 0

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid speed error")
	}
}

func TestEmissionIncreasesWithTraffic(t *testing.T) {
	t.Parallel()

	low := sampleSource()
	low.TrafficDay = TrafficPeriod{LightVehiclesPerHour: 200, HeavyVehiclesPerHour: 20}

	lowEmission, err := ComputeEmission(low)
	if err != nil {
		t.Fatalf("compute low emission: %v", err)
	}

	high := sampleSource()
	high.TrafficDay = TrafficPeriod{LightVehiclesPerHour: 1200, HeavyVehiclesPerHour: 150}

	highEmission, err := ComputeEmission(high)
	if err != nil {
		t.Fatalf("compute high emission: %v", err)
	}

	if highEmission.Lday <= lowEmission.Lday {
		t.Fatalf("expected higher traffic to increase Lday: low=%f high=%f", lowEmission.Lday, highEmission.Lday)
	}
}

func TestPropagationDecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 5}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.Lday <= far.Lday {
		t.Fatalf("expected near level > far level: near=%f far=%f", near.Lday, far.Lday)
	}
}

func TestLdenAggregation(t *testing.T) {
	t.Parallel()

	levels := PeriodLevels{
		Lday:     60,
		Levening: 60,
		Lnight:   60,
	}
	lden := ComputeLden(levels)

	expected := 66.39524300131856
	if math.Abs(lden-expected) > 1e-9 {
		t.Fatalf("unexpected Lden: got %f expected %f", lden, expected)
	}
}

func TestExportResultBundle(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
		{ID: "r4", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []RoadSource{sampleSource()}, cfg)
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, 2, 2)
	if err != nil {
		t.Fatalf("export bundle: %v", err)
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

	meta := raster.Metadata()
	if meta.Bands != 2 {
		t.Fatalf("expected 2 raster bands, got %d", meta.Bands)
	}

	if filepath.Base(exported.RasterMetaPath) != "cnossos-road.json" {
		t.Fatalf("unexpected raster metadata name: %s", exported.RasterMetaPath)
	}
}

func TestDescriptorValidates(t *testing.T) {
	t.Parallel()

	descriptor := Descriptor()

	err := descriptor.Validate()
	if err != nil {
		t.Fatalf("descriptor should validate: %v", err)
	}
}

func sampleSource() RoadSource {
	return RoadSource{
		ID:          "road-1",
		SurfaceType: SurfaceDenseAsphalt,
		SpeedKPH:    70,
		Centerline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		TrafficDay: TrafficPeriod{
			LightVehiclesPerHour: 900,
			HeavyVehiclesPerHour: 90,
		},
		TrafficEvening: TrafficPeriod{
			LightVehiclesPerHour: 500,
			HeavyVehiclesPerHour: 45,
		},
		TrafficNight: TrafficPeriod{
			LightVehiclesPerHour: 250,
			HeavyVehiclesPerHour: 30,
		},
	}
}
