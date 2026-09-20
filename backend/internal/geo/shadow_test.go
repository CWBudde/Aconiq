package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

// shadowedPoint constructs a point that is in the shadow by definition —
// apex + λ·(q − apex) for a q on [a,b] and a λ ≥ 1 — rather than testing a
// random point against a sampled approximation of the region. The guard below
// has to be tighter than the margin it is guarding, and only construction is.
func shadowedPoint(apex, a, b Point2D, s, lambda float64) Point2D {
	q := Point2D{X: a.X + s*(b.X-a.X), Y: a.Y + s*(b.Y-a.Y)}

	return Point2D{X: apex.X + lambda*(q.X-apex.X), Y: apex.Y + lambda*(q.Y-apex.Y)}
}

// TestSegmentShadowCoversEveryShadowedPoint is the exactness guard: whatever
// the shadow truly contains must survive both the box and the segment test.
func TestSegmentShadowCoversEveryShadowedPoint(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 11))

	for range 300 {
		apex := Point2D{X: rng.Float64()*400 - 200, Y: rng.Float64()*400 - 200}
		a := Point2D{X: rng.Float64()*400 - 200, Y: rng.Float64()*400 - 200}
		b := Point2D{X: rng.Float64()*400 - 200, Y: rng.Float64()*400 - 200}

		shadow := NewSegmentShadow(apex, a, b, 1e-3)

		for range 40 {
			p := shadowedPoint(apex, a, b, rng.Float64(), 1+rng.Float64()*4)
			if !p.IsFinite() {
				continue
			}

			if !shadow.MayContainSegment(p, p) {
				t.Fatalf("shadow of (%v,%v) from %v rejects shadowed point %v", a, b, apex, p)
			}

			bounds := BBox{
				MinX: math.Floor(p.X) - 1, MinY: math.Floor(p.Y) - 1,
				MaxX: math.Ceil(p.X) + 1, MaxY: math.Ceil(p.Y) + 1,
			}

			box, reachable := shadow.Bounds(bounds, 1e-3)
			if !reachable {
				t.Fatalf("shadow reported empty over bounds holding %v", p)
			}

			if !box.ContainsPoint(p) {
				t.Fatalf("shadow box %v misses shadowed point %v", box, p)
			}
		}
	}
}

// TestSegmentShadowNarrowsSomething keeps the prune honest: a shadow that
// accepted everything would pass the guard above and buy nothing.
func TestSegmentShadowNarrowsSomething(t *testing.T) {
	t.Parallel()

	// A short segment seen from close by casts a narrow shadow.
	shadow := NewSegmentShadow(Point2D{X: 0, Y: 0}, Point2D{X: -3, Y: 10}, Point2D{X: 3, Y: 10}, 1e-3)

	if shadow.MayContainSegment(Point2D{X: 0, Y: -5}, Point2D{X: 0, Y: -1}) {
		t.Fatal("a segment behind the apex is not in the shadow")
	}

	if shadow.MayContainSegment(Point2D{X: 100, Y: 20}, Point2D{X: 120, Y: 20}) {
		t.Fatal("a segment well outside the wedge is not in the shadow")
	}

	if !shadow.MayContainSegment(Point2D{X: -2, Y: 20}, Point2D{X: 2, Y: 20}) {
		t.Fatal("a segment straight down the wedge is in the shadow")
	}

	box, ok := shadow.Bounds(BBox{MinX: -1000, MinY: -1000, MaxX: 1000, MaxY: 1000}, 1e-3)
	if !ok {
		t.Fatal("the shadow reaches into these bounds")
	}

	if box.MaxY > 1000.01 || box.MinY > 10.01 {
		t.Fatalf("shadow box %v should start at the segment and run away from the apex", box)
	}
}

// TestSegmentShadowDegenerateNarrowsNothing pins the rule that an apex on the
// segment's own line gives up rather than guessing.
func TestSegmentShadowDegenerateNarrowsNothing(t *testing.T) {
	t.Parallel()

	shadow := NewSegmentShadow(Point2D{X: 0, Y: 0}, Point2D{X: 1, Y: 0}, Point2D{X: 5, Y: 0}, 1e-3)

	if !shadow.MayContainSegment(Point2D{X: -50, Y: -50}, Point2D{X: -40, Y: -40}) {
		t.Fatal("a degenerate shadow must reject nothing")
	}

	bounds := BBox{MinX: -10, MinY: -10, MaxX: 10, MaxY: 10}

	box, ok := shadow.Bounds(bounds, 1e-3)
	if !ok || box != bounds {
		t.Fatalf("a degenerate shadow must answer with the whole bounds, got %v %v", box, ok)
	}
}
