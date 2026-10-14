package wasmkernel_test

import (
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

func TestEstimateRLS19RoadPricesTheShardedRun(t *testing.T) {
	t.Parallel()

	receivers := sampleReceivers()

	estimate, err := wasmkernel.EstimateRLS19Road(
		receivers, sampleSources(), sampleBarriers(), road.DefaultPropagationConfig(), 4, 100*time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !estimate.Available || estimate.StandardID != road.StandardID {
		t.Fatalf("estimate %+v", estimate)
	}

	if estimate.Inputs.Receivers != len(receivers) || estimate.Inputs.Workers != 4 || estimate.Inputs.Units == 0 {
		t.Fatalf("inputs %+v", estimate.Inputs)
	}

	wall := estimate.WallSeconds
	if !(wall.Low > 0 && wall.Low <= wall.Estimate && wall.Estimate <= wall.High) {
		t.Fatalf("wall band %+v", wall)
	}

	// Fixed mode, so the distance-scaled run is offered beside it.
	if len(estimate.Alts) != 1 {
		t.Fatalf("alternatives %+v", estimate.Alts)
	}
}

func TestEstimateRLS19RoadRefusesAsTheRunRefuses(t *testing.T) {
	t.Parallel()

	_, want := wasmkernel.ComputeRLS19Road(nil, sampleSources(), sampleBarriers(), road.DefaultPropagationConfig(), 0, nil)

	_, err := wasmkernel.EstimateRLS19Road(nil, sampleSources(), sampleBarriers(), road.DefaultPropagationConfig(), 2, 0)
	if err == nil || want == nil || err.Error() != want.Error() {
		t.Fatalf("estimate refused with %v, the run with %v", err, want)
	}

	receivers := []geo.PointReceiver{{ID: "", Point: geo.Point2D{X: 1, Y: 1}, HeightM: 4}}

	_, want = wasmkernel.ComputeRLS19Road(receivers, sampleSources(), sampleBarriers(), road.DefaultPropagationConfig(), 0, nil)

	_, err = wasmkernel.EstimateRLS19Road(receivers, sampleSources(), sampleBarriers(), road.DefaultPropagationConfig(), 2, 0)
	if err == nil || want == nil || err.Error() != want.Error() {
		t.Fatalf("estimate refused with %v, the run with %v", err, want)
	}
}
