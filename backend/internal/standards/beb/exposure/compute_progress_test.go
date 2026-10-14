package exposure

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
)

// progressCall is one (done, total) pair a progress callback was handed.
type progressCall struct{ done, total int }

// progressBuildings returns two buildings, so that under max-facade
// evaluation the level step computes more receivers than there are buildings
// and a count of buildings could not pass for a count of receivers.
func progressBuildings() []BuildingUnit {
	second := sampleBuilding()
	second.ID = "b-2"
	second.Footprint = [][]geo.Point2D{{
		{X: -40, Y: 20},
		{X: -25, Y: 20},
		{X: -25, Y: 32},
		{X: -40, Y: 32},
		{X: -40, Y: 20},
	}}

	return []BuildingUnit{sampleBuilding(), second}
}

func maxFacadeConfig(upstream string) ExposureConfig {
	cfg := DefaultExposureConfig()
	cfg.FacadeEvaluationMode = FacadeEvaluationMaxFacade
	cfg.UpstreamMappingStandard = upstream

	return cfg
}

// wantProgressCalls is the sequence a successful walk over the facade
// receivers prepareBuildings builds must report: one call per receiver.
func wantProgressCalls(t *testing.T, buildings []BuildingUnit, cfg ExposureConfig) []progressCall {
	t.Helper()

	_, receivers, err := prepareBuildings(buildings, 4, cfg.FacadeEvaluationMode)
	if err != nil {
		t.Fatalf("prepare buildings: %v", err)
	}

	if len(receivers) <= len(buildings) {
		t.Fatalf("expected more facade receivers than buildings, got %d for %d", len(receivers), len(buildings))
	}

	calls := make([]progressCall, 0, len(receivers))
	for done := 1; done <= len(receivers); done++ {
		calls = append(calls, progressCall{done, len(receivers)})
	}

	return calls
}

func TestComputeOutputsProgressMatchesContextAndReportsEachFacadeReceiver(t *testing.T) {
	t.Parallel()

	buildings := progressBuildings()
	cfg := maxFacadeConfig(UpstreamStandardBUBRoad)
	roads := []road.RoadSource{sampleRoad()}

	wantOutputs, wantSummary, err := ComputeOutputsContext(t.Context(), buildings, roads, cfg, road.DefaultPropagationConfig(), 4)
	if err != nil {
		t.Fatalf("ComputeOutputsContext: %v", err)
	}

	var calls []progressCall

	gotOutputs, gotSummary, err := ComputeOutputsProgress(t.Context(), buildings, roads, cfg, road.DefaultPropagationConfig(), 4,
		func(done, total int) { calls = append(calls, progressCall{done, total}) })
	if err != nil {
		t.Fatalf("ComputeOutputsProgress: %v", err)
	}

	if !reflect.DeepEqual(gotOutputs, wantOutputs) {
		t.Fatalf("outputs differ from ComputeOutputsContext:\n got %#v\nwant %#v", gotOutputs, wantOutputs)
	}

	if !reflect.DeepEqual(gotSummary, wantSummary) {
		t.Fatalf("summary differs from ComputeOutputsContext:\n got %#v\nwant %#v", gotSummary, wantSummary)
	}

	if want := wantProgressCalls(t, buildings, cfg); !reflect.DeepEqual(calls, want) {
		t.Fatalf("progress calls: got %v want %v", calls, want)
	}
}

func TestComputeOutputsFromAircraftProgressMatchesContextAndReportsEachFacadeReceiver(t *testing.T) {
	t.Parallel()

	buildings := progressBuildings()
	cfg := maxFacadeConfig(UpstreamStandardBUFAircraft)
	sources := []bufaircraft.AircraftSource{sampleAircraft()}

	wantOutputs, wantSummary, err := ComputeOutputsFromAircraftContext(t.Context(), buildings, sources, cfg, bufaircraft.DefaultPropagationConfig(), 4)
	if err != nil {
		t.Fatalf("ComputeOutputsFromAircraftContext: %v", err)
	}

	var calls []progressCall

	gotOutputs, gotSummary, err := ComputeOutputsFromAircraftProgress(t.Context(), buildings, sources, cfg, bufaircraft.DefaultPropagationConfig(), 4,
		func(done, total int) { calls = append(calls, progressCall{done, total}) })
	if err != nil {
		t.Fatalf("ComputeOutputsFromAircraftProgress: %v", err)
	}

	if !reflect.DeepEqual(gotOutputs, wantOutputs) {
		t.Fatalf("outputs differ from ComputeOutputsFromAircraftContext:\n got %#v\nwant %#v", gotOutputs, wantOutputs)
	}

	if !reflect.DeepEqual(gotSummary, wantSummary) {
		t.Fatalf("summary differs from ComputeOutputsFromAircraftContext:\n got %#v\nwant %#v", gotSummary, wantSummary)
	}

	if want := wantProgressCalls(t, buildings, cfg); !reflect.DeepEqual(calls, want) {
		t.Fatalf("progress calls: got %v want %v", calls, want)
	}
}

func TestComputeOutputsProgressStopsOnCancellation(t *testing.T) {
	t.Parallel()

	buildings := progressBuildings()
	total := len(wantProgressCalls(t, buildings, maxFacadeConfig(UpstreamStandardBUBRoad)))

	run := map[string]func(ctx context.Context, progress func(done, total int)) error{
		"road": func(ctx context.Context, progress func(done, total int)) error {
			_, _, err := ComputeOutputsProgress(ctx, buildings, []road.RoadSource{sampleRoad()},
				maxFacadeConfig(UpstreamStandardBUBRoad), road.DefaultPropagationConfig(), 4, progress)

			return err
		},
		"aircraft": func(ctx context.Context, progress func(done, total int)) error {
			_, _, err := ComputeOutputsFromAircraftProgress(ctx, buildings, []bufaircraft.AircraftSource{sampleAircraft()},
				maxFacadeConfig(UpstreamStandardBUFAircraft), bufaircraft.DefaultPropagationConfig(), 4, progress)

			return err
		},
	}

	for name, compute := range run {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var calls []progressCall

			// Cancelling from the first report must stop the level step before
			// the second facade receiver, and nothing must be reported after it.
			err := compute(ctx, func(done, total int) {
				calls = append(calls, progressCall{done, total})

				cancel()
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled, got %v", err)
			}

			want := []progressCall{{1, total}}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("progress calls: got %v want %v", calls, want)
			}
		})
	}
}
