package schall03_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestTable18AbsorptionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		surface schall03.WallSurfaceType
		want    float64
	}{
		{schall03.WallSurfaceHard, 0},
		{schall03.WallSurfaceBuilding, -1},
		{schall03.WallSurfaceAbsorbing, -4},
		{schall03.WallSurfaceHighlyAbsorbing, -8},
	}

	for _, tt := range tests {
		if got := schall03.Table18AbsorptionLoss(tt.surface); got != tt.want {
			t.Errorf("surface %d: want %g, got %g", tt.surface, tt.want, got)
		}
	}
}

func TestReflectingWallValidate(t *testing.T) {
	t.Parallel()

	valid := schall03.ReflectingWall{
		A:       geo.Point2D{X: 0, Y: 0},
		B:       geo.Point2D{X: 10, Y: 0},
		HeightM: 5,
		Surface: schall03.WallSurfaceBuilding,
	}

	err := valid.Validate()
	if err != nil {
		t.Errorf("valid wall should pass: %v", err)
	}

	// Zero-length wall should fail.
	degenerate := valid
	degenerate.B = degenerate.A

	err = degenerate.Validate()
	if err == nil {
		t.Error("zero-length wall should fail validation")
	}

	// Negative height should fail.
	negHeight := valid
	negHeight.HeightM = -1

	err = negHeight.Validate()
	if err == nil {
		t.Error("negative height should fail validation")
	}
}

func assertApproxRefl(t *testing.T, got, want, tol float64, label string) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s: want %g, got %g (tol %g)", label, want, got, tol)
	}
}

func TestMirrorSourceAcrossHorizontalWall(t *testing.T) {
	t.Parallel()

	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 0, Y: 5}, B: geo.Point2D{X: 10, Y: 5},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}
	source := geo.Point2D{X: 3, Y: 2}

	image, ok := schall03.MirrorSource(source, wall)
	if !ok {
		t.Fatal("mirror should succeed")
	}

	assertApproxRefl(t, image.X, 3.0, 0.001, "image X")
	assertApproxRefl(t, image.Y, 8.0, 0.001, "image Y")
}

func TestMirrorSourceAcrossVerticalWall(t *testing.T) {
	t.Parallel()

	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 0, Y: 0}, B: geo.Point2D{X: 0, Y: 10},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}
	source := geo.Point2D{X: 4, Y: 3}

	image, ok := schall03.MirrorSource(source, wall)
	if !ok {
		t.Fatal("mirror should succeed")
	}

	assertApproxRefl(t, image.X, -4.0, 0.001, "image X")
	assertApproxRefl(t, image.Y, 3.0, 0.001, "image Y")
}

func TestMirrorSourceAcrossDiagonalWall(t *testing.T) {
	t.Parallel()

	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 0, Y: 0}, B: geo.Point2D{X: 10, Y: 10},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}
	source := geo.Point2D{X: 0, Y: 4}

	image, ok := schall03.MirrorSource(source, wall)
	if !ok {
		t.Fatal("mirror should succeed")
	}

	assertApproxRefl(t, image.X, 4.0, 0.001, "image X")
	assertApproxRefl(t, image.Y, 0.0, 0.001, "image Y")
}

func TestFresnelCheckPassesLargeWall(t *testing.T) {
	t.Parallel()
	// Large wall (20 m), short distances → should pass easily.
	// d_so=10, d_or=10, f=63 Hz, λ=5.397 m
	// RHS = √(2·5.397 / (1/10 + 1/10)) = √(10.794 / 0.2) = √53.97 = 7.35 m
	// LHS = 20·cos(0°) = 20 > 7.35 → pass
	ok := schall03.FresnelCheck(20.0, 0.0, 10.0, 10.0)
	if !ok {
		t.Error("large wall should pass Fresnel check")
	}
}

func TestFresnelCheckFailsSmallWall(t *testing.T) {
	t.Parallel()
	// Small wall (2 m), same geometry.
	// LHS = 2·cos(0°) = 2 < 7.35 → fail
	ok := schall03.FresnelCheck(2.0, 0.0, 10.0, 10.0)
	if ok {
		t.Error("small wall should fail Fresnel check")
	}
}

func TestFresnelCheckAngledReflection(t *testing.T) {
	t.Parallel()
	// Wall at 60° angle → cos(β)=0.5.
	// l_min=20, d_so=10, d_or=10
	// LHS = 20·0.5 = 10 > 7.35 → pass
	ok := schall03.FresnelCheck(20.0, math.Pi/3, 10.0, 10.0)
	if !ok {
		t.Error("angled large wall should pass Fresnel check")
	}

	// Smaller wall at same angle.
	// l_min=6, cos(60°)=0.5 → LHS = 3 < 7.35 → fail
	ok = schall03.FresnelCheck(6.0, math.Pi/3, 10.0, 10.0)
	if ok {
		t.Error("angled small wall should fail Fresnel check")
	}
}

func TestFresnelCheckLargeDistance(t *testing.T) {
	t.Parallel()
	// Large distances increase the RHS.
	// d_so=100, d_or=100, l_min=5, β=0
	// RHS = √(2·5.397 / (1/100 + 1/100)) = √(10.794/0.02) = √539.7 = 23.23 m
	// LHS = 5 < 23.23 → fail
	ok := schall03.FresnelCheck(5.0, 0.0, 100.0, 100.0)
	if ok {
		t.Error("should fail at large distances with small wall")
	}
}

func TestReflectionGeometrySimple(t *testing.T) {
	t.Parallel()
	// Source at (0, 0), receiver at (10, 0).
	// Wall from (3, 3) to (7, 3) — parallel to x-axis, above source-receiver line.
	// Image of source across wall: (0, 6).
	// Line from image (0,6) to receiver (10,0): parametric intersection with y=3.
	// t = (6-3)/(6-0) = 0.5 → x = 0 + 0.5·10 = 5.0
	// Reflection point: (5, 3). Within wall segment [3,7] → valid.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 10, Y: 0}
	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 3, Y: 3}, B: geo.Point2D{X: 7, Y: 3},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}

	rg, ok := schall03.ComputeReflectionGeometry(source, receiver, wall)
	if !ok {
		t.Fatal("should find valid reflection point")
	}

	assertApproxRefl(t, rg.ReflectionPoint.X, 5.0, 0.01, "reflection X")
	assertApproxRefl(t, rg.ReflectionPoint.Y, 3.0, 0.01, "reflection Y")
	assertApproxRefl(t, rg.DSO, math.Sqrt(34), 0.01, "d_so")
	assertApproxRefl(t, rg.DOR, math.Sqrt(34), 0.01, "d_or")
}

func TestReflectionGeometryMissesWall(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 10, Y: 0}
	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 20, Y: 3}, B: geo.Point2D{X: 25, Y: 3},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}

	_, ok := schall03.ComputeReflectionGeometry(source, receiver, wall)
	if ok {
		t.Error("reflection point should miss the wall segment")
	}
}

func TestReflectionGeometrySourceBehindWall(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 10, Y: 0}
	// Wall above both → same side → valid.
	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 0, Y: 3}, B: geo.Point2D{X: 10, Y: 3},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}

	_, ok := schall03.ComputeReflectionGeometry(source, receiver, wall)
	if !ok {
		t.Error("same-side reflection should be valid")
	}

	// Receiver on the other side (y=5) → opposite sides → no reflection.
	receiverOther := geo.Point2D{X: 10, Y: 5}

	_, ok = schall03.ComputeReflectionGeometry(source, receiverOther, wall)
	if ok {
		t.Error("opposite-side should not produce a reflection")
	}
}

func TestMirrorSourceOnWall(t *testing.T) {
	t.Parallel()

	wall := schall03.ReflectingWall{
		A: geo.Point2D{X: 0, Y: 0}, B: geo.Point2D{X: 10, Y: 0},
		HeightM: 5, Surface: schall03.WallSurfaceHard,
	}
	source := geo.Point2D{X: 5, Y: 0}

	image, ok := schall03.MirrorSource(source, wall)
	if !ok {
		t.Fatal("mirror should succeed for point on wall")
	}

	assertApproxRefl(t, image.X, 5.0, 0.001, "image X")
	assertApproxRefl(t, image.Y, 0.0, 0.001, "image Y")
}

func TestReflectedContribIsLowerThanDirect(t *testing.T) {
	t.Parallel()
	// A reflected path with absorption loss should produce less energy than
	// the same geometry without loss.
	emission := &schall03.StreckeEmissionResult{
		PerHeight: map[int]schall03.BeiblattSpectrum{
			1: {80, 80, 80, 80, 80, 80, 80, 80},
		},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 50, Y: 0}, HeightM: 3.5,
	}

	noLoss := schall03.ReflectedSubsegmentContrib(
		emission, 0, receiver, 50.0, 10.0, 1.0, 0, 0,
	)

	withLoss := schall03.ReflectedSubsegmentContrib(
		emission, 0, receiver, 50.0, 10.0, 1.0, 0, -1,
	)

	if withLoss >= noLoss {
		t.Errorf("with D_ρ=-1 (%g) should be less than D_ρ=0 (%g)", withLoss, noLoss)
	}
}

func TestReflectedContribHardWallNoLoss(t *testing.T) {
	t.Parallel()
	// With D_ρ = 0 and same distance, reflected = direct (same propagation).
	emission := &schall03.StreckeEmissionResult{
		PerHeight: map[int]schall03.BeiblattSpectrum{
			1: {80, 80, 80, 80, 80, 80, 80, 80},
		},
	}

	receiver := schall03.ReceiverInput{
		ID: "r1", Point: geo.Point2D{X: 50, Y: 0}, HeightM: 3.5,
	}

	a := schall03.ReflectedSubsegmentContrib(
		emission, 0, receiver, 50.0, 10.0, 1.0, 0, 0,
	)

	b := schall03.ReflectedSubsegmentContrib(
		emission, 0, receiver, 50.0, 10.0, 1.0, 0, 0,
	)

	assertApproxRefl(t, a, b, 0.001, "hard wall same dist")
}

func TestEnumerateReflectionPaths1stOrder(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 10, Y: 0}
	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -5, Y: 3}, B: geo.Point2D{X: 15, Y: 3}, HeightM: 20, Surface: schall03.WallSurfaceHard},
	}

	paths := schall03.EnumerateReflectionPaths(source, receiver, walls, 1)

	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}

	if paths[0].Order != 1 {
		t.Errorf("expected order 1, got %d", paths[0].Order)
	}

	if len(paths[0].Walls) != 1 {
		t.Errorf("expected 1 wall in path, got %d", len(paths[0].Walls))
	}
}

func TestEnumerateReflectionPaths2ndOrder(t *testing.T) {
	t.Parallel()

	// Two parallel walls forming a canyon.
	source := geo.Point2D{X: 5, Y: 0}
	receiver := geo.Point2D{X: 15, Y: 0}
	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: 0, Y: 5}, B: geo.Point2D{X: 20, Y: 5}, HeightM: 10, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: 0, Y: -5}, B: geo.Point2D{X: 20, Y: -5}, HeightM: 10, Surface: schall03.WallSurfaceHard},
	}

	paths := schall03.EnumerateReflectionPaths(source, receiver, walls, 2)

	has1st := 0
	has2nd := 0

	for _, p := range paths {
		switch p.Order {
		case 1:
			has1st++
		case 2:
			has2nd++
		}
	}

	if has1st < 2 {
		t.Errorf("expected at least 2 first-order paths, got %d", has1st)
	}

	if has2nd < 1 {
		t.Errorf("expected at least 1 second-order path, got %d", has2nd)
	}
}

func TestEnumerateReflectionPathsNoDoubleWall(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 5, Y: 0}
	receiver := geo.Point2D{X: 15, Y: 0}
	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: 0, Y: 5}, B: geo.Point2D{X: 20, Y: 5}, HeightM: 10, Surface: schall03.WallSurfaceHard},
	}

	paths := schall03.EnumerateReflectionPaths(source, receiver, walls, 3)
	for _, p := range paths {
		for i := 1; i < len(p.Walls); i++ {
			if p.Walls[i] == p.Walls[i-1] {
				t.Errorf("path has consecutive bounces off same wall index %d", p.Walls[i])
			}
		}
	}
}

func TestEnumerateReflectionPathsMaxOrder3(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 5, Y: 0}
	receiver := geo.Point2D{X: 15, Y: 0}
	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: 0, Y: 5}, B: geo.Point2D{X: 20, Y: 5}, HeightM: 10, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: 0, Y: -5}, B: geo.Point2D{X: 20, Y: -5}, HeightM: 10, Surface: schall03.WallSurfaceHard},
	}

	paths := schall03.EnumerateReflectionPaths(source, receiver, walls, 3)
	for _, p := range paths {
		if p.Order > 3 {
			t.Errorf("path has order %d, max is 3", p.Order)
		}
	}
}

// The fully unfolded origin of a reflected path is the image source of its LAST
// bounce, not its first.  That is the invariant the barrier check on reflected
// paths rests on, so it is asserted here on the enumerator itself, without any
// barriers in play.
//
// Why the last one is the right origin: EnumerateReflectionPaths solves each
// bounce from the previous image source, so ImageSource[n] is the source
// mirrored across wall 1 … wall n in order, and TotalDist is measured from that
// same point.  If the two ever disagreed, a diffraction check run from the
// unfolded origin would follow a different ray than the propagation distance.
func TestReflectionPathEffectiveSourceIsTheUnfoldedOrigin(t *testing.T) {
	t.Parallel()

	// A canyon deep enough to survive the Fresnel check at orders 1, 2 and 3.
	source := geo.Point2D{X: 5, Y: 0}
	receiver := geo.Point2D{X: 60, Y: 0}
	walls := []schall03.ReflectingWall{
		{A: geo.Point2D{X: -50, Y: 12}, B: geo.Point2D{X: 150, Y: 12}, HeightM: 40, Surface: schall03.WallSurfaceHard},
		{A: geo.Point2D{X: -50, Y: -12}, B: geo.Point2D{X: 150, Y: -12}, HeightM: 40, Surface: schall03.WallSurfaceHard},
	}

	paths := schall03.EnumerateReflectionPaths(source, receiver, walls, schall03.MaxReflectionOrder)

	seenOrders := map[int]int{}

	for _, rp := range paths {
		seenOrders[rp.Order]++

		// The propagation distance and the unfolded ray must agree.
		assertApproxRefl(t, geo.Distance(rp.EffectiveSource(), receiver), rp.TotalDist, 1e-9,
			fmt.Sprintf("order %d walls %v: distance from EffectiveSource", rp.Order, rp.Walls))

		if rp.Order == 1 {
			// Order 1 is the one case where first and last coincide.
			assertApproxRefl(t, rp.EffectiveSource().X, rp.Geometries[0].ImageSource.X, 1e-12, "order 1 image X")
			assertApproxRefl(t, rp.EffectiveSource().Y, rp.Geometries[0].ImageSource.Y, 1e-12, "order 1 image Y")
		}

		if rp.Order < 2 {
			continue
		}

		// Higher orders: the unfolded origin is the source mirrored across
		// every wall of the path, in bounce order — and it is NOT the first
		// bounce's image source.
		want := source
		for _, idx := range rp.Walls {
			m, ok := schall03.MirrorSource(want, walls[idx])
			if !ok {
				t.Fatalf("order %d walls %v: MirrorSource failed", rp.Order, rp.Walls)
			}

			want = m
		}

		assertApproxRefl(t, rp.EffectiveSource().X, want.X, 1e-9,
			fmt.Sprintf("order %d walls %v: unfolded image X", rp.Order, rp.Walls))
		assertApproxRefl(t, rp.EffectiveSource().Y, want.Y, 1e-9,
			fmt.Sprintf("order %d walls %v: unfolded image Y", rp.Order, rp.Walls))

		if geo.Distance(rp.EffectiveSource(), rp.Geometries[0].ImageSource) < 1e-6 {
			t.Errorf("order %d walls %v: unfolded origin coincides with the first bounce's image %v — "+
				"the scene no longer distinguishes the two, so this test proves nothing",
				rp.Order, rp.Walls, rp.Geometries[0].ImageSource)
		}
	}

	for order := 1; order <= schall03.MaxReflectionOrder; order++ {
		if seenOrders[order] == 0 {
			t.Errorf("expected at least one order-%d path, got none (orders seen: %v)", order, seenOrders)
		}
	}
}

// EffectiveSource is exported, so it must answer for a path the enumerator never
// builds rather than panicking on an empty Geometries slice.
func TestReflectionPathEffectiveSourceEmpty(t *testing.T) {
	t.Parallel()

	var empty schall03.ReflectionPath

	got := empty.EffectiveSource()
	if got != (geo.Point2D{}) {
		t.Errorf("EffectiveSource of an empty path = %v, want the zero point", got)
	}
}
