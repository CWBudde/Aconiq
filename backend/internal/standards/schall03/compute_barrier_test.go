package schall03_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestNormativeReceiverWithBarrier(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5,
	}

	resultNoBarrier, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	barriers := []schall03.BarrierSegment{
		{
			A: geo.Point2D{X: -100, Y: 15}, B: geo.Point2D{X: 100, Y: 15},
			TopHeightM: 4, BaseHeightM: 0,
		},
	}

	resultWithBarrier, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, nil, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	if resultWithBarrier.LpAeqDay >= resultNoBarrier.LpAeqDay {
		t.Errorf("barrier should reduce level: without=%g, with=%g",
			resultNoBarrier.LpAeqDay, resultWithBarrier.LpAeqDay)
	}
}

func TestNormativeReceiverBarrierTooLow(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 10,
	}

	resultNoBarrier, err := schall03.ComputeNormativeReceiverLevels(receiver, []schall03.TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	// Very low barrier — below the line-of-sight for a receiver at 10 m height.
	barriers := []schall03.BarrierSegment{
		{
			A: geo.Point2D{X: -100, Y: 15}, B: geo.Point2D{X: 100, Y: 15},
			TopHeightM: 0.5, BaseHeightM: 0,
		},
	}

	resultWithBarrier, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, nil, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	diff := math.Abs(resultWithBarrier.LpAeqDay - resultNoBarrier.LpAeqDay)
	if diff > 0.01 {
		t.Errorf("low barrier should not change result: without=%g, with=%g, diff=%g",
			resultNoBarrier.LpAeqDay, resultWithBarrier.LpAeqDay, diff)
	}
}

func TestNormativeReceiverSceneWallAndBarrier(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5,
	}

	// Barrier between track and receiver, wall behind receiver.
	barriers := []schall03.BarrierSegment{
		{
			A: geo.Point2D{X: -100, Y: 15}, B: geo.Point2D{X: 100, Y: 15},
			TopHeightM: 4, BaseHeightM: 0,
		},
	}
	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: -100, Y: 40}, B: geo.Point2D{X: 100, Y: 40},
			HeightM: 15, Surface: schall03.WallSurfaceHard,
		},
	}

	resultBarrierOnly, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, nil, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	resultBoth, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, walls, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Adding a reflecting wall should increase the level even with a barrier.
	if resultBoth.LpAeqDay <= resultBarrierOnly.LpAeqDay {
		t.Errorf("wall should increase level even with barrier: barrier=%g, both=%g",
			resultBarrierOnly.LpAeqDay, resultBoth.LpAeqDay)
	}
}
