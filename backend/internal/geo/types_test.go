package geo

import (
	"math"
	"testing"
)

func TestPoint3DHelpers(t *testing.T) {
	t.Parallel()

	p := Point3D{X: 1, Y: 2, Z: 3}
	if !p.IsFinite() {
		t.Fatal("expected finite point")
	}

	if xy := p.XY(); xy != (Point2D{X: 1, Y: 2}) {
		t.Fatalf("unexpected xy projection %#v", xy)
	}

	if (Point3D{X: math.NaN(), Y: 2, Z: 3}).IsFinite() {
		t.Fatal("expected non-finite point")
	}
}

func TestBBoxUtilities(t *testing.T) {
	t.Parallel()

	b, err := NewBBox(-1, -2, 3, 4)
	if err != nil {
		t.Fatalf("new bbox: %v", err)
	}

	if b.Width() != 4 || b.Height() != 6 {
		t.Fatalf("unexpected bbox dimensions %#v", b)
	}

	if !b.ContainsPoint(Point2D{X: 0, Y: 0}) {
		t.Fatal("expected point to be inside bbox")
	}

	if b.ContainsPoint(Point2D{X: 5, Y: 0}) {
		t.Fatal("expected point to be outside bbox")
	}

	expanded := b.ExpandToIncludeBBox(BBox{MinX: -5, MinY: -1, MaxX: 2, MaxY: 8})
	if expanded != (BBox{MinX: -5, MinY: -2, MaxX: 3, MaxY: 8}) {
		t.Fatalf("unexpected expanded bbox %#v", expanded)
	}
}

func TestNewBBoxErrors(t *testing.T) {
	t.Parallel()

	if _, err := NewBBox(math.NaN(), 0, 1, 1); err == nil {
		t.Fatal("expected non-finite bbox error")
	}

	if _, err := NewBBox(2, 0, 1, 1); err == nil {
		t.Fatal("expected invalid bbox error")
	}
}
