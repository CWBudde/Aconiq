package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestSegmentCrossingWithinMatchesSegmentIntersection holds the zero-tolerance
// form to SegmentIntersection, to the bit, over random rays and edges: the
// tolerance is the only thing it adds.
func TestSegmentCrossingWithinMatchesSegmentIntersection(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 13))
	coord := func() float64 { return math.Round(rng.Float64()*40-20) / 2 }

	for range 200_000 {
		a := Point2D{X: coord(), Y: coord()}
		b := Point2D{X: coord(), Y: coord()}
		p1 := Point2D{X: coord(), Y: coord()}
		p2 := Point2D{X: coord(), Y: coord()}

		got, gotOK := SegmentCrossingWithin(a, b, p1, p2, 0)
		want, _, wantOK := SegmentIntersection(p1, p2, a, b)

		if gotOK != wantOK ||
			math.Float64bits(got.X) != math.Float64bits(want.X) ||
			math.Float64bits(got.Y) != math.Float64bits(want.Y) {
			t.Fatalf("edge %v→%v, ray %v→%v: got (%v, %v), SegmentIntersection (%v, %v)",
				a, b, p1, p2, got, gotOK, want, wantOK)
		}
	}
}

// TestSegmentCrossingWithinLengthensTheEdge checks the tolerance at both ends
// of the edge: a crossing on the edge's line within it is accepted, one beyond
// it is not, and the crossing point is where the ray meets the line.
func TestSegmentCrossingWithinLengthensTheEdge(t *testing.T) {
	t.Parallel()

	const tol = 1e-3

	a := Point2D{X: 0, Y: 0}
	b := Point2D{X: 10, Y: 0}

	tests := []struct {
		name   string
		x      float64
		wantOK bool
	}{
		{"inside", 4, true},
		{"on the first end", 0, true},
		{"half the tolerance before the first end", -tol / 2, true},
		{"twice the tolerance before the first end", -2 * tol, false},
		{"half the tolerance past the second end", 10 + tol/2, true},
		{"twice the tolerance past the second end", 10 + 2*tol, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			p, ok := SegmentCrossingWithin(a, b, Point2D{X: test.x, Y: -3}, Point2D{X: test.x, Y: 5}, tol)
			if ok != test.wantOK {
				t.Fatalf("accepted = %v, want %v", ok, test.wantOK)
			}

			if ok && (math.Abs(p.X-test.x) > 1e-12 || p.Y != 0) {
				t.Fatalf("crossing at %v, want (%v, 0)", p, test.x)
			}
		})
	}
}

// TestSegmentCrossingWithinKeepsACornerHitAcrossARounding is the RLS-19
// CI-safe K3 case. The specular point of a Teilstück at x = 2.5 off a facade
// at x = 5 lands exactly on the facade's corner (5, 5). An arm64 build fused
// the Teilstück's midpoint into 2.5000000000000022, its image into
// 7.4999999999999982, and the exact test then missed the corner by a rounding
// and dropped the path.
func TestSegmentCrossingWithinKeepsACornerHitAcrossARounding(t *testing.T) {
	t.Parallel()

	a := Point2D{X: 5, Y: 5}
	b := Point2D{X: 5, Y: 25}
	receiver := Point2D{X: 0, Y: 15}

	for _, image := range []Point2D{{X: 7.5, Y: 0}, {X: 7.4999999999999982, Y: 0}} {
		p, ok := SegmentCrossingWithin(a, b, image, receiver, 1e-7)
		if !ok || math.Abs(p.Y-5) > 1e-12 {
			t.Errorf("image %v: got (%v, %v), want the corner (5, 5)", image, p, ok)
		}
	}

	if _, ok := SegmentCrossingWithin(a, b, Point2D{X: 7.4999999999999982, Y: 0}, receiver, 0); ok {
		t.Error("without a tolerance the rounded image is expected to miss the corner; " +
			"if it no longer does, this test no longer shows the tolerance at work")
	}
}
