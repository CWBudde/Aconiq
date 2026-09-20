package road

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func sampleReceivers() []geo.PointReceiver {
	return []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 25, Y: 80}, HeightM: 6},
		{ID: "r3", Point: geo.Point2D{X: -30, Y: 120}, HeightM: 2.8},
	}
}

// --- refusals -------------------------------------------------------------
//
// Every check below used to sit on the per-receiver path, which re-derived the
// scene on each iteration. Hoisting the scene out of the loop can only keep
// them firing where they fired before if that is pinned down, so it is.

func TestComputeReceiverOutputs_NoReceivers(t *testing.T) {
	t.Parallel()

	_, err := ComputeReceiverOutputs(nil, []RoadSource{sampleSource()}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for an empty receiver list")
	}

	if !strings.Contains(err.Error(), "at least one receiver") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeReceiverOutputs_EmptyReceiverID(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{{ID: "", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4}}

	_, err := ComputeReceiverOutputs(receivers, []RoadSource{sampleSource()}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for an empty receiver id")
	}

	if !strings.Contains(err.Error(), "receiver id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeReceiverOutputs_NonFiniteReceiver(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: math.NaN(), Y: 40}, HeightM: 4}}

	_, err := ComputeReceiverOutputs(receivers, []RoadSource{sampleSource()}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for non-finite receiver coordinates")
	}

	if !strings.Contains(err.Error(), "not finite") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestComputeReceiverOutputs_InvalidReceiverHeight covers the one config field
// the receiver overrides: a bad height on the receiver must still be refused,
// even though the scene is now prepared from a config that carries it too.
func TestComputeReceiverOutputs_InvalidReceiverHeight(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: 0, Y: 40}, HeightM: -1}}

	_, err := ComputeReceiverOutputs(receivers, []RoadSource{sampleSource()}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for a negative receiver height")
	}

	if !strings.Contains(err.Error(), "receiver_height_m") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeReceiverOutputs_InvalidConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = 0

	_, err := ComputeReceiverOutputs(sampleReceivers(), []RoadSource{sampleSource()}, nil, cfg)
	if err == nil {
		t.Fatal("expected error for a zero segment length")
	}

	if !strings.Contains(err.Error(), "segment_length_m") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeReceiverOutputs_NoSources(t *testing.T) {
	t.Parallel()

	_, err := ComputeReceiverOutputs(sampleReceivers(), nil, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for a model with no source at all")
	}

	if !strings.Contains(err.Error(), "at least one road source or parking source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeReceiverOutputs_InvalidSource(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ID = ""

	_, err := ComputeReceiverOutputs(sampleReceivers(), []RoadSource{source}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for an invalid source")
	}
}

// TestComputeReceiverOutputs_ReceiverRefusalPrecedesSourceRefusal pins the
// order the two refusals come in. The receiver list is walked first and the
// scene is prepared only once a receiver has cleared its own checks, so a model
// that is wrong in both ways reports the receiver, as it did when every
// receiver re-prepared the scene.
func TestComputeReceiverOutputs_ReceiverRefusalPrecedesSourceRefusal(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ID = ""

	receivers := []geo.PointReceiver{{ID: "", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4}}

	_, err := ComputeReceiverOutputs(receivers, []RoadSource{source}, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "receiver id is required") {
		t.Fatalf("expected the receiver refusal first, got: %v", err)
	}
}

func TestComputeReceiverLevels_NonFiniteReceiver(t *testing.T) {
	t.Parallel()

	_, err := ComputeReceiverLevels(
		geo.Point2D{X: 0, Y: math.Inf(1)},
		[]RoadSource{sampleSource()}, nil, DefaultPropagationConfig(),
	)
	if err == nil {
		t.Fatal("expected error for a non-finite receiver")
	}
}

func TestComputeReceiverLevels_InvalidConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.MinDistanceM = 0

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 40}, []RoadSource{sampleSource()}, nil, cfg)
	if err == nil {
		t.Fatal("expected error for a zero minimum distance")
	}

	if !strings.Contains(err.Error(), "min_distance_m") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestComputeReceiverLevels_ConfigRefusalPrecedesSourceRefusal is the
// single-receiver half of the ordering above: the config is checked before the
// scene is prepared, so an invalid config outranks an invalid source.
func TestComputeReceiverLevels_ConfigRefusalPrecedesSourceRefusal(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ID = ""

	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = 0

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 40}, []RoadSource{source}, nil, cfg)
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "segment_length_m") {
		t.Fatalf("expected the config refusal first, got: %v", err)
	}
}

func TestComputeReceiverLevels_NoSources(t *testing.T) {
	t.Parallel()

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 40}, nil, nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for a model with no source at all")
	}

	if !strings.Contains(err.Error(), "at least one road source or parking source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrepareScene_RefusesInvalidConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = math.NaN()

	_, err := PrepareScene([]RoadSource{sampleSource()}, nil, cfg)
	if err == nil {
		t.Fatal("expected error for a non-finite receiver terrain datum")
	}
}

func TestScene_ComputeReceivers_NoReceivers(t *testing.T) {
	t.Parallel()

	scene, err := PrepareScene([]RoadSource{sampleSource()}, nil, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("PrepareScene: %v", err)
	}

	_, err = scene.ComputeReceivers(nil, DefaultPropagationConfig())
	if err == nil {
		t.Fatal("expected error for an empty receiver list")
	}
}

// --- equivalence ----------------------------------------------------------

// TestScenePathMatchesPerReceiverPath is the guard on the hoist itself. The
// scene holds only receiver-independent state, so preparing it once must give
// the same levels to the last bit as deriving it per receiver did — not "within
// 0.05 dB", identically. Anything else is a bug in the hoist.
func TestScenePathMatchesPerReceiverPath(t *testing.T) {
	t.Parallel()

	sources := []RoadSource{sampleSource(), risingSource()}
	barriers := []Barrier{{
		ID:       "wall",
		Geometry: []geo.Point2D{{X: -60, Y: 20}, {X: 60, Y: 20}},
		HeightM:  4,
	}}

	cfg := DefaultPropagationConfig()
	cfg.Buildings = []Building{{
		ID: "house",
		Footprint: []geo.Point2D{
			{X: 10, Y: 55}, {X: 30, Y: 55}, {X: 30, Y: 70}, {X: 10, Y: 70},
		},
		HeightM: 12,
	}}
	cfg.ParkingSources = []ParkingSource{{
		ID:                     "lot",
		Center:                 geo.Point2D{X: -20, Y: 30},
		AreaM2:                 625,
		NumSpaces:              50,
		LotType:                ParkingLotPkw,
		MovementsPerSpaceDay:   MovementRate(0.3),
		MovementsPerSpaceNight: MovementRate(0.06),
	}}

	receivers := sampleReceivers()

	want, err := ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverOutputs: %v", err)
	}

	scene, err := PrepareScene(sources, barriers, cfg)
	if err != nil {
		t.Fatalf("PrepareScene: %v", err)
	}

	got, err := scene.ComputeReceivers(receivers, cfg)
	if err != nil {
		t.Fatalf("Scene.ComputeReceivers: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("output count = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].Indicators != want[i].Indicators {
			t.Errorf("receiver %q: scene path %+v, per-receiver path %+v",
				want[i].Receiver.ID, got[i].Indicators, want[i].Indicators)
		}

		// The single-receiver entry point must agree with both, bit for bit.
		receiverCfg := cfg
		receiverCfg.ReceiverHeightM = receivers[i].HeightM

		levels, err := ComputeReceiverLevels(receivers[i].Point, sources, barriers, receiverCfg)
		if err != nil {
			t.Fatalf("ComputeReceiverLevels: %v", err)
		}

		if levels.ToReceiverIndicators() != want[i].Indicators {
			t.Errorf("receiver %q: single-receiver path %+v, grid path %+v",
				want[i].Receiver.ID, levels, want[i].Indicators)
		}
	}
}

// risingSource is a second source with per-vertex elevations and an automatic
// source-line offset, so the equivalence test covers the two pieces of
// per-source preparation the hoist moved: the elevation fill and
// EffectiveCenterline.
func risingSource() RoadSource {
	source := sampleSource()
	source.ID = "road-2"
	source.LaneCount = 4
	source.Centerline = []geo.Point2D{{X: -40, Y: -10}, {X: 0, Y: -5}, {X: 40, Y: -20}}
	source.CenterlineElevations = []float64{0, 3, 8}

	return source
}
