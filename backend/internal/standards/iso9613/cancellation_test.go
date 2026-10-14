package iso9613

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

// expensiveReceiverScene is one receiver whose single walk costs seconds: two
// hundred thousand point sources around it, every path screened against a
// thousand barriers. A source→receiver path is the unit of ISO 9613-2's walk,
// and with one receiver there is no receiver boundary for a check between
// receivers to fire on.
func expensiveReceiverScene() ([]geo.PointReceiver, []PointSource, PropagationConfig) {
	receivers := []geo.PointReceiver{{ID: "r-1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}}

	sources := make([]PointSource, 200_000)
	for i := range sources {
		sources[i] = PointSource{
			ID:                fmt.Sprintf("s-%d", i),
			Point:             geo.Point2D{X: float64(i%400)*5 - 1000, Y: float64(i/400)*5 + 50},
			SourceHeightM:     2,
			SoundPowerLevelDB: 100,
		}
	}

	cfg := DefaultPropagationConfig()

	for i := range 1000 {
		x := float64(i%100)*20 - 1000
		y := float64(i/100)*3 + 10
		cfg.Barriers = append(cfg.Barriers, Barrier{
			ID:       fmt.Sprintf("b-%d", i),
			Geometry: []geo.Point2D{{X: x, Y: y}, {X: x + 15, Y: y}},
			HeightM:  6,
		})
	}

	return receivers, sources, cfg
}

// A deliberately expensive single receiver, cancelled 50 ms after it starts,
// returns at once — well within httpv1's runInterruptGrace of 10 s, so
// `aconiq serve` sees the child record its own stop instead of killing it. A
// walk that checks only between receivers finishes the one receiver and
// returns no error at all.
func TestAnExpensiveReceiverStopsWellWithinTheGracePeriod(t *testing.T) {
	t.Parallel()

	receivers, sources, cfg := expensiveReceiverScene()

	const (
		cancelAfter = 50 * time.Millisecond
		// A tenth of runInterruptGrace, and still generous: one path is
		// microseconds.
		budget = time.Second
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	timer := time.AfterFunc(cancelAfter, cancel)
	defer timer.Stop()

	started := time.Now()
	outputs, err := ComputeReceiverOutputsContext(ctx, receivers, sources, cfg)
	elapsed := time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v after %v, want context.Canceled from inside the only receiver", err, elapsed)
	}

	if !strings.Contains(err.Error(), `"r-1"`) {
		t.Fatalf("err = %v, want it to name the receiver it abandoned", err)
	}

	if outputs != nil {
		t.Fatalf("a cancelled walk returned %d receivers; it must return none", len(outputs))
	}

	if elapsed > cancelAfter+budget {
		t.Fatalf("the walk took %v to stop; it must stop within %v of the cancel", elapsed, budget)
	}
}

// The per-path check reads no number: under a live context, and through the
// exported entry point that has none, one receiver over the same scene comes
// out the same bits.
func TestThePerPathCheckMovesNoLevel(t *testing.T) {
	t.Parallel()

	receivers, sources, cfg := expensiveReceiverScene()
	sources = sources[:500]

	want, err := ComputeReceiverIndicators(receivers[0], sources, cfg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ComputeReceiverOutputsContext(t.Context(), receivers, sources, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if got[0].Indicators != want {
		t.Fatalf("a live context moved a level: got %#v, want %#v", got[0].Indicators, want)
	}
}
