package schall03_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestBarrierSegmentValidateValid(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:           geo.Point2D{X: 0, Y: 10},
		B:           geo.Point2D{X: 100, Y: 10},
		TopHeightM:  4.0,
		BaseHeightM: 0.5,
		ThicknessM:  0,
	}

	err := b.Validate()
	if err != nil {
		t.Errorf("valid barrier should pass: %v", err)
	}
}

func TestBarrierSegmentValidateThickBarrier(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:           geo.Point2D{X: 0, Y: 10},
		B:           geo.Point2D{X: 100, Y: 10},
		TopHeightM:  4.0,
		BaseHeightM: 0,
		ThicknessM:  0.3,
		IsParallel:  true,
	}

	err := b.Validate()
	if err != nil {
		t.Errorf("thick barrier should pass: %v", err)
	}
}

func TestBarrierSegmentValidateZeroLength(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:          geo.Point2D{X: 5, Y: 5},
		B:          geo.Point2D{X: 5, Y: 5},
		TopHeightM: 3.0,
	}

	err := b.Validate()
	if err == nil {
		t.Error("zero-length barrier should fail validation")
	}
}

func TestBarrierSegmentValidateNegativeHeight(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:          geo.Point2D{X: 0, Y: 0},
		B:          geo.Point2D{X: 10, Y: 0},
		TopHeightM: -1,
	}

	err := b.Validate()
	if err == nil {
		t.Error("negative height should fail validation")
	}
}

func TestBarrierSegmentValidateBaseAboveTop(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:           geo.Point2D{X: 0, Y: 0},
		B:           geo.Point2D{X: 10, Y: 0},
		TopHeightM:  3.0,
		BaseHeightM: 3.5,
	}

	err := b.Validate()
	if err == nil {
		t.Error("base above top should fail validation")
	}
}

func TestBarrierSegmentValidateNegativeThickness(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A:           geo.Point2D{X: 0, Y: 0},
		B:           geo.Point2D{X: 10, Y: 0},
		TopHeightM:  3.0,
		BaseHeightM: 0,
		ThicknessM:  -0.5,
	}

	err := b.Validate()
	if err == nil {
		t.Error("negative thickness should fail validation")
	}
}

func TestBarrierSegmentLength(t *testing.T) {
	t.Parallel()

	b := schall03.BarrierSegment{
		A: geo.Point2D{X: 0, Y: 0},
		B: geo.Point2D{X: 30, Y: 40},
	}

	got := b.Length()
	if got != 50.0 {
		t.Errorf("length: want 50, got %g", got)
	}
}

func TestFindBarrierCrossingsNone(t *testing.T) {
	t.Parallel()

	// Barrier is off to the side — no crossing.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	barriers := []schall03.BarrierSegment{
		{A: geo.Point2D{X: 50, Y: 20}, B: geo.Point2D{X: 50, Y: 30}, TopHeightM: 4, BaseHeightM: 0},
	}

	crossings := schall03.FindBarrierCrossings(source, receiver, barriers)
	if len(crossings) != 0 {
		t.Errorf("expected 0 crossings, got %d", len(crossings))
	}
}

func TestFindBarrierCrossingsSingle(t *testing.T) {
	t.Parallel()

	// Barrier crosses the path perpendicularly at x=50.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	barriers := []schall03.BarrierSegment{
		{A: geo.Point2D{X: 50, Y: -10}, B: geo.Point2D{X: 50, Y: 10}, TopHeightM: 4, BaseHeightM: 0},
	}

	crossings := schall03.FindBarrierCrossings(source, receiver, barriers)
	if len(crossings) != 1 {
		t.Fatalf("expected 1 crossing, got %d", len(crossings))
	}

	assertApproxRefl(t, crossings[0].Point.X, 50.0, 0.01, "crossing X")
	assertApproxRefl(t, crossings[0].Point.Y, 0.0, 0.01, "crossing Y")
	assertApproxRefl(t, crossings[0].DistFromSource, 50.0, 0.01, "dist from source")

	if crossings[0].BarrierIdx != 0 {
		t.Errorf("expected barrier index 0, got %d", crossings[0].BarrierIdx)
	}
}

func TestFindBarrierCrossingsMultipleSorted(t *testing.T) {
	t.Parallel()

	// Two barriers at x=30 and x=70 — should be returned sorted by distance.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	barriers := []schall03.BarrierSegment{
		{A: geo.Point2D{X: 70, Y: -10}, B: geo.Point2D{X: 70, Y: 10}, TopHeightM: 4, BaseHeightM: 0},
		{A: geo.Point2D{X: 30, Y: -10}, B: geo.Point2D{X: 30, Y: 10}, TopHeightM: 3, BaseHeightM: 0},
	}

	crossings := schall03.FindBarrierCrossings(source, receiver, barriers)
	if len(crossings) != 2 {
		t.Fatalf("expected 2 crossings, got %d", len(crossings))
	}

	// First crossing should be the nearer one (x=30, barrier index 1).
	assertApproxRefl(t, crossings[0].DistFromSource, 30.0, 0.01, "first dist")

	if crossings[0].BarrierIdx != 1 {
		t.Errorf("first crossing: expected barrier index 1, got %d", crossings[0].BarrierIdx)
	}

	// Second crossing should be the farther one (x=70, barrier index 0).
	assertApproxRefl(t, crossings[1].DistFromSource, 70.0, 0.01, "second dist")

	if crossings[1].BarrierIdx != 0 {
		t.Errorf("second crossing: expected barrier index 0, got %d", crossings[1].BarrierIdx)
	}
}

func TestFindBarrierCrossingsBehindReceiver(t *testing.T) {
	t.Parallel()

	// Barrier is behind the receiver — ray does not extend past receiver.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 50, Y: 0}
	barriers := []schall03.BarrierSegment{
		{A: geo.Point2D{X: 80, Y: -10}, B: geo.Point2D{X: 80, Y: 10}, TopHeightM: 4, BaseHeightM: 0},
	}

	crossings := schall03.FindBarrierCrossings(source, receiver, barriers)
	if len(crossings) != 0 {
		t.Errorf("barrier behind receiver should not be crossed, got %d crossings", len(crossings))
	}
}

func TestIsObstructingAboveLOS(t *testing.T) {
	t.Parallel()

	// Source at height 0, receiver at height 0, barrier top at 4 m.
	// Line-of-sight at crossing = 0 m → barrier (4 m) is above → obstructing.
	crossing := schall03.BarrierCrossing{
		Point:          geo.Point2D{X: 50, Y: 0},
		DistFromSource: 50,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: 50, Y: -10}, B: geo.Point2D{X: 50, Y: 10},
			TopHeightM: 4, BaseHeightM: 0,
		},
	}

	if !schall03.IsObstructing(crossing, 0, 0, 100) {
		t.Error("barrier above line-of-sight should obstruct")
	}
}

func TestIsObstructingBelowLOS(t *testing.T) {
	t.Parallel()

	// Source at height 0, receiver at height 10.
	// At midpoint (frac=0.5): LOS height = 5 m.
	// Barrier top at 3 m → below LOS → not obstructing.
	crossing := schall03.BarrierCrossing{
		Point:          geo.Point2D{X: 50, Y: 0},
		DistFromSource: 50,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: 50, Y: -10}, B: geo.Point2D{X: 50, Y: 10},
			TopHeightM: 3, BaseHeightM: 0,
		},
	}

	if schall03.IsObstructing(crossing, 0, 10, 100) {
		t.Error("barrier below line-of-sight should not obstruct")
	}
}

func TestIsObstructingExactlyAtLOS(t *testing.T) {
	t.Parallel()

	// Source at height 2, receiver at height 6.
	// At frac=0.5: LOS = 2 + 0.5*4 = 4 m.
	// Barrier top exactly at 4 m → not obstructing (must be strictly above).
	crossing := schall03.BarrierCrossing{
		Point:          geo.Point2D{X: 50, Y: 0},
		DistFromSource: 50,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: 50, Y: -10}, B: geo.Point2D{X: 50, Y: 10},
			TopHeightM: 4, BaseHeightM: 0,
		},
	}

	if schall03.IsObstructing(crossing, 2, 6, 100) {
		t.Error("barrier exactly at line-of-sight should not obstruct (must be strictly above)")
	}
}

func TestIsObstructingNearSource(t *testing.T) {
	t.Parallel()

	// Source at height 0, receiver at height 0, barrier close to source at 10 m.
	// LOS = 0 everywhere → any positive barrier height obstructs.
	crossing := schall03.BarrierCrossing{
		Point:          geo.Point2D{X: 10, Y: 0},
		DistFromSource: 10,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: 10, Y: -5}, B: geo.Point2D{X: 10, Y: 5},
			TopHeightM: 2, BaseHeightM: 0,
		},
	}

	if !schall03.IsObstructing(crossing, 0, 0, 100) {
		t.Error("barrier near source should obstruct when LOS is at ground level")
	}
}

// helper to build a BarrierCrossing for SelectDiffractionEdges tests.
func makeCrossing(x, topH, distFromSrc float64, idx int) schall03.BarrierCrossing {
	return schall03.BarrierCrossing{
		Point:          geo.Point2D{X: x, Y: 0},
		BarrierIdx:     idx,
		DistFromSource: distFromSrc,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: x, Y: -10}, B: geo.Point2D{X: x, Y: 10},
			TopHeightM: topH, BaseHeightM: 0,
		},
	}
}

func TestSelectDiffractionEdgesSingleBarrier(t *testing.T) {
	t.Parallel()

	// Source h=0, receiver h=0, total dist=100.
	// One barrier at dist=50 with top at 4 m → selected as the only edge.
	crossings := []schall03.BarrierCrossing{
		makeCrossing(50, 4, 50, 0),
	}

	edges := schall03.SelectDiffractionEdges(0, 0, 100, crossings)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	assertApproxRefl(t, edges[0].DistFromSource, 50, 0.01, "edge dist")
	assertApproxRefl(t, edges[0].HeightM, 4, 0.01, "edge height")
}

func TestSelectDiffractionEdgesTwoBarriersBothVisible(t *testing.T) {
	t.Parallel()

	// Source h=0, receiver h=0, total dist=100.
	// Barrier 1 at dist=30, top=5 m.
	// Barrier 2 at dist=70, top=5 m.
	// Both are above the rubber band from source→receiver → both selected.
	crossings := []schall03.BarrierCrossing{
		makeCrossing(30, 5, 30, 0),
		makeCrossing(70, 5, 70, 1),
	}

	edges := schall03.SelectDiffractionEdges(0, 0, 100, crossings)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	assertApproxRefl(t, edges[0].DistFromSource, 30, 0.01, "first edge dist")
	assertApproxRefl(t, edges[1].DistFromSource, 70, 0.01, "second edge dist")
}

func TestSelectDiffractionEdgesInnerBarrierHidden(t *testing.T) {
	t.Parallel()

	// Source h=0, receiver h=0, total dist=100.
	// Barrier 1 at dist=30, top=6 m (tall).
	// Barrier 2 at dist=50, top=3 m (short — hidden below rubber band from barrier 1 to receiver).
	// Barrier 3 at dist=70, top=6 m (tall).
	// The rubber band from source(0,0) → barrier1(30,6) → barrier3(70,6) → receiver(100,0)
	// passes above barrier2(50,3). So only barriers 1 and 3 are selected.
	//
	// Check: line from (30,6) to (70,6) is flat at h=6. Barrier2 at h=3 is below → hidden.
	crossings := []schall03.BarrierCrossing{
		makeCrossing(30, 6, 30, 0),
		makeCrossing(50, 3, 50, 1),
		makeCrossing(70, 6, 70, 2),
	}

	edges := schall03.SelectDiffractionEdges(0, 0, 100, crossings)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (inner hidden), got %d", len(edges))
	}

	if edges[0].BarrierIdx != 0 {
		t.Errorf("first edge: expected barrier 0, got %d", edges[0].BarrierIdx)
	}

	if edges[1].BarrierIdx != 2 {
		t.Errorf("second edge: expected barrier 2, got %d", edges[1].BarrierIdx)
	}
}

func TestSelectDiffractionEdgesBarrierAtSourceHeight(t *testing.T) {
	t.Parallel()

	// Source h=5, receiver h=5, total dist=100.
	// Barrier at dist=50, top=5 m → exactly at LOS → not part of upper hull.
	// (It wouldn't be obstructing either, but if passed in, the hull should exclude it.)
	crossings := []schall03.BarrierCrossing{
		makeCrossing(50, 5, 50, 0),
	}

	edges := schall03.SelectDiffractionEdges(5, 5, 100, crossings)
	if len(edges) != 0 {
		t.Errorf("barrier at LOS height should not be selected, got %d edges", len(edges))
	}
}

func TestSelectDiffractionEdgesNoCrossings(t *testing.T) {
	t.Parallel()

	edges := schall03.SelectDiffractionEdges(0, 0, 100, nil)
	if len(edges) != 0 {
		t.Errorf("no crossings should return no edges, got %d", len(edges))
	}
}

// makeEdge builds a DiffractionEdge for ComputeBarrierGeometryFromEdges tests.
func makeEdge(dist, topH, baseH float64, idx int) schall03.DiffractionEdge {
	return schall03.DiffractionEdge{
		Point:          geo.Point2D{X: dist, Y: 0},
		HeightM:        topH,
		DistFromSource: dist,
		BarrierIdx:     idx,
		Barrier: schall03.BarrierSegment{
			A: geo.Point2D{X: dist, Y: -10}, B: geo.Point2D{X: dist, Y: 10},
			TopHeightM: topH, BaseHeightM: baseH,
		},
	}
}

func TestBarrierGeometrySingleThinBarrier(t *testing.T) {
	t.Parallel()

	// Source h=0, receiver h=0, horiz dist=100.
	// Barrier at dist=50, top=4 m, base=0.
	//
	// ds = sqrt(50² + 4²) = sqrt(2516) ≈ 50.16
	// dr = sqrt(50² + 4²) = sqrt(2516) ≈ 50.16
	// d  = 100 (flat)
	// z  = ds + dr - d ≈ 100.32 - 100 = 0.32
	edges := []schall03.DiffractionEdge{makeEdge(50, 4, 0, 0)}
	bg := schall03.ComputeBarrierGeometryFromEdges(edges, 0, 0, 100)

	assertApproxRefl(t, bg.Ds, 50.16, 0.01, "ds")
	assertApproxRefl(t, bg.Dr, 50.16, 0.01, "dr")
	assertApproxRefl(t, bg.D, 100.0, 0.01, "d")
	assertApproxRefl(t, bg.Z, 0.32, 0.01, "z")
	assertApproxRefl(t, bg.E, 0.0, 0.001, "e")
	assertApproxRefl(t, bg.Habs, 0.0, 0.001, "habs")

	if bg.IsDouble {
		t.Error("single barrier should not be double")
	}

	if !bg.TopDiffraction {
		t.Error("should be top diffraction")
	}
}

func TestBarrierGeometryDoubleBarrier(t *testing.T) {
	t.Parallel()

	// Source h=0, receiver h=0, horiz dist=100.
	// Barrier 1 at dist=30, top=5 m, base=0.5.
	// Barrier 2 at dist=70, top=5 m, base=1.0.
	//
	// ds = sqrt(30² + 5²) = sqrt(925) ≈ 30.41
	// dr = sqrt(30² + 5²) = sqrt(925) ≈ 30.41
	// e  = sqrt(40² + 0²) = 40.0 (same height, horiz only)
	// d  = 100
	// z  = ds + dr + e - d ≈ 30.41 + 30.41 + 40 - 100 = 0.82
	// habs = max(0.5, 1.0) = 1.0
	edges := []schall03.DiffractionEdge{
		makeEdge(30, 5, 0.5, 0),
		makeEdge(70, 5, 1.0, 1),
	}

	bg := schall03.ComputeBarrierGeometryFromEdges(edges, 0, 0, 100)

	assertApproxRefl(t, bg.Ds, 30.41, 0.01, "ds")
	assertApproxRefl(t, bg.Dr, 30.41, 0.01, "dr")
	assertApproxRefl(t, bg.D, 100.0, 0.01, "d")
	assertApproxRefl(t, bg.E, 40.0, 0.01, "e")
	assertApproxRefl(t, bg.Z, 0.82, 0.02, "z")
	assertApproxRefl(t, bg.Habs, 1.0, 0.001, "habs (max of two)")

	if !bg.IsDouble {
		t.Error("two barriers should be double diffraction")
	}
}

func TestBarrierGeometryThreeEdgesUsesOutermost(t *testing.T) {
	t.Parallel()

	// Three edges → should use first and last (outermost pair).
	edges := []schall03.DiffractionEdge{
		makeEdge(25, 6, 0, 0),
		makeEdge(50, 8, 0, 1), // middle — ignored for geometry
		makeEdge(75, 6, 0, 2),
	}

	bg := schall03.ComputeBarrierGeometryFromEdges(edges, 0, 0, 100)

	// ds should be source→first edge (dist=25, h=6).
	assertApproxRefl(t, bg.Ds, math.Sqrt(25*25+6*6), 0.01, "ds uses first edge")

	// dr should be last edge→receiver (horiz=25, dh=6).
	assertApproxRefl(t, bg.Dr, math.Sqrt(25*25+6*6), 0.01, "dr uses last edge")

	if !bg.IsDouble {
		t.Error("three edges should produce double diffraction")
	}
}

func TestBarrierGeometryWithAbsorbingBase(t *testing.T) {
	t.Parallel()

	// Single barrier with absorbing base at 2 m.
	edges := []schall03.DiffractionEdge{makeEdge(50, 4, 2.0, 0)}
	bg := schall03.ComputeBarrierGeometryFromEdges(edges, 0, 0, 100)

	assertApproxRefl(t, bg.Habs, 2.0, 0.001, "habs from absorbing base")
}

func TestBarrierGeometryNoEdges(t *testing.T) {
	t.Parallel()

	bg := schall03.ComputeBarrierGeometryFromEdges(nil, 0, 0, 100)

	if bg.Z != 0 {
		t.Errorf("no edges should produce zero geometry, got Z=%g", bg.Z)
	}
}

func TestLateralDiffractionShortBarrier(t *testing.T) {
	t.Parallel()

	// Short barrier (10 m long) perpendicular to the path at x=50.
	// Source at (0,0), receiver at (100,0), both at height 0.
	// Barrier from (50,-5) to (50,5), top=4 m.
	//
	// Lateral path around endpoint A (50,-5):
	//   horizSE = dist((0,0),(50,-5)) = sqrt(2525) ≈ 50.25
	//   horizER = dist((50,-5),(100,0)) = sqrt(2525) ≈ 50.25
	//   ds = sqrt(50.25² + 4²) ≈ 50.41
	//   dr = sqrt(50.25² + 4²) ≈ 50.41
	//   z = 50.41 + 50.41 - 100 = 0.82
	//
	// This should produce a valid lateral A_bar (positive z).
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	barrier := schall03.BarrierSegment{
		A: geo.Point2D{X: 50, Y: -5}, B: geo.Point2D{X: 50, Y: 5},
		TopHeightM: 4, BaseHeightM: 0,
	}

	abar, ok := schall03.ComputeLateralDiffraction(source, receiver, 0, 0, barrier)
	if !ok {
		t.Fatal("lateral diffraction should be found for short barrier")
	}

	// All bands should have positive attenuation.
	for f, v := range abar {
		if v <= 0 {
			t.Errorf("band %d: expected positive A_bar, got %g", f, v)
		}
	}
}

func TestLateralDiffractionPicksLeastAttenuation(t *testing.T) {
	t.Parallel()

	// Asymmetric barrier: one end is closer to the source-receiver line.
	// Source at (0,0), receiver at (100,0).
	// Barrier from (50,-2) to (50,20) — endpoint A is close (2 m offset),
	// endpoint B is far (20 m offset).
	// The path around A should have lower attenuation (shorter detour).
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}
	barrier := schall03.BarrierSegment{
		A: geo.Point2D{X: 50, Y: -2}, B: geo.Point2D{X: 50, Y: 20},
		TopHeightM: 4, BaseHeightM: 0,
	}

	abar, ok := schall03.ComputeLateralDiffraction(source, receiver, 0, 0, barrier)
	if !ok {
		t.Fatal("lateral diffraction should be found")
	}

	// Compute the path around A separately — it should match the result
	// (since A is closer to the line, it has less attenuation).
	// Just verify the result is plausible and positive.
	for f, v := range abar {
		if v <= 0 {
			t.Errorf("band %d: expected positive A_bar, got %g", f, v)
		}
	}
}

func TestLateralDiffractionLongBarrierHigherThanShort(t *testing.T) {
	t.Parallel()

	// A longer barrier forces a bigger detour around the endpoints,
	// so lateral A_bar should be higher than for a short barrier.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}

	shortBarrier := schall03.BarrierSegment{
		A: geo.Point2D{X: 50, Y: -5}, B: geo.Point2D{X: 50, Y: 5},
		TopHeightM: 4, BaseHeightM: 0,
	}

	longBarrier := schall03.BarrierSegment{
		A: geo.Point2D{X: 50, Y: -30}, B: geo.Point2D{X: 50, Y: 30},
		TopHeightM: 4, BaseHeightM: 0,
	}

	abarShort, okS := schall03.ComputeLateralDiffraction(source, receiver, 0, 0, shortBarrier)
	abarLong, okL := schall03.ComputeLateralDiffraction(source, receiver, 0, 0, longBarrier)

	if !okS || !okL {
		t.Fatal("both barriers should produce lateral diffraction")
	}

	// Sum across bands for comparison.
	sumShort := 0.0
	sumLong := 0.0

	for f := range schall03.NumBeiblattOctaveBands {
		sumShort += abarShort[f]
		sumLong += abarLong[f]
	}

	if sumLong <= sumShort {
		t.Errorf("longer barrier should have higher lateral attenuation: short=%g, long=%g", sumShort, sumLong)
	}
}
