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
