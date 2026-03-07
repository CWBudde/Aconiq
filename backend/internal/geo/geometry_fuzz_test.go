package geo

import (
	"math"
	"testing"
)

func FuzzDistancePointToSegment(f *testing.F) {
	f.Add(0.0, 1.0, 0.0, 0.0, 10.0, 0.0)
	f.Add(5.0, 0.0, -2.0, 1.0, 3.0, 7.0)

	f.Fuzz(func(t *testing.T, px, py, ax, ay, bx, by float64) {
		p := Point2D{X: px, Y: py}
		a := Point2D{X: ax, Y: ay}

		b := Point2D{X: bx, Y: by}
		if !p.IsFinite() || !a.IsFinite() || !b.IsFinite() {
			t.Skip()
		}

		d := DistancePointToSegment(p, a, b)
		if math.IsNaN(d) || math.IsInf(d, 0) || d < 0 {
			t.Fatalf("invalid distance result: %f", d)
		}
	})
}
