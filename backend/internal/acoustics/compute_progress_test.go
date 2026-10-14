package acoustics

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// progressCall is one (done, total) pair a progress callback was handed.
type progressCall struct{ done, total int }

func testPeriodLevels(receiver geo.PointReceiver, sources []testSource) (PeriodLevels, error) {
	base := sources[0].levelDB - receiver.Point.X/7

	return PeriodLevels{Lday: base, Levening: base - 4.3, Lnight: base - 9.1}, nil
}

func TestComputeReceiverOutputsProgressReportsEachReceiverAndMatchesTheWalk(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		finiteReceiver("a", 3),
		finiteReceiver("b", 11),
		finiteReceiver("c", 29),
	}
	sources := []testSource{{levelDB: 71.3}}

	want, err := ComputeReceiverOutputs(t.Context(), receivers, sources, testPeriodLevels)
	if err != nil {
		t.Fatalf("ComputeReceiverOutputs: %v", err)
	}

	var calls []progressCall

	got, err := ComputeReceiverOutputsProgress(t.Context(), receivers, sources, testPeriodLevels,
		func(done, total int) { calls = append(calls, progressCall{done, total}) })
	if err != nil {
		t.Fatalf("ComputeReceiverOutputsProgress: %v", err)
	}

	// Reporting reads nothing the walk computes, so the outputs must be the
	// very values the walk without a callback returns, bit for bit.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outputs differ from the walk without progress:\n got %#v\nwant %#v", got, want)
	}

	wantCalls := []progressCall{{1, 3}, {2, 3}, {3, 3}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("progress calls: got %v want %v", calls, wantCalls)
	}
}

func TestComputeReceiverOutputsProgressAcceptsANilCallback(t *testing.T) {
	t.Parallel()

	outputs, err := ComputeReceiverOutputsProgress(t.Context(),
		[]geo.PointReceiver{finiteReceiver("a", 1)}, []testSource{{levelDB: 60}}, testPeriodLevels, nil)
	if err != nil {
		t.Fatalf("ComputeReceiverOutputsProgress: %v", err)
	}

	if len(outputs) != 1 {
		t.Fatalf("unexpected output count: got %d want 1", len(outputs))
	}
}

func TestComputeReceiverOutputsProgressStopsReportingOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	receivers := []geo.PointReceiver{
		finiteReceiver("a", 1),
		finiteReceiver("b", 2),
		finiteReceiver("c", 3),
	}

	var calls []progressCall

	// The callback cancels once the first receiver is reported, so the
	// context check before the second receiver must end the walk there.
	outputs, err := ComputeReceiverOutputsProgress(ctx, receivers, []testSource{{levelDB: 60}}, testPeriodLevels,
		func(done, total int) {
			calls = append(calls, progressCall{done, total})

			cancel()
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if outputs != nil {
		t.Fatalf("expected no outputs, got %d", len(outputs))
	}

	wantCalls := []progressCall{{1, 3}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("progress calls: got %v want %v", calls, wantCalls)
	}
}

func TestComputeReceiverOutputsProgressReportsNothingForAFailedReceiver(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		finiteReceiver("a", 1),
		finiteReceiver("", 2),
		finiteReceiver("c", 3),
	}

	var calls []progressCall

	_, err := ComputeReceiverOutputsProgress(t.Context(), receivers, []testSource{{levelDB: 60}}, testPeriodLevels,
		func(done, total int) { calls = append(calls, progressCall{done, total}) })
	if err == nil {
		t.Fatal("expected an error for a receiver without an id")
	}

	wantCalls := []progressCall{{1, 3}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("progress calls: got %v want %v", calls, wantCalls)
	}
}
