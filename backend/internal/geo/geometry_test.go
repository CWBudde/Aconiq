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
