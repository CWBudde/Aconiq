package schall03

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

// countingTerrain counts the elevations a walk asks it for and, when cancel is
// set, cancels its context at the first of them — so a walk is cancelled from
// inside its first receiver, the shape Ctrl-C has during a long grid, rather
// than a context that was dead on arrival. Every subsegment path samples the
// DTM, so the count is a measure of how far the walk got.
type countingTerrain struct {
	profileTerrain

	calls  *int
	cancel context.CancelFunc
}

func (c countingTerrain) ElevationAt(x, y float64) (float64, bool) {
	*c.calls++

	if c.cancel != nil {
		c.cancel()
	}

	return c.profileTerrain.ElevationAt(x, y)
}

func flatProfileTerrain() profileTerrain {
	return profileTerrain{
		bounds: [4]float64{-5000, -5000, 5000, 5000},
		z:      func(float64, float64) float64 { return 0 },
	}
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

// Cancelled while the first receiver is being computed: the walk stops inside
// that receiver, at the next subsegment, rather than finishing it. One
// receiver over a large scene can outlast the API's grace period for an
// interrupted run, and a check only between receivers then left the server
// to kill the process instead of letting it record its own stop.
//
// How far the walk got is read off the DTM: every subsegment path samples it,
// so a walk that stopped at the next subsegment asked for a small fraction of
// the elevations the whole receiver does.
func TestComputeNormativeReceiverOutputsForSceneContextStopsInsideAReceiver(t *testing.T) {
	t.Parallel()

	receivers, scene := cancellationScene(t)
	receivers = receivers[:1]
	// 2 km of track: two hundred subsegments for the one receiver.
	scene.Segments[0].TrackCenterline = []geo.Point2D{{X: -1000, Y: 0}, {X: 1000, Y: 0}}

	fullCalls := 0
	scene.Terrain = countingTerrain{profileTerrain: flatProfileTerrain(), calls: &fullCalls}

	_, err := ComputeNormativeReceiverOutputsForSceneContext(t.Context(), receivers, scene)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cancelledCalls := 0
	scene.Terrain = countingTerrain{profileTerrain: flatProfileTerrain(), calls: &cancelledCalls, cancel: cancel}

	outputs, err := ComputeNormativeReceiverOutputsForSceneContext(ctx, receivers, scene)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled from inside the only receiver", err)
	}

	if !strings.Contains(err.Error(), `"r-1"`) {
		t.Fatalf("err = %v, want the walk to name the receiver it abandoned", err)
	}

	if outputs != nil {
		t.Fatalf("a cancelled run returned %d receivers; it must return none", len(outputs))
	}

	if cancelledCalls*20 > fullCalls {
		t.Fatalf("the cancelled walk sampled the DTM %d times of the receiver's %d: it did not stop at the next subsegment", cancelledCalls, fullCalls)
	}
}

// expensiveNormativeScene is one receiver over a scene whose single receiver
// costs seconds: 100 km of track carrying ten train types, each walked once
// per period, and every 10 m subsegment of it searched for reflections off a
// thousand facades along the line. It is the shape the grace period cannot
// absorb: no receiver boundary for a check between receivers to fire on.
func expensiveNormativeScene(t *testing.T) ([]ReceiverInput, NormativeScene) {
	t.Helper()

	receivers, scene := cancellationScene(t)
	scene.Segments[0].TrackCenterline = []geo.Point2D{{X: -50_000, Y: 0}, {X: 50_000, Y: 0}}
	scene.Segments[0].Operations = slices.Repeat(scene.Segments[0].Operations, 10)

	for i := range 500 {
		x := float64(i)*200 - 50_000
		for _, y := range []float64{-40, 40} {
			scene.Walls = append(scene.Walls, ReflectingWall{
				A: geo.Point2D{X: x, Y: y}, B: geo.Point2D{X: x + 20, Y: y}, HeightM: 10, Surface: WallSurfaceHard,
			})
		}
	}

	return receivers[:1], scene
}

// The acceptance shape for the whole change: a deliberately expensive single
// receiver, cancelled shortly after it starts, returns at once — well within
// httpv1's runInterruptGrace of 10 s, so `aconiq serve` sees the child record
// its own stop instead of killing it.
func TestAnExpensiveNormativeReceiverStopsWellWithinTheGracePeriod(t *testing.T) {
	t.Parallel()

	receivers, scene := expensiveNormativeScene(t)

	assertStopsPromptly(t, func(ctx context.Context) error {
		_, err := ComputeNormativeReceiverOutputsForSceneContext(ctx, receivers, scene)

		return err
	})
}

func TestAnExpensivePreviewReceiverStopsWellWithinTheGracePeriod(t *testing.T) {
	t.Parallel()

	receivers, sources := cancellationPreviewSources()
	receivers = receivers[:1]

	// Forty thousand tracks, one receiver: the preview chain's unit of
	// work is a source, and there is no second receiver to stop before.
	template := sources[0]
	sources = make([]RailSource, 40_000)

	for i := range sources {
		sources[i] = template
		sources[i].ID = fmt.Sprintf("track-%d", i)
	}

	assertStopsPromptly(t, func(ctx context.Context) error {
		_, err := ComputeReceiverOutputsContext(ctx, receivers, sources, DefaultPropagationConfig())

		return err
	})
}

// assertStopsPromptly runs compute under a context cancelled 50 ms in, and
// requires it to return context.Canceled within a second. With one receiver
// there is no receiver boundary left to stop at: a walk that only checks
// between receivers finishes the receiver and returns no error at all.
func assertStopsPromptly(t *testing.T, compute func(context.Context) error) {
	t.Helper()

	const (
		cancelAfter = 50 * time.Millisecond
		// A tenth of httpv1's runInterruptGrace, and still generous: one
		// subsegment or one source is microseconds.
		budget = time.Second
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	timer := time.AfterFunc(cancelAfter, cancel)
	defer timer.Stop()

	started := time.Now()
	err := compute(ctx)
	elapsed := time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v after %v, want context.Canceled from inside the only receiver", err, elapsed)
	}

	if elapsed > cancelAfter+budget {
		t.Fatalf("the walk took %v to stop; it must stop within %v of the cancel", elapsed, budget)
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

// Both walks report (done, total) after every receiver, in order, and the
// report moves no level: the outputs are those of the walk that reports
// nothing.
func TestTheProgressWalksReportEveryReceiverAndMoveNoLevel(t *testing.T) {
	t.Parallel()

	receivers, scene := cancellationScene(t)

	var normativeReports []string

	got, err := ComputeNormativeReceiverOutputsForSceneProgress(t.Context(), receivers, scene, func(done, total int) {
		normativeReports = append(normativeReports, fmt.Sprintf("%d/%d", done, total))
	})
	if err != nil {
		t.Fatal(err)
	}

	want, err := ComputeNormativeReceiverOutputsForScene(receivers, scene)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) || fmt.Sprint(normativeReports) != "[1/3 2/3 3/3]" {
		t.Fatalf("normative: reports %v, outputs moved: %v", normativeReports, !reflect.DeepEqual(got, want))
	}

	previewReceivers, sources := cancellationPreviewSources()

	var previewReports []string

	gotPreview, err := ComputeReceiverOutputsWithDataPackProgress(
		t.Context(), previewReceivers, sources, DefaultPropagationConfig(), BuiltinDataPack(),
		func(done, total int) { previewReports = append(previewReports, fmt.Sprintf("%d/%d", done, total)) },
	)
	if err != nil {
		t.Fatal(err)
	}

	wantPreview, err := ComputeReceiverOutputs(previewReceivers, sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(gotPreview, wantPreview) || fmt.Sprint(previewReports) != "[1/2 2/2]" {
		t.Fatalf("preview: reports %v, outputs moved: %v", previewReports, !reflect.DeepEqual(gotPreview, wantPreview))
	}
}
