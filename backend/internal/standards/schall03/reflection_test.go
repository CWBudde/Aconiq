package schall03_test

import (
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
