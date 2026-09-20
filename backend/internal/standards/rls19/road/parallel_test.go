package road_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// Worker counts chosen so none of them divides the receiver count: 1 is the
// delegating path, 2/3/4/8 are plausible machines, and 17 is more workers than
// chunks, which must not deadlock or produce an empty chunk.
var parallelWorkerCounts = []int{1, 2, 3, 4, 8, 17}

// parallelScene is deliberately the expensive shape rather than the convenient
// one. The Scene is shared read-only across goroutines, and the things it
// shares are the barrier grid and the reflector field — so a scene with no
// buildings would let -race pass without ever visiting either index, which is
// precisely where a data race would live.
func parallelScene() ([]road.RoadSource, []road.Barrier, road.PropagationConfig) {
	sources := []road.RoadSource{{
		ID: "road",
		Centerline: []geo.Point2D{
			{X: 0, Y: 0}, {X: 120, Y: 18}, {X: 240, Y: -12}, {X: 360, Y: 5},
		},
		SurfaceType: road.SurfaceSMA,
		LaneCount:   2,
		Speeds: road.SpeedInput{
			PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100,
		},
		TrafficDay: road.TrafficInput{
			PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 55, KradPerHour: 10,
		},
		TrafficNight: road.TrafficInput{
			PkwPerHour: 220, Lkw1PerHour: 10, Lkw2PerHour: 18, KradPerHour: 2,
		},
	}}

	barriers := []road.Barrier{{
		ID:      "wall",
		HeightM: 3.5,
		Geometry: []geo.Point2D{
			{X: -20, Y: 26}, {X: 400, Y: 26},
		},
	}}

	cfg := road.DefaultPropagationConfig()
	cfg.SegmentLengthM = 12

	// Buildings, so both derived sets are populated: a Building is a barrier
	// and a reflector at once, which is what puts the reflection search and
	// the shielding search on the shared indexes.
	for i := range 12 {
		x := float64(i) * 30

		cfg.Buildings = append(cfg.Buildings, road.Building{
			ID: fmt.Sprintf("b-%d", i),
			Footprint: []geo.Point2D{
				{X: x, Y: 34}, {X: x + 11, Y: 34}, {X: x + 11, Y: 47}, {X: x, Y: 47},
			},
			HeightM: 8,
		})
	}

	return sources, barriers, cfg
}

// parallelReceiverCount is prime, so no worker count under test divides it
// evenly and every run ends on a ragged chunk.
const parallelReceiverCount = 601

// parallelReceivers lays out a receiver grid of the given size.
func parallelReceivers(count int) []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, count)

	for i := range count {
		receivers = append(receivers, geo.PointReceiver{
			ID:      fmt.Sprintf("r-%d", i),
			Point:   geo.Point2D{X: float64(i%37) * 10, Y: 60 + float64(i/37)*7},
			HeightM: 4,
		})
	}

	return receivers
}

// The claim the whole parallel driver rests on, in the form spatial_test.go
// established for the spatial index: run it both ways and require the two to
// agree to the bit. reflect.DeepEqual over the full struct rather than a dB
// tolerance, because docs/policies/determinism.md does not have a tolerance in
// it — "same inputs, same outputs" here means the bits.
func TestComputeReceiverOutputsParallelMatchesTheSequentialWalk(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(parallelReceiverCount)

	want, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatalf("sequential walk failed: %v", err)
	}

	for _, workers := range parallelWorkerCounts {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()

			got, err := road.ComputeReceiverOutputsParallel(
				t.Context(), receivers, sources, barriers, cfg, workers,
			)
			if err != nil {
				t.Fatalf("parallel walk failed: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("workers=%d moved a result", workers)
			}
		})
	}
}

// docs/policies/determinism.md's "Determinism checks" asks, in as many words,
// for a test comparing output hashes for one worker against N. Until now
// RLS-19 had none, because RLS-19 never went through the engine that the
// clause was written for.
//
// Hashing the marshalled output rather than comparing structs is not a weaker
// check here: Go emits the shortest round-trippable decimal for a float64, so
// two levels that hash alike are the same bits.
func TestOneWorkerAndNWorkersHashAlike(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(parallelReceiverCount)

	hashes := make(map[string][]int, len(parallelWorkerCounts))

	for _, workers := range parallelWorkerCounts {
		outputs, err := road.ComputeReceiverOutputsParallel(
			t.Context(), receivers, sources, barriers, cfg, workers,
		)
		if err != nil {
			t.Fatalf("workers=%d: %v", workers, err)
		}

		encoded, err := json.Marshal(outputs)
		if err != nil {
			t.Fatalf("workers=%d: marshal: %v", workers, err)
		}

		sum := sha256.Sum256(encoded)
		key := hex.EncodeToString(sum[:])
		hashes[key] = append(hashes[key], workers)
	}

	if len(hashes) != 1 {
		t.Fatalf("worker counts disagreed about the output: %v", hashes)
	}
}

// A pool must not change which refusal a user reads. Chunks partition the
// receiver list in input order and a chunk is the sequential walk over its own
// slice, so the lowest-indexed failure is the sequential one — whichever
// goroutine reached it first.
func TestComputeReceiverOutputsParallelRefusesLikeTheSequentialWalk(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()

	// Two bad receivers, deep enough into the list that different worker
	// counts put them in different chunks, and the *second* one bad in a
	// different way — so a driver that reported whichever failure it found
	// first would be caught rather than accidentally right.
	spoil := func(at int, spoiler func(*geo.PointReceiver)) []geo.PointReceiver {
		receivers := parallelReceivers(parallelReceiverCount)
		spoiler(&receivers[at])

		return receivers
	}

	cases := map[string][]geo.PointReceiver{
		"missing id": spoil(300, func(r *geo.PointReceiver) { r.ID = "" }),
		"infinite coordinate": spoil(457, func(r *geo.PointReceiver) {
			r.Point.X = math.Inf(1)
		}),
		"first receiver bad": spoil(0, func(r *geo.PointReceiver) { r.ID = "" }),
		"two bad, the earlier one wins": func() []geo.PointReceiver {
			receivers := parallelReceivers(parallelReceiverCount)
			receivers[120].ID = ""
			receivers[480].HeightM = -1

			return receivers
		}(),
	}

	for name, receivers := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, want := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
			if want == nil {
				t.Fatal("the sequential walk accepted a receiver set this test needs it to refuse")
			}

			for _, workers := range parallelWorkerCounts {
				_, got := road.ComputeReceiverOutputsParallel(
					t.Context(), receivers, sources, barriers, cfg, workers,
				)
				if got == nil {
					t.Fatalf("workers=%d accepted what the sequential walk refused", workers)
				}

				if got.Error() != want.Error() {
					t.Fatalf("workers=%d refused with %q, sequential said %q",
						workers, got.Error(), want.Error())
				}
			}
		})
	}
}

// An empty receiver list is refused before anything is prepared, in the same
// words either way — the one refusal that precedes even the receiver checks.
func TestComputeReceiverOutputsParallelRefusesAnEmptyReceiverSet(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()

	_, want := road.ComputeReceiverOutputs(nil, sources, barriers, cfg)
	_, got := road.ComputeReceiverOutputsParallel(t.Context(), nil, sources, barriers, cfg, 8)

	if want == nil || got == nil || got.Error() != want.Error() {
		t.Fatalf("empty set: parallel said %v, sequential said %v", got, want)
	}
}

// A caller who gave up must hear that, and must never be handed the chunks
// that happened to finish first as though they were the whole run.
func TestComputeReceiverOutputsParallelHonoursACancelledContext(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(parallelReceiverCount)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	outputs, err := road.ComputeReceiverOutputsParallel(ctx, receivers, sources, barriers, cfg, 4)
	if err == nil {
		t.Fatalf("a cancelled context produced %d receivers and no error", len(outputs))
	}

	if outputs != nil {
		t.Fatalf("a cancelled run returned %d receivers; it must return none", len(outputs))
	}
}

// A run too small to be worth splitting must come out of the pool exactly as
// it goes into the sequential walk — the partition keeps it whole, and the
// driver then delegates rather than starting a goroutine to do nothing.
func TestComputeReceiverOutputsParallelLeavesASmallRunWhole(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()

	for _, count := range []int{1, 7, 49} {
		receivers := parallelReceivers(count)

		want, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
		if err != nil {
			t.Fatalf("%d receivers, sequential: %v", count, err)
		}

		got, err := road.ComputeReceiverOutputsParallel(
			t.Context(), receivers, sources, barriers, cfg, 8,
		)
		if err != nil {
			t.Fatalf("%d receivers, parallel: %v", count, err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%d receivers: the pool moved a result", count)
		}
	}
}

// The refusal a pool reports must not depend on which goroutine lost the
// race, and the dangerous shape is several bad receivers in different chunks:
// a high-index chunk can fail and cancel its siblings before a lower-index
// chunk has been dispatched, and the skipped chunk then records nothing.
//
// Be honest about what this test is. It pins the contract; it does not
// reproduce the race. Chunks are dispatched in index order, so the earliest
// bad receiver is almost always reached first anyway — checked, and this
// passes against the version without the up-front validation too. What closes
// the hole is structural rather than statistical: ValidateReceivers decides
// every receiver-shaped refusal before a chunk exists, so there is no
// scheduling for the answer to depend on. The repetition below is cheap
// insurance against that reasoning being wrong, not the guarantee itself.
func TestComputeReceiverOutputsParallelReportsTheEarliestRefusalUnderRacing(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()

	receivers := parallelReceivers(parallelReceiverCount)
	// Spread across chunks at every worker count under test, with the latest
	// one first in index order only by accident of nothing: 70 is what the
	// sequential walk reaches first and must therefore always be reported.
	receivers[70].ID = ""
	receivers[300].HeightM = -1
	receivers[590].Point.X = math.Inf(1)

	_, want := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if want == nil {
		t.Fatal("the sequential walk accepted a receiver set this test needs it to refuse")
	}

	for _, workers := range parallelWorkerCounts {
		for attempt := range 25 {
			_, got := road.ComputeReceiverOutputsParallel(
				t.Context(), receivers, sources, barriers, cfg, workers,
			)
			if got == nil {
				t.Fatalf("workers=%d attempt %d accepted what the sequential walk refused",
					workers, attempt)
			}

			if got.Error() != want.Error() {
				t.Fatalf("workers=%d attempt %d refused with %q, sequential said %q",
					workers, attempt, got.Error(), want.Error())
			}
		}
	}
}

// ValidateReceivers has to agree with the walk it stands in for, including on
// which of several bad receivers it names.
func TestValidateReceiversMatchesTheSequentialRefusal(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()

	spoil := func(spoilers map[int]func(*geo.PointReceiver)) []geo.PointReceiver {
		receivers := parallelReceivers(parallelReceiverCount)
		for at, spoiler := range spoilers {
			spoiler(&receivers[at])
		}

		return receivers
	}

	cases := map[string][]geo.PointReceiver{
		"clean": parallelReceivers(parallelReceiverCount),
		"missing id": spoil(map[int]func(*geo.PointReceiver){
			12: func(r *geo.PointReceiver) { r.ID = "" },
		}),
		"non-finite point": spoil(map[int]func(*geo.PointReceiver){
			400: func(r *geo.PointReceiver) { r.Point.Y = math.NaN() },
		}),
		"bad height": spoil(map[int]func(*geo.PointReceiver){
			7: func(r *geo.PointReceiver) { r.HeightM = -3 },
		}),
		"several, the earliest wins": spoil(map[int]func(*geo.PointReceiver){
			5:   func(r *geo.PointReceiver) { r.HeightM = -3 },
			200: func(r *geo.PointReceiver) { r.ID = "" },
		}),
	}

	for name, receivers := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, want := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
			got := road.ValidateReceivers(receivers, cfg)

			switch {
			case want == nil && got != nil:
				t.Fatalf("ValidateReceivers refused %q where the walk accepted", got)
			case want != nil && got == nil:
				t.Fatalf("ValidateReceivers accepted where the walk refused %q", want)
			case want != nil && got.Error() != want.Error():
				t.Fatalf("ValidateReceivers said %q, the walk said %q", got, want)
			}
		})
	}
}
