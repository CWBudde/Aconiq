package geo

import (
	"math"
	"testing"
)

// bentLine is 100 m east, then 50 m north: total length 150 m, one interior
// vertex at chainage 100.
var bentLine = []Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}}

func TestLineStringLength(t *testing.T) {
	t.Parallel()

	if got := LineStringLength(bentLine); math.Abs(got-150) > 1e-9 {
		t.Fatalf("expected length 150, got %.12f", got)
	}

	if got := LineStringLength([]Point2D{{X: 1, Y: 1}}); got != 0 {
		t.Fatalf("expected 0 for a single vertex, got %v", got)
	}

	if got := LineStringLength([]Point2D{{X: 0, Y: 0}, {X: math.NaN(), Y: 0}}); !math.IsNaN(got) {
		t.Fatalf("expected NaN for a non-finite vertex, got %v", got)
	}
}

func TestProjectPointOntoLineString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		point        Point2D
		wantChainage float64
		wantOffset   float64
	}{
		{"beside the first leg", Point2D{X: 40, Y: 7}, 40, 7},
		{"beside the second leg", Point2D{X: 103, Y: 20}, 120, 3},
		{"before the start clamps to it", Point2D{X: -30, Y: 0}, 0, 30},
		{"past the end clamps to it", Point2D{X: 100, Y: 80}, 150, 30},
		{"on the interior vertex", Point2D{X: 100, Y: 0}, 100, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			chainage, offset, ok := ProjectPointOntoLineString(tc.point, bentLine)
			if !ok {
				t.Fatal("expected ok")
			}

			if math.Abs(chainage-tc.wantChainage) > 1e-9 || math.Abs(offset-tc.wantOffset) > 1e-9 {
				t.Fatalf(
					"expected chainage %.3f offset %.3f, got %.12f / %.12f",
					tc.wantChainage, tc.wantOffset, chainage, offset,
				)
			}
		})
	}

	if _, _, ok := ProjectPointOntoLineString(Point2D{X: 0, Y: 0}, []Point2D{{X: 1, Y: 1}}); ok {
		t.Fatal("expected not ok for a line with fewer than two vertices")
	}
}

// TestSliceLineStringReproducesTheWholeLine is the property the Nr. 5.3.2 speed
// split relies on: a segment whose substitution zone covers everything must
// come back bit-for-bit identical, or splitting would move goldens on its own.
func TestSliceLineStringReproducesTheWholeLine(t *testing.T) {
	t.Parallel()

	got, ok := SliceLineString(bentLine, 0, 150)
	if !ok {
		t.Fatal("expected ok")
	}

	if len(got) != len(bentLine) {
		t.Fatalf("expected %d vertices, got %d: %v", len(bentLine), len(got), got)
	}

	for i := range got {
		if got[i] != bentLine[i] {
			t.Fatalf("vertex %d is %v, want %v", i, got[i], bentLine[i])
		}
	}
}

func TestSliceLineStringKeepsInteriorVertices(t *testing.T) {
	t.Parallel()

	got, ok := SliceLineString(bentLine, 90, 120)
	if !ok {
		t.Fatal("expected ok")
	}

	want := []Point2D{{X: 90, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 20}}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	for i := range got {
		if math.Abs(got[i].X-want[i].X) > 1e-9 || math.Abs(got[i].Y-want[i].Y) > 1e-9 {
			t.Fatalf("vertex %d is %v, want %v", i, got[i], want[i])
		}
	}
}

// TestSliceLineStringPartitionsLength pins that cutting a line in two conserves
// its length, which is what makes the emission split energy-preserving.
func TestSliceLineStringPartitionsLength(t *testing.T) {
	t.Parallel()

	head, ok := SliceLineString(bentLine, 0, 37)
	if !ok {
		t.Fatal("expected ok for the head")
	}

	tail, ok := SliceLineString(bentLine, 37, 150)
	if !ok {
		t.Fatal("expected ok for the tail")
	}

	if sum := LineStringLength(head) + LineStringLength(tail); math.Abs(sum-150) > 1e-9 {
		t.Fatalf("expected the parts to sum to 150, got %.12f", sum)
	}
}

func TestSliceLineStringRejectsAnEmptySpan(t *testing.T) {
	t.Parallel()

	if _, ok := SliceLineString(bentLine, 60, 60); ok {
		t.Fatal("expected not ok for a zero-length span")
	}

	if _, ok := SliceLineString(bentLine, 200, 300); ok {
		t.Fatal("expected not ok for a span beyond the line")
	}
}

func TestPointAtChainageClampsToTheEnds(t *testing.T) {
	t.Parallel()

	if got := PointAtChainage(bentLine, -5); got != bentLine[0] {
		t.Fatalf("expected the start vertex, got %v", got)
	}

	if got := PointAtChainage(bentLine, 500); got != bentLine[len(bentLine)-1] {
		t.Fatalf("expected the end vertex, got %v", got)
	}
}
