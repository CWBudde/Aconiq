package schall03_test

import (
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
