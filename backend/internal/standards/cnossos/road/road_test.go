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

	source = sampleSource()
	source.RoadCategory = "unknown"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid road category error")
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

func TestEmissionUsesExpandedVehicleClasses(t *testing.T) {
	t.Parallel()

	base := sampleSource()
	base.TrafficDay = TrafficPeriod{}

	baseEmission, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base emission: %v", err)
	}

	expanded := sampleSource()
	expanded.TrafficDay = TrafficPeriod{
		MediumVehiclesPerHour:     80,
		PoweredTwoWheelersPerHour: 20,
	}

	expandedEmission, err := ComputeEmission(expanded)
	if err != nil {
		t.Fatalf("compute expanded emission: %v", err)
	}

	if expandedEmission.Lday <= baseEmission.Lday {
		t.Fatalf("expected added medium/PTW traffic to increase Lday: base=%f expanded=%f", baseEmission.Lday, expandedEmission.Lday)
	}
}

func TestEmissionUsesRoadContextCorrections(t *testing.T) {
	t.Parallel()

	base := sampleSource()
	base.JunctionType = JunctionNone
	base.StuddedTyreShare = 0
	base.TemperatureC = 20

	baseEmission, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base emission: %v", err)
	}

	contextual := sampleSource()
	contextual.JunctionType = JunctionTrafficLight
	contextual.JunctionDistanceM = 10
	contextual.StuddedTyreShare = 0.5
	contextual.TemperatureC = -5

	contextualEmission, err := ComputeEmission(contextual)
	if err != nil {
		t.Fatalf("compute contextual emission: %v", err)
	}

	if contextualEmission.Lday <= baseEmission.Lday {
		t.Fatalf("expected road context to increase Lday: base=%f contextual=%f", baseEmission.Lday, contextualEmission.Lday)
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

func TestAttenuationTermsExposePropagationComponents(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	terms := attenuationTerms(100, cfg)

	if math.Abs(terms.DistanceM-100) > 1e-9 {
		t.Fatalf("unexpected effective distance: %#v", terms)
	}

	if math.Abs(terms.GeometricDB-geometricDivergence(100)) > 1e-9 {
		t.Fatalf("unexpected geometric term: %#v", terms)
	}

	if math.Abs(terms.AirDB-airAbsorption(100, cfg)) > 1e-9 {
		t.Fatalf("unexpected air term: %#v", terms)
	}

	if terms.GroundDB != cfg.GroundAttenuationDB || terms.BarrierDB != cfg.BarrierAttenuationDB {
		t.Fatalf("unexpected fixed propagation terms: %#v", terms)
	}
}

func TestAttenuationUsesMinimumDistanceClamp(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	terms := attenuationTerms(0.5, cfg)

	if math.Abs(terms.DistanceM-cfg.MinDistanceM) > 1e-9 {
		t.Fatalf("expected min distance clamp, got %#v", terms)
	}
}

func TestBarrierAndGroundTermsIncreaseAttenuation(t *testing.T) {
	t.Parallel()

	base := attenuationTerms(50, PropagationConfig{
		AirAbsorptionDBPerKM: 0.7,
		GroundAttenuationDB:  0,
		BarrierAttenuationDB: 0,
		MinDistanceM:         3,
	})

	withBarrier := attenuationTerms(50, PropagationConfig{
		AirAbsorptionDBPerKM: 0.7,
		GroundAttenuationDB:  2,
		BarrierAttenuationDB: 4,
		MinDistanceM:         3,
	})

	if totalAttenuation(withBarrier) <= totalAttenuation(base) {
		t.Fatalf("expected barrier/ground to increase attenuation: base=%f with_barrier=%f", totalAttenuation(base), totalAttenuation(withBarrier))
	}
}

func TestDiscretizeLineSegmentUsesDeterministicMidpoints(t *testing.T) {
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

func TestProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"road_category":           CategoryUrbanLocal,
		"road_junction_type":      JunctionTrafficLight,
		"traffic_day_medium_vph":  "21",
		"traffic_day_ptw_vph":     "7",
		"road_studded_tyre_share": "0.15",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["key_parameter.road_category"] != CategoryUrbanLocal {
		t.Fatalf("expected road_category in provenance: %#v", metadata)
	}

	if metadata["key_parameter.traffic_day_medium_vph"] != "21" || metadata["key_parameter.traffic_day_ptw_vph"] != "7" {
		t.Fatalf("expected expanded traffic parameters in provenance: %#v", metadata)
	}
}

func sampleSource() RoadSource {
	return RoadSource{
		ID:           "road-1",
		RoadCategory: CategoryUrbanMajor,
		SurfaceType:  SurfaceDenseAsphalt,
		SpeedKPH:     70,
		Centerline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		JunctionType:      JunctionNone,
		JunctionDistanceM: 0,
		TemperatureC:      20,
		StuddedTyreShare:  0,
		TrafficDay: TrafficPeriod{
			LightVehiclesPerHour:      900,
			MediumVehiclesPerHour:     120,
			HeavyVehiclesPerHour:      90,
			PoweredTwoWheelersPerHour: 40,
		},
		TrafficEvening: TrafficPeriod{
			LightVehiclesPerHour:      500,
			MediumVehiclesPerHour:     60,
			HeavyVehiclesPerHour:      45,
			PoweredTwoWheelersPerHour: 20,
		},
		TrafficNight: TrafficPeriod{
			LightVehiclesPerHour:      250,
			MediumVehiclesPerHour:     30,
			HeavyVehiclesPerHour:      30,
			PoweredTwoWheelersPerHour: 5,
		},
	}
}
