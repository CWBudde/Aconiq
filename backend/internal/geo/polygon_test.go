package geo

import (
	"math"
	"testing"
)

func closedRing(points ...Point2D) []Point2D {
	return append(points, points[0])
}

var unitSquare = [][]Point2D{closedRing(
	Point2D{X: 0, Y: 0},
	Point2D{X: 1, Y: 0},
	Point2D{X: 1, Y: 1},
	Point2D{X: 0, Y: 1},
)}

func TestPolygonArea(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rings [][]Point2D
		want  float64
	}{
		{name: "unit square", rings: unitSquare, want: 1},
		{
			name: "rectangle",
			rings: [][]Point2D{closedRing(
				Point2D{X: -10, Y: -5}, Point2D{X: 10, Y: -5},
				Point2D{X: 10, Y: 5}, Point2D{X: -10, Y: 5},
			)},
			want: 200,
		},
		{
			// Winding order must not change the magnitude: PolygonArea takes
			// the absolute area of every ring.
			name: "clockwise winding",
			rings: [][]Point2D{closedRing(
				Point2D{X: 0, Y: 0}, Point2D{X: 0, Y: 1},
				Point2D{X: 1, Y: 1}, Point2D{X: 1, Y: 0},
			)},
			want: 1,
		},
		{
			name: "square with a square hole",
			rings: [][]Point2D{
				closedRing(
					Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0},
					Point2D{X: 10, Y: 10}, Point2D{X: 0, Y: 10},
				),
				closedRing(
					Point2D{X: 2, Y: 2}, Point2D{X: 4, Y: 2},
					Point2D{X: 4, Y: 4}, Point2D{X: 2, Y: 4},
				),
			},
			want: 96,
		},
		{
			// A hole larger than its exterior is malformed input; clamping to 0
			// keeps callers from taking sqrt or lg of a negative area.
			name: "hole larger than exterior clamps to zero",
			rings: [][]Point2D{
				unitSquare[0],
				closedRing(
					Point2D{X: 0, Y: 0}, Point2D{X: 5, Y: 0},
					Point2D{X: 5, Y: 5}, Point2D{X: 0, Y: 5},
				),
			},
			want: 0,
		},
		{name: "no rings", rings: nil, want: 0},
		{
			name:  "collinear ring encloses nothing",
			rings: [][]Point2D{closedRing(Point2D{X: 0, Y: 0}, Point2D{X: 1, Y: 0}, Point2D{X: 2, Y: 0})},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := PolygonArea(tt.rings)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("PolygonArea() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolygonCentroid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rings [][]Point2D
		want  Point2D
	}{
		{name: "unit square", rings: unitSquare, want: Point2D{X: 0.5, Y: 0.5}},
		{
			name: "rectangle off the origin",
			rings: [][]Point2D{closedRing(
				Point2D{X: 100, Y: 200}, Point2D{X: 300, Y: 200},
				Point2D{X: 300, Y: 300}, Point2D{X: 100, Y: 300},
			)},
			want: Point2D{X: 200, Y: 250},
		},
		{
			// Reversing the winding negates both the numerator and the double
			// area, so the centroid is unchanged.
			name: "clockwise winding gives the same centroid",
			rings: [][]Point2D{closedRing(
				Point2D{X: 0, Y: 0}, Point2D{X: 0, Y: 1},
				Point2D{X: 1, Y: 1}, Point2D{X: 1, Y: 0},
			)},
			want: Point2D{X: 0.5, Y: 0.5},
		},
		{
			// A hole centred on the exterior removes area symmetrically, so the
			// Flächenschwerpunkt does not move. This is the case that made the
			// old exterior-only implementation look correct.
			name: "a centred hole leaves the centroid where it was",
			rings: [][]Point2D{
				unitSquare[0],
				closedRing(
					Point2D{X: 0.4, Y: 0.4}, Point2D{X: 0.6, Y: 0.4},
					Point2D{X: 0.6, Y: 0.6}, Point2D{X: 0.4, Y: 0.6},
				),
			},
			want: Point2D{X: 0.5, Y: 0.5},
		},
		{
			// An off-centre hole does move it, away from the removed area.
			// Exterior: A = 1 at (0.5, 0.5). Hole: A = 0.04 at (0.1, 0.1).
			// C_x = (1·0.5 − 0.04·0.1) / 0.96 = 0.496 / 0.96 = 0.51666…, same
			// in y — the centroid moves away from the corner the hole removes.
			name: "an off-centre hole shifts the centroid away from it",
			rings: [][]Point2D{
				unitSquare[0],
				closedRing(
					Point2D{X: 0, Y: 0}, Point2D{X: 0.2, Y: 0},
					Point2D{X: 0.2, Y: 0.2}, Point2D{X: 0, Y: 0.2},
				),
			},
			want: Point2D{X: 0.5166666666666667, Y: 0.5166666666666667},
		},
		{
			// A hole wound the same way as its exterior is still a hole: ring
			// position decides, not winding, so PolygonCentroid and PolygonArea
			// agree on which ring is which.
			name: "a hole is identified by position, not winding",
			rings: [][]Point2D{
				unitSquare[0],
				closedRing(
					Point2D{X: 0, Y: 0}, Point2D{X: 0, Y: 0.2},
					Point2D{X: 0.2, Y: 0.2}, Point2D{X: 0.2, Y: 0},
				),
			},
			want: Point2D{X: 0.5166666666666667, Y: 0.5166666666666667},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := PolygonCentroid(tt.rings)
			if !ok {
				t.Fatalf("PolygonCentroid() reported undefined, want %v", tt.want)
			}

			if math.Abs(got.X-tt.want.X) > 1e-9 || math.Abs(got.Y-tt.want.Y) > 1e-9 {
				t.Errorf("PolygonCentroid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPolygonCentroidUndefined pins the cases a caller must refuse rather than
// place a source at an arbitrary point.
func TestPolygonCentroidUndefined(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rings [][]Point2D
	}{
		{name: "no rings", rings: nil},
		{name: "empty ring", rings: [][]Point2D{{}}},
		{
			name:  "ring shorter than four points",
			rings: [][]Point2D{{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0}}},
		},
		{
			name:  "collinear ring encloses no area",
			rings: [][]Point2D{closedRing(Point2D{X: 0, Y: 0}, Point2D{X: 1, Y: 0}, Point2D{X: 2, Y: 0})},
		},
		{
			name:  "degenerate ring collapsed to a point",
			rings: [][]Point2D{closedRing(Point2D{X: 5, Y: 5}, Point2D{X: 5, Y: 5}, Point2D{X: 5, Y: 5})},
		},
		{
			name: "non-finite vertex",
			rings: [][]Point2D{closedRing(
				Point2D{X: 0, Y: 0}, Point2D{X: math.NaN(), Y: 0},
				Point2D{X: 1, Y: 1}, Point2D{X: 0, Y: 1},
			)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := PolygonCentroid(tt.rings)
			if ok {
				t.Errorf("PolygonCentroid() = %v, true; want undefined", got)
			}
		})
	}
}
