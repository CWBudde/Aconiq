package geo

import (
	"math"
	"testing"
)

func TestDistancePointToSegment(t *testing.T) {
	t.Parallel()

	d := DistancePointToSegment(Point2D{X: 5, Y: 2}, Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0})
	if math.Abs(d-2) > 1e-9 {
		t.Fatalf("expected distance 2, got %.12f", d)
	}
}

func TestPointInPolygonWithHole(t *testing.T) {
	t.Parallel()

	rings := [][]Point2D{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
		{{3, 3}, {7, 3}, {7, 7}, {3, 7}, {3, 3}},
	}

	if !PointInPolygon(Point2D{X: 2, Y: 2}, rings) {
		t.Fatal("expected point to be inside polygon")
	}
	if PointInPolygon(Point2D{X: 4, Y: 4}, rings) {
		t.Fatal("expected point inside hole to be outside polygon")
	}
	if !PointInPolygon(Point2D{X: 0, Y: 5}, rings) {
		t.Fatal("expected edge point to be treated as inside")
	}
}

func TestSegmentIntersection_Crossing(t *testing.T) {
	t.Parallel()

	// Two crossing segments.
	pt, tVal, ok := SegmentIntersection(
		Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0},
		Point2D{X: 5, Y: -5}, Point2D{X: 5, Y: 5},
	)
	if !ok {
		t.Fatal("expected intersection")
	}

	if math.Abs(pt.X-5) > 1e-9 || math.Abs(pt.Y) > 1e-9 {
		t.Fatalf("expected (5,0), got (%f,%f)", pt.X, pt.Y)
	}

	if math.Abs(tVal-0.5) > 1e-9 {
		t.Fatalf("expected t=0.5, got %f", tVal)
	}
}

func TestSegmentIntersection_NoIntersection(t *testing.T) {
	t.Parallel()

	// Parallel segments.
	_, _, ok := SegmentIntersection(
		Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0},
		Point2D{X: 0, Y: 1}, Point2D{X: 10, Y: 1},
	)
	if ok {
		t.Fatal("expected no intersection for parallel segments")
	}

	// Non-overlapping segments.
	_, _, ok = SegmentIntersection(
		Point2D{X: 0, Y: 0}, Point2D{X: 5, Y: 0},
		Point2D{X: 6, Y: -1}, Point2D{X: 6, Y: 1},
	)
	if ok {
		t.Fatal("expected no intersection for non-overlapping segments")
	}
}

func TestLineStringIntersectsSegment(t *testing.T) {
	t.Parallel()

	// Barrier polyline running north-south at x=5.
	barrier := []Point2D{{X: 5, Y: -10}, {X: 5, Y: 10}}

	// Source-receiver line crossing the barrier.
	pt, edge, ok := LineStringIntersectsSegment(barrier, Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0})
	if !ok {
		t.Fatal("expected intersection")
	}

	if math.Abs(pt.X-5) > 1e-9 || math.Abs(pt.Y) > 1e-9 {
		t.Fatalf("expected (5,0), got (%f,%f)", pt.X, pt.Y)
	}

	if edge != 0 {
		t.Fatalf("expected edge 0, got %d", edge)
	}

	// Line that does not cross the barrier.
	_, _, ok = LineStringIntersectsSegment(barrier, Point2D{X: 0, Y: 0}, Point2D{X: 3, Y: 0})
	if ok {
		t.Fatal("expected no intersection")
	}
}

func TestBBoxFromPoints(t *testing.T) {
	t.Parallel()

	bbox, ok := BBoxFromPoints([]Point2D{{X: 1, Y: 2}, {X: -3, Y: 10}, {X: 5, Y: -2}})
	if !ok {
		t.Fatal("expected bbox")
	}
	if bbox.MinX != -3 || bbox.MinY != -2 || bbox.MaxX != 5 || bbox.MaxY != 10 {
		t.Fatalf("unexpected bbox %#v", bbox)
	}
}
