package schall03

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// cancellingTerrain cancels its context the first time a path asks it for an
// elevation, so a walk is cancelled from inside its first receiver — the shape
// Ctrl-C has during a long grid, rather than a context that was dead on
// arrival.
type cancellingTerrain struct {
	profileTerrain

	cancel context.CancelFunc
}

func (c cancellingTerrain) ElevationAt(x, y float64) (float64, bool) {
	c.cancel()

	return c.profileTerrain.ElevationAt(x, y)
}

func cancellationScene(t *testing.T) ([]ReceiverInput, NormativeScene) {
	t.Helper()

	op, err := NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	receivers := []ReceiverInput{
		{ID: "r-1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5},
		{ID: "r-2", Point: geo.Point2D{X: 20, Y: 60}, HeightM: 3.5},
		{ID: "r-3", Point: geo.Point2D{X: -40, Y: 90}, HeightM: 3.5},
	}

	return receivers, NormativeScene{Segments: []TrackSegment{{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		Fahrbahn:        FahrbahnartSchwellengleis,
		Surface:         SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []TrainOperation{*op},
	}}}
}

func cancellationPreviewSources() ([]geo.PointReceiver, []RailSource) {
	receivers := []geo.PointReceiver{
		{ID: "r-1", Point: geo.Point2D{X: 10, Y: 25}, HeightM: 4},
		{ID: "r-2", Point: geo.Point2D{X: 50, Y: 40}, HeightM: 4},
	}

	return receivers, []RailSource{{
		ID:              "track-1",
		TrackCenterline: []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}},
		TrainClass:      TrainClassPassenger,
		AverageSpeedKPH: 100,
		Infrastructure: RailInfrastructure{
			TractionType:        TractionElectric,
			TrackType:           TrackTypeSlab,
			TrackForm:           TrackFormSwitches,
			TrackRoughnessClass: RoughnessStandard,
		},
		TrafficDay:   TrafficPeriod{TrainsPerHour: 8},
		TrafficNight: TrafficPeriod{TrainsPerHour: 4},
	}}
}

func TestComputeNormativeReceiverOutputsForSceneContextHonoursACancelledContext(t *testing.T) {
	t.Parallel()

	receivers, scene := cancellationScene(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	outputs, err := ComputeNormativeReceiverOutputsForSceneContext(ctx, receivers, scene)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if outputs != nil {
		t.Fatalf("a cancelled run returned %d receivers; it must return none", len(outputs))
	}
}

// Cancelled while the first receiver is being computed: that receiver
// finishes, because the check sits between receivers and never inside the
// arithmetic, and the walk stops before the second.
func TestComputeNormativeReceiverOutputsForSceneContextStopsBetweenReceivers(t *testing.T) {
	t.Parallel()

	receivers, scene := cancellationScene(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	scene.Terrain = cancellingTerrain{
		profileTerrain: profileTerrain{
			bounds: [4]float64{-1000, -1000, 1000, 1000},
			z:      func(float64, float64) float64 { return 0 },
		},
		cancel: cancel,
	}

	outputs, err := ComputeNormativeReceiverOutputsForSceneContext(ctx, receivers, scene)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if !strings.Contains(err.Error(), `"r-2"`) {
		t.Fatalf("err = %v, want the walk to stop at the second receiver", err)
	}

	if outputs != nil {
		t.Fatalf("a cancelled run returned %d receivers; it must return none", len(outputs))
	}
}

func TestComputeNormativeReceiverOutputsForSceneContextMatchesWithoutCancellation(t *testing.T) {
	t.Parallel()

	receivers, scene := cancellationScene(t)

	want, err := ComputeNormativeReceiverOutputsForScene(receivers, scene)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ComputeNormativeReceiverOutputsForSceneContext(t.Context(), receivers, scene)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("a live context moved a result:\n got %#v\nwant %#v", got, want)
	}
}

func TestComputeReceiverOutputsContextHonoursACancelledContext(t *testing.T) {
	t.Parallel()

	receivers, sources := cancellationPreviewSources()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	outputs, err := ComputeReceiverOutputsContext(ctx, receivers, sources, DefaultPropagationConfig())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if outputs != nil {
		t.Fatalf("a cancelled run returned %d receivers; it must return none", len(outputs))
	}
}

func TestComputeReceiverOutputsContextMatchesWithoutCancellation(t *testing.T) {
	t.Parallel()

	receivers, sources := cancellationPreviewSources()

	want, err := ComputeReceiverOutputs(receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatal(err)
	}

	got, err := ComputeReceiverOutputsContext(t.Context(), receivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("a live context moved a result:\n got %#v\nwant %#v", got, want)
	}
}
