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

// A barrier on a SECOND-order reflected path shields it.
//
// The reflected level is computed through ComputeReflectedLineSourceLpAeqWithBarriers
// directly rather than through ComputeNormativeReceiverLevelsWithScene, so the
// direct contribution — which dominates the total and which the barrier here
// does not touch — cannot dilute the assertion.
//
// Geometry.  A canyon of two parallel reflectors at y = +40 and y = −20 around a
// track on y = 0, with the receiver at y = 30.  The paths the enumerator finds
// unfold to these origins (see TestReflectionPathEffectiveSourceIsTheUnfoldedOrigin
// for why the unfolded origin is the last bounce's image):
//
//	order 1 via y=+40 → image at y =  80, ray reaches y ≤  80
//	order 1 via y=−20 → image at y = −40, ray reaches y ≤  30
//	order 2 via y=−20 then y=+40 → image at y = 120, ray reaches y ≤ 120
//
// The barrier stands at y = 100, in the strip that only the order-2 unfolded ray
// crosses.  It is therefore invisible to every first-order path and visible to
// the second-order one, which is exactly the distinction the effective-source
// choice makes.  Handing the barrier check the FIRST bounce's image instead
// leaves this barrier unseen and the level unchanged; that regression is what
// this test catches.
func TestSecondOrderReflectedPathObstructedByBarrier(t *testing.T) {
	t.Parallel()

	// A flat spectrum on all three Teilquelle heights: the assertion is about
	// which ray the diffraction check follows, not about emission tables.
	emission := &schall03.StreckeEmissionResult{
		PerHeight: map[int]schall03.BeiblattSpectrum{
			1: {90, 90, 90, 90, 90, 90, 90, 90},
			2: {90, 90, 90, 90, 90, 90, 90, 90},
			3: {90, 90, 90, 90, 90, 90, 90, 90},
		},
	}

	centerline := []geo.Point2D{{X: -100, Y: 0}, {X: 100, Y: 0}}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5,
	}

	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -150, Y: 40}, B: geo.Point2D{X: 150, Y: 40}, HeightM: 15, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: -150, Y: -20}, B: geo.Point2D{X: 150, Y: -20}, HeightM: 15, Surface: schall03.WallSurfaceHard},
	}

	// Guard the premise: the scene must actually produce a second-order path,
	// and no first-order unfolded ray may reach the barrier line.
	assertSecondOrderReachesBarrierLine(t, centerline, receiver, walls, 100)

	barriers := []schall03.BarrierSegment{
		{
			A: geo.Point2D{X: -250, Y: 100}, B: geo.Point2D{X: 250, Y: 100},
			TopHeightM: 8, BaseHeightM: 0,
		},
	}

	free := schall03.ComputeReflectedLineSourceLpAeqWithBarriers(
		emission, centerline, 0, receiver, 0, walls, nil,
	)

	shielded := schall03.ComputeReflectedLineSourceLpAeqWithBarriers(
		emission, centerline, 0, receiver, 0, walls, barriers,
	)

	if math.IsInf(free, -1) {
		t.Fatal("expected a finite reflected level without barriers")
	}

	// A margin, not a bare ">": before this was fixed the barrier was invisible
	// to every path and the two levels were bit-identical, so any movement at all
	// would pass an inequality — including a movement that is only a rounding
	// step.  0.2 dB is well inside the ~0.55 dB this scene actually produces.
	const wantInsertionLossDB = 0.2

	got := free - shielded
	if got < wantInsertionLossDB {
		t.Errorf("a barrier across the order-2 unfolded ray must reduce the reflected level by at least "+
			"%.2f dB: got %.4f dB (free=%.4f, shielded=%.4f)", wantInsertionLossDB, got, free, shielded)
	}
}

// assertSecondOrderReachesBarrierLine checks the premise of the test above: over
// the whole centerline at least one order-2 path exists whose unfolded ray
// crosses y = barrierY, and no order-1 unfolded ray gets that far.  Without this
// the test could pass for the wrong reason if the geometry ever drifted.
func assertSecondOrderReachesBarrierLine(
	t *testing.T,
	centerline []geo.Point2D,
	receiver schall03.ReceiverInput,
	walls []schall03.ReflectingWall,
	barrierY float64,
) {
	t.Helper()

	secondOrderCrossings := 0

	// Sample along the centerline the way the integrator does, rather than only
	// at its vertices: the enumerator's answer depends on where on the track the
	// source sits.
	const samples = 41

	a := centerline[0]
	b := centerline[len(centerline)-1]

	for i := range samples {
		frac := float64(i) / float64(samples-1)
		pt := geo.Point2D{X: a.X + (b.X-a.X)*frac, Y: a.Y + (b.Y-a.Y)*frac}

		for _, rp := range schall03.EnumerateReflectionPaths(pt, receiver.Point, walls, schall03.MaxReflectionOrder) {
			reach := math.Max(rp.EffectiveSource().Y, receiver.Point.Y)
			if reach < barrierY {
				continue
			}

			if rp.Order == 1 {
				t.Fatalf("an order-1 path already reaches y=%g (image %v) — the barrier no longer isolates order ≥ 2",
					barrierY, rp.EffectiveSource())
			}

			secondOrderCrossings++
		}
	}

	if secondOrderCrossings == 0 {
		t.Fatalf("no reflected path of order ≥ 2 reaches y=%g — the barrier would shield nothing", barrierY)
	}
}
