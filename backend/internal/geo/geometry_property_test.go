package geo

import (
	"math"
	"math/rand"
	"testing"
)

func TestDistancePointToSegmentProperties(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 2000; i++ {
		p := Point2D{X: rng.Float64()*200 - 100, Y: rng.Float64()*200 - 100}
		a := Point2D{X: rng.Float64()*200 - 100, Y: rng.Float64()*200 - 100}
		b := Point2D{X: rng.Float64()*200 - 100, Y: rng.Float64()*200 - 100}

		d1 := DistancePointToSegment(p, a, b)
		d2 := DistancePointToSegment(p, b, a)

		if math.IsNaN(d1) || d1 < 0 {
			t.Fatalf("distance must be finite and non-negative, got %f", d1)
		}
		if math.Abs(d1-d2) > 1e-9 {
			t.Fatalf("distance should be symmetric in segment endpoints: %f vs %f", d1, d2)
		}
	}
}
