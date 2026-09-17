package schall03

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// ---------------------------------------------------------------------------
// Gl. 28: directivity of a reflected path
// ---------------------------------------------------------------------------

// TestReflectedDirectivityUsesReflectionPoint pins the direction used for D_Ir
// in Gl. 28.
//
// REGRESSION (PLAN 1.7): the reflected-path directivity used to be evaluated
// along source → its own mirror image.  That ray is perpendicular to the wall
// by construction and does not depend on the receiver at all, so for a wall
// running parallel to the track it always produced sin²δ = 1 (D_I = +1.73 dB),
// no matter where the receiver stood.
//
// Gl. 28 defines D_Ir as the directivity "in der Richtung des
// Spiegelschallempfängers"; Bild 8 shows the reflection point R on the line
// Q–IO_i, so source → reflection point is that direction.
func TestReflectedDirectivityUsesReflectionPoint(t *testing.T) {
	t.Parallel()

	// Track along the x axis, wall parallel to it at y = 40 m.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 10}
	wall := ReflectingWall{
		A:       geo.Point2D{X: -200, Y: 40},
		B:       geo.Point2D{X: 200, Y: 40},
		HeightM: 5,
		Surface: WallSurfaceHard,
	}

	geom, ok := ComputeReflectionGeometry(source, receiver, wall)
	if !ok {
		t.Fatal("expected a valid reflection geometry")
	}

	// Track tangent vector (parallel to the wall).
	tvX, tvY, tvLen := 1.0, 0.0, 1.0

	// Correct direction: source → reflection point.
	rvX := geom.ReflectionPoint.X - source.X
	rvY := geom.ReflectionPoint.Y - source.Y
	dp := math.Hypot(rvX, rvY)
	sd2 := normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen)

	// The reflection point is at x = 400/7 on the wall.
	wantX := 400.0 / 7.0
	if math.Abs(geom.ReflectionPoint.X-wantX) > 1e-6 || math.Abs(geom.ReflectionPoint.Y-40) > 1e-9 {
		t.Fatalf("reflection point: want (%.6f, 40), got (%.6f, %.6f)",
			wantX, geom.ReflectionPoint.X, geom.ReflectionPoint.Y)
	}

	wantDp := math.Hypot(wantX, 40)
	wantSd2 := 1 - (wantX/wantDp)*(wantX/wantDp)

	if math.Abs(sd2-wantSd2) > 1e-9 {
		t.Errorf("sin²δ towards the reflection point: want %.9f, got %.9f", wantSd2, sd2)
	}

	// The old direction (source → own mirror image) is perpendicular to a
	// track-parallel wall, so it degenerates to sin²δ = 1 regardless of where
	// the receiver is.
	ivX := geom.ImageSource.X - source.X
	ivY := geom.ImageSource.Y - source.Y
	idp := math.Hypot(ivX, ivY)
	badSd2 := normativeSinDelta2(ivX, ivY, idp, tvX, tvY, tvLen)

	if math.Abs(badSd2-1.0) > 1e-12 {
		t.Fatalf("sanity: source→image should be perpendicular (sin²δ = 1), got %.12f", badSd2)
	}

	// The two differ by several dB of D_I — this is what the defect cost.
	diDelta := directivityDI(badSd2) - directivityDI(sd2)
	if diDelta < 3.0 {
		t.Errorf("expected the wrong direction to over-predict D_I by > 3 dB, got %.3f dB", diDelta)
	}
}

// ---------------------------------------------------------------------------
// Ranking of competing diffraction paths
// ---------------------------------------------------------------------------

// TestEnergeticTotalSpectrumIsEnergetic pins the energetic band combination.
//
// REGRESSION (PLAN 1.7): the function documented as "the A-weighted energetic
// sum" added the per-band dB values arithmetically, so a path that is fully
// transparent in one band could be ranked behind a path that attenuates
// everything moderately.
func TestEnergeticTotalSpectrumIsEnergetic(t *testing.T) {
	t.Parallel()

	// leaky is transparent at 63 Hz and opaque everywhere else.
	leaky := BeiblattSpectrum{0, 40, 40, 40, 40, 40, 40, 40}
	// uniform attenuates every band by 30 dB.
	uniform := BeiblattSpectrum{30, 30, 30, 30, 30, 30, 30, 30}

	// Arithmetically the leaky path looks like the *stronger* screen
	// (280 dB vs 240 dB) — that ordering is the defect.
	var leakyArith, uniformArith float64

	for f := range NumBeiblattOctaveBands {
		leakyArith += leaky[f]
		uniformArith += uniform[f]
	}

	if leakyArith <= uniformArith {
		t.Fatalf("sanity: expected the arithmetic sums to order the other way (%.0f vs %.0f)",
			leakyArith, uniformArith)
	}

	// Energetically the leaky path transmits far more, so it must rank as the
	// dominant (least attenuating) path.
	if energeticTotalSpectrum(leaky) >= energeticTotalSpectrum(uniform) {
		t.Errorf(
			"the leaky path must rank as dominant: leaky=%.3f dB, uniform=%.3f dB",
			energeticTotalSpectrum(leaky), energeticTotalSpectrum(uniform),
		)
	}

	// A flat spectrum must collapse to its own value.
	flat := BeiblattSpectrum{7, 7, 7, 7, 7, 7, 7, 7}
	if math.Abs(energeticTotalSpectrum(flat)-7.0) > 1e-9 {
		t.Errorf("flat spectrum: want 7 dB, got %.9f dB", energeticTotalSpectrum(flat))
	}
}

// ---------------------------------------------------------------------------
// Gl. 26 / Bild 6: lateral diffraction around a Seitenkante
// ---------------------------------------------------------------------------

// TestLateralPathDoesNotClimbOverTheBarrierTop pins the lateral detour
// geometry.
//
// REGRESSION (PLAN 1.7): lateralPathAbar computed the plan-view detour around
// the barrier *endpoint* but charged the path a full vertical climb from the
// source up to the barrier crest, which grossly over-estimated z (and with it
// the lateral attenuation).  A path that goes *around* a vertical Seitenkante
// never crosses the crest.
func TestLateralPathDoesNotClimbOverTheBarrierTop(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	endpoint := geo.Point2D{X: 50, Y: 5} // barrier end just 5 m off the ray

	const (
		sourceH     = 0.0
		receiverH   = 0.0
		barrierTopH = 20.0 // a tall wall — the old climb dominated z
		directDist  = 100.0
	)

	abar, ok := lateralPathAbar(source, receiver, endpoint, sourceH, receiverH, barrierTopH, directDist)
	if !ok {
		t.Fatal("expected a valid lateral path")
	}

	// Source and receiver are at the same height, so the shortest detour runs
	// horizontally around the vertical edge: z = (d_SE + d_ER) - d.
	leg := math.Hypot(50, 5)
	wantZ := 2*leg - directDist

	km := kmet(leg, leg, directDist, wantZ)

	for f := range NumBeiblattOctaveBands {
		want := math.Max(math.Min(barrierDz(wavelength(BeiblattOctaveBandFrequencies[f]), 1.0, wantZ, km), DzCapSingle), 0)
		if math.Abs(abar[f]-want) > 1e-9 {
			t.Errorf("band %d: want A_bar = %.9f dB, got %.9f dB", f, want, abar[f])
		}
	}

	// The discarded climb inflated z by more than an order of magnitude.
	climbLeg := math.Sqrt(leg*leg + barrierTopH*barrierTopH)
	oldZ := 2*climbLeg - directDist

	if oldZ < 10*wantZ {
		t.Fatalf("sanity: expected the old climb to inflate z (old=%.3f, new=%.3f)", oldZ, wantZ)
	}
}

// TestLateralPathClampsToBarrierTop covers the case where the optimal
// diffraction height on the Seitenkante would lie above the barrier crest.
func TestLateralPathClampsToBarrierTop(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	endpoint := geo.Point2D{X: 50, Y: 5}

	const (
		sourceH     = 0.0
		receiverH   = 20.0 // interpolated edge height would be ~10 m
		barrierTopH = 3.0  // but the wall is only 3 m tall
	)

	directDist := math.Hypot(100, receiverH-sourceH)

	abar, ok := lateralPathAbar(source, receiver, endpoint, sourceH, receiverH, barrierTopH, directDist)
	if !ok {
		t.Fatal("expected a valid lateral path")
	}

	leg := math.Hypot(50, 5)
	ds := math.Hypot(leg, barrierTopH-sourceH)
	dr := math.Hypot(leg, receiverH-barrierTopH)
	wantZ := ds + dr - directDist
	km := kmet(ds, dr, directDist, wantZ)

	want := math.Max(math.Min(barrierDz(wavelength(1000), 1.0, wantZ, km), DzCapSingle), 0)
	if math.Abs(abar[4]-want) > 1e-9 {
		t.Errorf("1000 Hz: want A_bar = %.9f dB, got %.9f dB", want, abar[4])
	}
}

// ---------------------------------------------------------------------------
// Nr. 6.5: a lateral path may only round a Seitenkante of the whole obstacle
// ---------------------------------------------------------------------------

// ringBarriers turns a closed plan-view ring into one obstacle's wall panels.
func ringBarriers(ring []geo.Point2D, topHeightM float64, obstacleID string) []BarrierSegment {
	out := make([]BarrierSegment, 0, len(ring)-1)

	for i := range len(ring) - 1 {
		out = append(out, BarrierSegment{
			A: ring[i], B: ring[i+1], TopHeightM: topHeightM, ObstacleID: obstacleID,
		})
	}

	return out
}

// withoutObstacleID is the pre-obstacle-identity scene: every panel stands on
// its own, so both of its endpoints count as free ends.
func withoutObstacleID(barriers []BarrierSegment) []BarrierSegment {
	out := make([]BarrierSegment, len(barriers))
	copy(out, barriers)

	for i := range out {
		out[i].ObstacleID = ""
	}

	return out
}

// topDiffractionOnly recomputes A_bar from the rubber-band edges alone, with no
// lateral candidate considered at all.
func topDiffractionOnly(
	source, receiver geo.Point2D,
	sourceHeightM, receiverHeightM float64,
	barriers []BarrierSegment,
) BeiblattSpectrum {
	totalHorizDist := geo.Distance(source, receiver)

	var obstructing []BarrierCrossing

	for _, c := range FindBarrierCrossings(source, receiver, barriers) {
		if IsObstructing(c, sourceHeightM, receiverHeightM, totalHorizDist) {
			obstructing = append(obstructing, c)
		}
	}

	edges := SelectDiffractionEdges(sourceHeightM, receiverHeightM, totalHorizDist, obstructing)
	geom := ComputeBarrierGeometryFromEdges(edges, sourceHeightM, receiverHeightM, totalHorizDist)

	return ComputeAbar(geom, BeiblattSpectrum{})
}

// TestLateralPathDoesNotRoundAnInteriorVertex pins the rule that makes a
// building footprint shield at all.
//
// Every footprint with more than four corners has a wall vertex that is not a
// corner of its silhouette.  Emitting the ring as independent panels makes that
// vertex a Seitenkante, and the path around it runs straight through the
// building.  Because barrierDz is 10·lg(3 + …), a vertex a metre off the ray
// still yields a finite A_bar, and ComputePathBarrierAttenuation's per-band
// minimum then takes it in preference to the real screening.
//
// The footprint here is 20 m wide with an extra vertex 1 m off the ray in the
// middle of its front wall — the shape any real building has.
func TestLateralPathDoesNotRoundAnInteriorVertex(t *testing.T) {
	t.Parallel()

	const (
		sourceH     = 0.5
		receiverH   = 3.5
		topHeightM  = 3.5
		obstacleID  = "house-1"
		bandMargin  = 1e-12
		receiverPos = 35.0
	)

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 0, Y: receiverPos}

	withVertex := ringBarriers([]geo.Point2D{
		{X: -10, Y: 10},
		{X: 1, Y: 10},
		{X: 10, Y: 10},
		{X: 10, Y: 20},
		{X: -10, Y: 20},
		{X: -10, Y: 10},
	}, topHeightM, obstacleID)

	// The same footprint without the collinear front-wall vertex.
	plain := ringBarriers([]geo.Point2D{
		{X: -10, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 20}, {X: -10, Y: 20}, {X: -10, Y: 10},
	}, topHeightM, obstacleID)

	grouped := ComputePathBarrierAttenuation(source, receiver, sourceH, receiverH, withVertex, BeiblattSpectrum{})
	ungrouped := ComputePathBarrierAttenuation(source, receiver, sourceH, receiverH, withoutObstacleID(withVertex), BeiblattSpectrum{})
	plainGrouped := ComputePathBarrierAttenuation(source, receiver, sourceH, receiverH, plain, BeiblattSpectrum{})
	topOnly := topDiffractionOnly(source, receiver, sourceH, receiverH, withVertex)

	for f := range NumBeiblattOctaveBands {
		// The receiver sits dead centre behind the footprint, so rounding a
		// silhouette corner costs more than going over the roof.  No lateral
		// path may undercut the top-diffraction spectrum at all.
		if math.Abs(grouped[f]-topOnly[f]) > bandMargin {
			t.Errorf("band %d: grouped A_bar = %.9f dB, want the top-diffraction %.9f dB",
				f, grouped[f], topOnly[f])
		}

		// A vertex that merely subdivides a wall is acoustically nothing.
		if math.Abs(grouped[f]-plainGrouped[f]) > bandMargin {
			t.Errorf("band %d: a collinear wall vertex moved A_bar from %.9f to %.9f dB",
				f, plainGrouped[f], grouped[f])
		}

		// Without the shared identity the interior vertex is taken as a free
		// end and pulls the screening down in every band.
		if ungrouped[f] >= grouped[f] {
			t.Errorf("band %d: expected the ungrouped panels to lose screening, got %.9f dB against %.9f dB",
				f, ungrouped[f], grouped[f])
		}
	}
}

// TestLateralPathAroundASmallFootprintStillWins is the other half of the rule:
// suppressing lateral diffraction for buildings would satisfy
// TestLateralPathDoesNotRoundAnInteriorVertex and still be wrong.  Rounding a
// small building really is the dominant path, so a 4 m x 4 m footprint with the
// receiver 40 m behind it must come out below the top-diffraction spectrum.
func TestLateralPathAroundASmallFootprintStillWins(t *testing.T) {
	t.Parallel()

	const (
		sourceH    = 0.5
		receiverH  = 3.5
		topHeightM = 8.0
	)

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 0, Y: 54}

	hut := ringBarriers([]geo.Point2D{
		{X: -2, Y: 10}, {X: 2, Y: 10}, {X: 2, Y: 14}, {X: -2, Y: 14}, {X: -2, Y: 10},
	}, topHeightM, "hut-1")

	got := ComputePathBarrierAttenuation(source, receiver, sourceH, receiverH, hut, BeiblattSpectrum{})
	topOnly := topDiffractionOnly(source, receiver, sourceH, receiverH, hut)

	for f := range NumBeiblattOctaveBands {
		if got[f] >= topOnly[f] {
			t.Errorf("band %d: lateral diffraction around a small footprint must win, got %.9f dB against top-only %.9f dB",
				f, got[f], topOnly[f])
		}
	}
}

// TestCoincidentCrossingsOfOneObstacleAreOneCrossing pins the dedupe in
// FindBarrierCrossings.  geo.SegmentIntersection tests t,u ∈ [0,1] closed, so a
// ray through a footprint corner intersects both adjoining edges at that
// vertex.  Two crossings at the same distance and height read as double
// diffraction with e = 0, which silently raises the D_z cap from DzCapSingle to
// DzCapDouble.
func TestCoincidentCrossingsOfOneObstacleAreOneCrossing(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: -10, Y: -10}
	receiver := geo.Point2D{X: 10, Y: 10}

	// The ray runs along y = x and passes exactly through the corner (0, 0).
	corner := []BarrierSegment{
		{A: geo.Point2D{X: -5, Y: 0}, B: geo.Point2D{X: 0, Y: 0}, TopHeightM: 6, ObstacleID: "house-1"},
		{A: geo.Point2D{X: 0, Y: 0}, B: geo.Point2D{X: 0, Y: 5}, TopHeightM: 6, ObstacleID: "house-1"},
	}

	crossings := FindBarrierCrossings(source, receiver, corner)
	if len(crossings) != 1 {
		t.Fatalf("got %d crossings through one corner of one obstacle, want 1: %+v", len(crossings), crossings)
	}

	// Two panels that merely happen to meet there are still two obstacles.
	separate := withoutObstacleID(corner)

	crossings = FindBarrierCrossings(source, receiver, separate)
	if len(crossings) != 2 {
		t.Fatalf("got %d crossings for two unnamed panels, want 2: %+v", len(crossings), crossings)
	}
}

// ---------------------------------------------------------------------------
// Gl. 19/20: D_refl gating in a scene
// ---------------------------------------------------------------------------

// TestDreflOnlyForNearReflectiveWalls exercises the Gl. 20 gate through the
// scene-level geometry builder.
//
// REGRESSION (PLAN 1.7): D_refl was subtracted from every top-diffraction
// path, with no d_s ≤ 5 m test and no reflective/absorbing distinction, so
// every barrier lost up to 3 dB of insertion loss.
func TestDreflOnlyForNearReflectiveWalls(t *testing.T) {
	t.Parallel()

	var noGroundEffect BeiblattSpectrum

	edgeAt := func(dist float64, reflective bool) []DiffractionEdge {
		return []DiffractionEdge{{
			Point:          geo.Point2D{X: dist},
			HeightM:        4,
			DistFromSource: dist,
			Barrier: BarrierSegment{
				A: geo.Point2D{X: dist, Y: -10}, B: geo.Point2D{X: dist, Y: 10},
				TopHeightM: 4, BaseHeightM: 0, Reflective: reflective,
			},
		}}
	}

	// A reflective wall 30 m from the source: d_s ≈ 30.3 m > 5 m, so Gl. 20
	// Anmerkung 5 applies and the wall keeps its full insertion loss.
	farRefl := ComputeBarrierGeometryFromEdges(edgeAt(30, true), 0, 4, 100)
	if farRefl.Ds <= dReflMaxDsM {
		t.Fatalf("sanity: expected d_s > %g m, got %.3f m", dReflMaxDsM, farRefl.Ds)
	}

	farAbs := ComputeBarrierGeometryFromEdges(edgeAt(30, false), 0, 4, 100)

	gotFarRefl := ComputeAbar(farRefl, noGroundEffect)
	gotFarAbs := ComputeAbar(farAbs, noGroundEffect)

	for f := range NumBeiblattOctaveBands {
		if gotFarRefl[f] != gotFarAbs[f] {
			t.Errorf("band %d: a wall at d_s > 5 m must not lose D_refl (%.6f vs %.6f)",
				f, gotFarRefl[f], gotFarAbs[f])
		}

		if gotFarRefl[f] <= 0 {
			t.Errorf("band %d: expected a positive insertion loss, got %.6f dB", f, gotFarRefl[f])
		}
	}

	// A reflective wall 2 m from the source with no absorbing base does lose
	// the full 3 dB of Gl. 20; the same wall built absorbing does not.
	nearRefl := ComputeBarrierGeometryFromEdges(edgeAt(2, true), 0, 4, 100)
	if nearRefl.Ds > dReflMaxDsM {
		t.Fatalf("sanity: expected d_s <= %g m, got %.3f m", dReflMaxDsM, nearRefl.Ds)
	}

	nearAbs := ComputeBarrierGeometryFromEdges(edgeAt(2, false), 0, 4, 100)

	gotNearRefl := ComputeAbar(nearRefl, noGroundEffect)
	gotNearAbs := ComputeAbar(nearAbs, noGroundEffect)

	// 8000 Hz has the largest D_z, so it is furthest from the ≥ 0 dB clamp.
	if diff := gotNearAbs[7] - gotNearRefl[7]; math.Abs(diff-3.0) > 1e-9 {
		t.Errorf("8000 Hz: a near reflective wall must lose 3 dB, got %.6f dB", diff)
	}
}

// ---------------------------------------------------------------------------
// Bild 6: e = e₁ + e₂ + e₃ …
// ---------------------------------------------------------------------------

// TestMultiEdgeELengthIsAPolyline pins e as the travel path length between the
// first and the last Schirmkante.
//
// REGRESSION (PLAN 1.7): with more than two significant edges the geometry
// kept only the outermost two and used the straight chord between them, which
// under-estimates e and therefore z.
func TestMultiEdgeELengthIsAPolyline(t *testing.T) {
	t.Parallel()

	edges := []DiffractionEdge{
		{DistFromSource: 25, HeightM: 6, Point: geo.Point2D{X: 25}},
		{DistFromSource: 50, HeightM: 10, Point: geo.Point2D{X: 50}},
		{DistFromSource: 75, HeightM: 6, Point: geo.Point2D{X: 75}},
	}

	geom := ComputeBarrierGeometryFromEdges(edges, 0, 0, 100)

	wantE := 2 * math.Hypot(25, 4) // e₁ + e₂ over the raised middle edge
	if math.Abs(geom.E-wantE) > 1e-9 {
		t.Errorf("e: want %.9f m (polyline), got %.9f m", wantE, geom.E)
	}

	chord := math.Hypot(50, 0) // what the old chord-based code produced
	if math.Abs(geom.E-chord) < 1e-6 {
		t.Error("e must not collapse to the chord between the outermost edges")
	}

	// z must follow Gl. 26 with the polyline e.
	ds := math.Hypot(25, 6)
	dr := math.Hypot(25, 6)
	wantZ := ds + dr + wantE - 100

	if math.Abs(geom.Z-wantZ) > 1e-9 {
		t.Errorf("z: want %.9f m, got %.9f m", wantZ, geom.Z)
	}
}

// ---------------------------------------------------------------------------
// Determinism
// ---------------------------------------------------------------------------

// TestSubsegmentContribIsOrderDeterministic guards the fixed height-index
// iteration order required by docs/policies/determinism.md.
//
// REGRESSION (PLAN 1.8): the accumulation used to range over
// emission.PerHeight, a map[int]BeiblattSpectrum, so Go's randomised map
// iteration decided the float summation order.
func TestSubsegmentContribIsOrderDeterministic(t *testing.T) {
	t.Parallel()

	emission := &StreckeEmissionResult{
		PerHeight: map[int]BeiblattSpectrum{
			1: {91.3, 88.7, 86.1, 90.9, 93.4, 89.2, 84.6, 78.1},
			2: {70.2, 74.9, 79.5, 82.3, 80.7, 77.4, 71.8, 65.3},
			3: {60.11, 63.27, 66.53, 69.71, 68.19, 64.83, 59.47, 53.09},
		},
	}

	receiver := ReceiverInput{
		ID:      "r1",
		Point:   geo.Point2D{X: 0, Y: 137},
		HeightM: 4.7,
	}

	first := normativeSubsegmentContrib(emission, 1.3, receiver, 137.0, 3.7, 0.83, 0.0)

	for i := range 500 {
		got := normativeSubsegmentContrib(emission, 1.3, receiver, 137.0, 3.7, 0.83, 0.0)
		if got != first {
			t.Fatalf("run %d: summation is not bit-identical: %.20g vs %.20g", i, got, first)
		}
	}
}

// ---------------------------------------------------------------------------
// Gl. 28: a facade must not diffract the reflection it produced
// ---------------------------------------------------------------------------

// reflectionSceneTrack is the emitting track the reflection regressions share.
func reflectionSceneTrack(t *testing.T) TrackSegment {
	t.Helper()

	op, err := NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	return TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -150, Y: 0}, {X: 150, Y: 0}},
		ElevationM:      0,
		Fahrbahn:        FahrbahnartSchwellengleis,
		Surface:         SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []TrainOperation{*op},
	}
}

// reflectingHouse returns the shielding panels and the reflecting facades of one
// opt-in reflecting building, both carrying obstacleID.
func reflectingHouse(obstacleID string) ([]BarrierSegment, []ReflectingWall) {
	ring := []geo.Point2D{
		{X: -60, Y: 40}, {X: 60, Y: 40}, {X: 60, Y: 50}, {X: -60, Y: 50}, {X: -60, Y: 40},
	}

	barriers := ringBarriers(ring, 12, obstacleID)

	walls := make([]ReflectingWall, 0, len(barriers))
	for _, b := range barriers {
		walls = append(walls, ReflectingWall{
			A: b.A, B: b.B, HeightM: 12, Surface: WallSurfaceBuilding, ObstacleID: obstacleID,
		})
	}

	return barriers, walls
}

// withoutWallObstacleID is the pre-identity wall set: every facade belongs to no
// named obstacle, so nothing is excluded from the paths it generates.
func withoutWallObstacleID(walls []ReflectingWall) []ReflectingWall {
	out := make([]ReflectingWall, len(walls))
	copy(out, walls)

	for i := range out {
		out[i].ObstacleID = ""
	}

	return out
}

// TestReflectingBuildingDoesNotShieldItsOwnReflection pins the obstacle
// exclusion on mirrored paths.
//
// REGRESSION: making building footprints shield turned every opt-in reflecting
// building into a barrier standing exactly on its own reflection point, so the
// facade attenuated the reflection it had just produced and the opt-in
// contribution collapsed.  A reflected ray crosses its own reflector by
// construction — that crossing is the reflection, not a diffraction edge.
//
// The house stands behind the receiver, so only the reflected contribution can
// differ: the direct path from the track never reaches y = 40 m.
func TestReflectingBuildingDoesNotShieldItsOwnReflection(t *testing.T) {
	t.Parallel()

	seg := reflectionSceneTrack(t)
	receiver := ReceiverInput{ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5}

	barriers, walls := reflectingHouse("house-1")

	excluded, err := ComputeNormativeReceiverLevelsWithScene(receiver, []TrackSegment{seg}, walls, barriers)
	if err != nil {
		t.Fatal(err)
	}

	selfShielded, err := ComputeNormativeReceiverLevelsWithScene(
		receiver, []TrackSegment{seg}, withoutWallObstacleID(walls), barriers,
	)
	if err != nil {
		t.Fatal(err)
	}

	// A margin, not a bare ">": the self-shielded scene sits within 0.01 dB of
	// the free field, because the facade swallowed the whole reflection.
	if excluded.LpAeqDay-selfShielded.LpAeqDay < 0.5 {
		t.Errorf("a facade must not diffract its own reflection: excluded = %.6f dB, self-shielded = %.6f dB",
			excluded.LpAeqDay, selfShielded.LpAeqDay)
	}
}

// TestReflectedPathStillShieldedByAnotherObstacle keeps the exclusion scoped.
//
// The fix above must not degenerate into "reflections are never shielded": a
// screen that is a *different* obstacle still stands in the mirrored ray and
// still attenuates it.  The screen sits between the receiver and the house, so
// it too leaves the direct path alone.
func TestReflectedPathStillShieldedByAnotherObstacle(t *testing.T) {
	t.Parallel()

	seg := reflectionSceneTrack(t)
	receiver := ReceiverInput{ID: "r1", Point: geo.Point2D{X: 0, Y: 30}, HeightM: 3.5}

	barriers, walls := reflectingHouse("house-1")

	withScreen := append([]BarrierSegment{}, barriers...)
	withScreen = append(withScreen, BarrierSegment{
		A: geo.Point2D{X: -80, Y: 35}, B: geo.Point2D{X: 80, Y: 35},
		TopHeightM: 8, ObstacleID: "screen-1",
	})

	houseOnly, err := ComputeNormativeReceiverLevelsWithScene(receiver, []TrackSegment{seg}, walls, barriers)
	if err != nil {
		t.Fatal(err)
	}

	screened, err := ComputeNormativeReceiverLevelsWithScene(receiver, []TrackSegment{seg}, walls, withScreen)
	if err != nil {
		t.Fatal(err)
	}

	if houseOnly.LpAeqDay-screened.LpAeqDay < 0.5 {
		t.Errorf("another obstacle in the mirrored ray must still shield it: screened = %.6f dB, unscreened = %.6f dB",
			screened.LpAeqDay, houseOnly.LpAeqDay)
	}
}

// TestBarriersExcludingObstaclesKeepsUnnamedPanels pins the empty-id
// convention: a panel that belongs to no named obstacle is never excluded, so a
// scene built before obstacle identity existed keeps its previous behaviour.
func TestBarriersExcludingObstaclesKeepsUnnamedPanels(t *testing.T) {
	t.Parallel()

	barriers := []BarrierSegment{
		{A: geo.Point2D{X: 0, Y: 0}, B: geo.Point2D{X: 10, Y: 0}, TopHeightM: 4},
		{A: geo.Point2D{X: 0, Y: 5}, B: geo.Point2D{X: 10, Y: 5}, TopHeightM: 4, ObstacleID: "house-1"},
		{A: geo.Point2D{X: 0, Y: 9}, B: geo.Point2D{X: 10, Y: 9}, TopHeightM: 4, ObstacleID: "screen-1"},
	}

	// No ids to exclude: the slice is returned untouched.
	if got := barriersExcludingObstacles(barriers, nil); len(got) != 3 {
		t.Fatalf("got %d panels for an empty exclusion set, want 3", len(got))
	}

	// An empty id is not a name, so it must not match the unnamed panel.
	if got := barriersExcludingObstacles(barriers, []string{""}); len(got) != 3 {
		t.Fatalf("got %d panels when excluding the empty id, want 3", len(got))
	}

	got := barriersExcludingObstacles(barriers, []string{"house-1"})
	if len(got) != 2 {
		t.Fatalf("got %d panels after excluding one obstacle, want 2: %+v", len(got), got)
	}

	for _, b := range got {
		if b.ObstacleID == "house-1" {
			t.Fatalf("excluded obstacle survived: %+v", b)
		}
	}
}
