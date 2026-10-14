package road

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// progressCall is one (done, total) pair a progress callback was handed.
type progressCall struct{ done, total int }

func progressReceivers() []geo.PointReceiver {
	return []geo.PointReceiver{
		{ID: "r-1", Point: geo.Point2D{X: 10, Y: 20}, HeightM: 4},
		{ID: "r-2", Point: geo.Point2D{X: 0, Y: 50}, HeightM: 4},
		{ID: "r-3", Point: geo.Point2D{X: 35, Y: 120}, HeightM: 4},
	}
}

func TestComputeReceiverOutputsProgressMatchesContextAndReportsEachReceiver(t *testing.T) {
	t.Parallel()

	sources := []RoadSource{sampleSource()}

	want, err := ComputeReceiverOutputsContext(t.Context(), progressReceivers(), sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("ComputeReceiverOutputsContext: %v", err)
	}

	var calls []progressCall

	got, err := ComputeReceiverOutputsProgress(t.Context(), progressReceivers(), sources, DefaultPropagationConfig(),
		func(done, total int) { calls = append(calls, progressCall{done, total}) })
	if err != nil {
		t.Fatalf("ComputeReceiverOutputsProgress: %v", err)
	}

	// The report reads nothing the walk computes, so the levels must be the
	// ones the call without a callback returns, bit for bit.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outputs differ from ComputeReceiverOutputsContext:\n got %#v\nwant %#v", got, want)
	}

	wantCalls := []progressCall{{1, 3}, {2, 3}, {3, 3}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("progress calls: got %v want %v", calls, wantCalls)
	}
}

func TestComputeReceiverOutputsProgressStopsOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var calls []progressCall

	// Cancelling from the first report must stop the walk before the second
	// receiver, with no outputs and no further report.
	outputs, err := ComputeReceiverOutputsProgress(ctx, progressReceivers(), []RoadSource{sampleSource()}, DefaultPropagationConfig(),
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
