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

func TestReflectedPathObstructedByBarrier(t *testing.T) {
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

	// Wall behind receiver reflects sound back.
	walls := []schall03.ReflectingWall{
		{
			A: geo.Point2D{X: -100, Y: 40}, B: geo.Point2D{X: 100, Y: 40},
			HeightM: 15, Surface: schall03.WallSurfaceHard,
		},
	}

	// Barrier between track and receiver obstructs both direct and reflected paths.
	barriers := []schall03.BarrierSegment{
		{
			A: geo.Point2D{X: -100, Y: 15}, B: geo.Point2D{X: 100, Y: 15},
			TopHeightM: 6, BaseHeightM: 0,
		},
	}

	// Walls only (no barriers) — reflected path is unobstructed.
	resultWallOnly, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, walls, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Walls + barriers — reflected path is obstructed by the barrier.
	resultBoth, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, walls, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The barrier should reduce the total level (affects both direct and reflected).
	if resultBoth.LpAeqDay >= resultWallOnly.LpAeqDay {
		t.Errorf("barrier should reduce level even with wall: wall=%g, both=%g",
			resultWallOnly.LpAeqDay, resultBoth.LpAeqDay)
	}
}

// A building footprint shields.  The assertion is a margin, not a bare ">": a
// defect that left buildings all but transparent and moved the level by a
// rounding step would satisfy an inequality.
func TestNormativeReceiverBehindBuildingFootprint(t *testing.T) {
	t.Parallel()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	seg := schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -200, Y: 0}, {X: 200, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 45}, HeightM: 3.5,
	}

	// A closed 40 m x 12 m, 9 m high footprint between track and receiver.
	ring := []geo.Point2D{
		{X: -20, Y: 15}, {X: 20, Y: 15}, {X: 20, Y: 27}, {X: -20, Y: 27}, {X: -20, Y: 15},
	}

	barriers := make([]schall03.BarrierSegment, 0, len(ring)-1)
	for i := range len(ring) - 1 {
		barriers = append(barriers, schall03.BarrierSegment{
			A: ring[i], B: ring[i+1], TopHeightM: 9, ObstacleID: "house-1",
		})
	}

	free, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	shielded, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		receiver, []schall03.TrackSegment{seg}, nil, barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	const wantInsertionLossDB = 5.0

	got := free.LpAeqDay - shielded.LpAeqDay
	if got < wantInsertionLossDB {
		t.Errorf("building insertion loss = %.2f dB, want at least %.2f dB (free=%.2f, shielded=%.2f)",
			got, wantInsertionLossDB, free.LpAeqDay, shielded.LpAeqDay)
	}

	if shielded.LrNight >= free.LrNight {
		t.Errorf("the night assessment level must fall too: free=%.2f, shielded=%.2f",
			free.LrNight, shielded.LrNight)
	}
}
