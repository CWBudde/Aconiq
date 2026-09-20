package wasmkernel_test

import (
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// sampleReceivers is a receiver line long enough that every chunk size under
// test lands differently: 23 is prime, so 1, 7, 256 and len() give four
// distinct partitions and none of them divides the list evenly except the
// trivial ones. Heights vary because ReceiverHeightM is the one config field a
// receiver overrides, and a chunked walk that forgot to carry it would still
// agree with itself.
func sampleReceivers() []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, 23)

	for i := range 23 {
		receivers = append(receivers, geo.PointReceiver{
			ID:      "r" + string(rune('a'+i)),
			Point:   geo.Point2D{X: float64(i)*7 - 70, Y: 30 + float64(i%5)*12},
			HeightM: 2 + float64(i%4),
		})
	}

	return receivers
}

func sampleSources() []road.RoadSource {
	return []road.RoadSource{{
		ID:          "road-1",
		SurfaceType: road.SurfaceSMA,
		Speeds: road.SpeedInput{
			PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100,
		},
		Centerline: []geo.Point2D{
			{X: -120, Y: 0},
			{X: -10, Y: 8},
			{X: 120, Y: 0},
		},
		TrafficDay: road.TrafficInput{
			PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 60, KradPerHour: 10,
		},
		TrafficNight: road.TrafficInput{
			PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 20, KradPerHour: 2,
		},
	}}
}

// A barrier is in the scene on purpose. Shielding is the part of the walk that
// reads the merged barrier set the scene holds, so a chunked walk that
// re-derived that set per chunk — or carried a stale one — would show up here
// and nowhere else.
func sampleBarriers() []road.Barrier {
	return []road.Barrier{{
		ID:       "wall",
		Geometry: []geo.Point2D{{X: -80, Y: 18}, {X: 80, Y: 18}},
		HeightM:  4,
	}}
}

// TestComputeRLS19RoadMatchesTheUnchunkedWalk is the determinism guard, and it
// is the reason this function may exist at all.
//
// `docs/policies/determinism.md` requires that the same inputs give identical
// outputs whatever the partitioning, and chunking is a partitioning even when
// nothing runs in parallel. The scene is receiver-independent and each receiver
// reads only it, so the chunk size can decide where the walk pauses and nothing
// else — which is a claim, until this test. Full struct equality, not a
// tolerance: a chunk boundary that moved a level by an ulp would be a bug in
// the chunking, not a rounding difference to be forgiven.
func TestComputeRLS19RoadMatchesTheUnchunkedWalk(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()
	sources := sampleSources()
	barriers := sampleBarriers()
	cfg := road.DefaultPropagationConfig()

	want, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverOutputs: %v", err)
	}

	if len(want) != len(receivers) {
		t.Fatalf("reference walk produced %d outputs, want %d", len(want), len(receivers))
	}

	// 0 is not a chunk size but the request to use DefaultChunkSize, and it is
	// the one every browser-mode run takes.
	for _, chunkSize := range []int{1, 7, 256, len(receivers), 0} {
		t.Run(chunkName(chunkSize), func(t *testing.T) {
			t.Parallel()

			got, err := wasmkernel.ComputeRLS19Road(
				receivers, sources, barriers, cfg, chunkSize, nil,
			)
			if err != nil {
				t.Fatalf("ComputeRLS19Road: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("chunk size %d does not reproduce the unchunked walk", chunkSize)
			}
		})
	}
}

func chunkName(chunkSize int) string {
	if chunkSize == 0 {
		return "default chunk size"
	}

	return "chunks of " + strconv.Itoa(chunkSize)
}

// A nil progress is the one-argument `aconiq.rls19Road` path, which is every
// call the app made before the worker landed. It must not be a special case
// the tests never take.
func TestComputeRLS19RoadAcceptsNoProgressListener(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()

	got, err := wasmkernel.ComputeRLS19Road(
		receivers, sampleSources(), nil, road.DefaultPropagationConfig(), 4, nil,
	)
	if err != nil {
		t.Fatalf("ComputeRLS19Road: %v", err)
	}

	if len(got) != len(receivers) {
		t.Fatalf("got %d outputs, want %d", len(got), len(receivers))
	}
}

// What a progress bar needs is not "some reports" but a sequence it can draw:
// strictly increasing, never past the end, and finishing exactly at it. A run
// whose last report says 22 of 23 leaves a bar stuck at 96 % forever, which is
// indistinguishable from a kernel that hung.
func TestComputeRLS19RoadReportsProgressToCompletion(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()

	for _, chunkSize := range []int{1, 7, 256, len(receivers)} {
		t.Run(chunkName(chunkSize), func(t *testing.T) {
			t.Parallel()

			type report struct{ done, total int }

			var reports []report

			_, err := wasmkernel.ComputeRLS19Road(
				receivers, sampleSources(), nil, road.DefaultPropagationConfig(), chunkSize,
				func(done, total int) {
					reports = append(reports, report{done: done, total: total})
				},
			)
			if err != nil {
				t.Fatalf("ComputeRLS19Road: %v", err)
			}

			if len(reports) == 0 {
				t.Fatal("the walk reported no progress at all")
			}

			previous := 0

			for i, got := range reports {
				if got.total != len(receivers) {
					t.Fatalf("report %d: total = %d, want %d", i, got.total, len(receivers))
				}

				if got.done <= previous {
					t.Fatalf("report %d: done = %d, which does not advance on %d", i, got.done, previous)
				}

				if got.done > got.total {
					t.Fatalf("report %d: done = %d, past the total %d", i, got.done, got.total)
				}

				previous = got.done
			}

			if previous != len(receivers) {
				t.Fatalf("the last report said %d of %d", previous, len(receivers))
			}

			wantReports := (len(receivers) + chunkSize - 1) / chunkSize
			if len(reports) != wantReports {
				t.Fatalf("got %d reports for chunk size %d, want %d", len(reports), chunkSize, wantReports)
			}
		})
	}
}

// TestComputeRLS19RoadRefusalsMatchTheUnchunkedWalk is the other half of the
// equivalence, and the easier one to get wrong.
//
// ComputeReceiverOutputs prepares its scene lazily, on the first receiver that
// has cleared its own checks, so a model that is wrong in two ways at once
// reports the receiver rather than the source. ComputeRLS19Road prepares
// eagerly — that is the saving it exists for — and so has to hand the wording
// back when the preparation fails. Comparing the full error text, not a
// substring, is what pins that: browser mode and `aconiq run` refuse the same
// model in the same sentence or a reader cannot compare them.
func TestComputeRLS19RoadRefusalsMatchTheUnchunkedWalk(t *testing.T) {
	t.Parallel()

	brokenSource := sampleSources()
	brokenSource[0].ID = ""

	invalidConfig := road.DefaultPropagationConfig()
	invalidConfig.SegmentLengthM = 0

	tests := map[string]struct {
		receivers []geo.PointReceiver
		sources   []road.RoadSource
		cfg       road.PropagationConfig
	}{
		"no receivers": {
			receivers: nil,
			sources:   sampleSources(),
			cfg:       road.DefaultPropagationConfig(),
		},
		"receiver without an id": {
			receivers: []geo.PointReceiver{{ID: "", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4}},
			sources:   sampleSources(),
			cfg:       road.DefaultPropagationConfig(),
		},
		"receiver with non-finite coordinates": {
			receivers: []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: math.NaN(), Y: 40}, HeightM: 4}},
			sources:   sampleSources(),
			cfg:       road.DefaultPropagationConfig(),
		},
		"receiver with a negative height": {
			receivers: []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: 0, Y: 40}, HeightM: -1}},
			sources:   sampleSources(),
			cfg:       road.DefaultPropagationConfig(),
		},
		// The second receiver is the interesting one: the scene is already
		// prepared by the time the walk reaches it, in both implementations.
		"a later receiver without an id": {
			receivers: []geo.PointReceiver{
				{ID: "r1", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4},
				{ID: "", Point: geo.Point2D{X: 10, Y: 40}, HeightM: 4},
			},
			sources: sampleSources(),
			cfg:     road.DefaultPropagationConfig(),
		},
		"no sources": {
			receivers: sampleReceivers(),
			sources:   nil,
			cfg:       road.DefaultPropagationConfig(),
		},
		"an invalid source": {
			receivers: sampleReceivers(),
			sources:   brokenSource,
			cfg:       road.DefaultPropagationConfig(),
		},
		"an invalid config": {
			receivers: sampleReceivers(),
			sources:   sampleSources(),
			cfg:       invalidConfig,
		},
		// Both wrong at once. This is the ordering the eager preparation could
		// silently invert, and the only case that catches it.
		"a receiver and a source, both without an id": {
			receivers: []geo.PointReceiver{{ID: "", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4}},
			sources:   brokenSource,
			cfg:       road.DefaultPropagationConfig(),
		},
		"no receivers and no sources": {
			receivers: nil,
			sources:   nil,
			cfg:       road.DefaultPropagationConfig(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, want := road.ComputeReceiverOutputs(test.receivers, test.sources, nil, test.cfg)
			if want == nil {
				t.Fatal("the reference walk accepted an input this table calls broken")
			}

			for _, chunkSize := range []int{1, 7, 0} {
				outputs, got := wasmkernel.ComputeRLS19Road(
					test.receivers, test.sources, nil, test.cfg, chunkSize, nil,
				)
				if got == nil {
					t.Fatalf("chunk size %d accepted the input; the unchunked walk refused it with %v",
						chunkSize, want)
				}

				if got.Error() != want.Error() {
					t.Fatalf("chunk size %d refused with %q, the unchunked walk with %q",
						chunkSize, got, want)
				}

				if outputs != nil {
					t.Fatalf("chunk size %d returned %d outputs beside its refusal",
						chunkSize, len(outputs))
				}
			}
		})
	}
}

// A refusal must not leave a progress bar claiming the run finished. The walk
// reports after a chunk completes, so a chunk that failed reports nothing —
// and with a chunk size of one, a failure at receiver two means exactly one
// report, for receiver one.
func TestComputeRLS19RoadStopsReportingWhenItRefuses(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 40}, HeightM: 4},
		{ID: "", Point: geo.Point2D{X: 10, Y: 40}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 20, Y: 40}, HeightM: 4},
	}

	var reports int

	_, err := wasmkernel.ComputeRLS19Road(
		receivers, sampleSources(), nil, road.DefaultPropagationConfig(), 1,
		func(_, _ int) { reports++ },
	)
	if err == nil {
		t.Fatal("expected a refusal for a receiver with no id")
	}

	if reports != 1 {
		t.Fatalf("got %d progress reports before the refusal, want 1", reports)
	}
}
