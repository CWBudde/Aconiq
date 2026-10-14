package road_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
	road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// expensiveReceiverScene is one receiver whose walk is long enough to cancel
// in the middle of: a 1.5 km road at 0.2 m Teilstücke along a row of buildings,
// so every Teilstück runs the reflection search. It is the shape of the Linden
// extract, where one receiver is minutes of CPU, scaled down to a fraction of
// a second.
func expensiveReceiverScene() ([]geo.PointReceiver, []road.RoadSource, road.PropagationConfig) {
	sources := []road.RoadSource{{
		ID:          "road",
		Centerline:  []geo.Point2D{{X: 0, Y: 0}, {X: 1500, Y: 0}},
		SurfaceType: road.SurfaceSMA,
		LaneCount:   2,
		Speeds:      road.SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50},
		TrafficDay:  road.TrafficInput{PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 55, KradPerHour: 10},
		TrafficNight: road.TrafficInput{
			PkwPerHour: 220, Lkw1PerHour: 10, Lkw2PerHour: 18, KradPerHour: 2,
		},
	}}

	cfg := road.DefaultPropagationConfig()
	cfg.SegmentLengthM = 0.2

	for i := range 40 {
		x := float64(i) * 37

		for _, y := range []float64{20, -32} {
			cfg.Buildings = append(cfg.Buildings, road.Building{
				ID: fmt.Sprintf("b-%d-%.0f", i, y),
				Footprint: []geo.Point2D{
					{X: x, Y: y}, {X: x + 14, Y: y}, {X: x + 14, Y: y + 12}, {X: x, Y: y + 12},
				},
				HeightM: 12,
			})
		}
	}

	receivers := []geo.PointReceiver{{ID: "r-0", Point: geo.Point2D{X: 750, Y: 8}, HeightM: 4}}

	return receivers, sources, cfg
}

// A cancel must not wait for the receiver in flight to finish. The walk used
// to check its context only between receivers, so on a building-dense scene a
// SIGTERM, a Ctrl-C or the API's cancel waited for minutes of reflection
// search — over an hour once, on a loaded machine.
//
// The bound is relative, because absolute times depend on the machine and on
// -race: the uncancelled receiver is timed first, then the same receiver is
// cancelled a tenth of the way in and must come back long before the walk
// could have finished. The pre-receiver check cannot pass this — the
// context is still live when the receiver starts.
func TestCancellationStopsInsideASingleExpensiveReceiver(t *testing.T) {
	t.Parallel()

	receivers, sources, cfg := expensiveReceiverScene()

	started := time.Now()

	want, err := road.ComputeReceiverOutputs(receivers, sources, nil, cfg)
	if err != nil {
		t.Fatalf("uncancelled walk: %v", err)
	}

	full := time.Since(started)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	timer := time.AfterFunc(full/10, cancel)
	defer timer.Stop()

	started = time.Now()

	_, err = road.ComputeReceiverOutputsContext(ctx, receivers, sources, nil, cfg)

	took := time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the walk to be cancelled, got %v (uncancelled walk took %s)", err, full)
	}

	if took > full/2 {
		t.Fatalf("cancelled walk took %s against %s for the whole receiver: the stop waited for the receiver", took, full)
	}

	// And the check that made that possible changes nothing when it does not
	// fire: the same receiver under a live context is the same bits.
	got, err := road.ComputeReceiverOutputsContext(t.Context(), receivers, sources, nil, cfg)
	if err != nil {
		t.Fatalf("walk under a live context: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatal("observing a context moved a level")
	}
}

// The pool reports a stop inside a receiver as the caller's cancellation, as it
// does one between receivers, at every worker count.
func TestParallelCancellationStopsInsideAReceiver(t *testing.T) {
	t.Parallel()

	receivers, sources, cfg := expensiveReceiverScene()

	// Several copies of the expensive receiver, so the pool has chunks.
	many := make([]geo.PointReceiver, 0, 8)

	for i := range 8 {
		r := receivers[0]
		r.ID = fmt.Sprintf("r-%d", i)
		many = append(many, r)
	}

	ctx, cancel := context.WithCancel(t.Context())
	timer := time.AfterFunc(20*time.Millisecond, cancel)

	defer timer.Stop()

	_, err := road.ComputeReceiverOutputsParallel(ctx, many, sources, nil, cfg, 4)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the caller's cancellation, got %v", err)
	}
}
