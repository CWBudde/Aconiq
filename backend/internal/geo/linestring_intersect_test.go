package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

// lineStringIntersectsSegmentReference is LineStringIntersectsSegment as it
// stood before its loop was made branch-free: one SegmentIntersection per
// edge, nearest crossing kept. The rewrite has to answer with the same bits.
func lineStringIntersectsSegmentReference(line []Point2D, p1, p2 Point2D) (Point2D, int, bool) {
	if len(line) < 2 {
		return Point2D{}, 0, false
	}

	bestT := math.MaxFloat64
	bestPt := Point2D{}
	bestEdge := -1

	for i := range len(line) - 1 {
		pt, t, ok := SegmentIntersection(p1, p2, line[i], line[i+1])
		if ok && t < bestT {
			bestT = t
			bestPt = pt
			bestEdge = i
		}
	}

	if bestEdge < 0 {
		return Point2D{}, 0, false
	}

	return bestPt, bestEdge, true
}

func assertSameLineStringCrossing(t *testing.T, line []Point2D, p1, p2 Point2D) {
	t.Helper()

	gotPt, gotEdge, gotOK := LineStringIntersectsSegment(line, p1, p2)
	wantPt, wantEdge, wantOK := lineStringIntersectsSegmentReference(line, p1, p2)

	if gotOK != wantOK || gotEdge != wantEdge ||
		math.Float64bits(gotPt.X) != math.Float64bits(wantPt.X) ||
		math.Float64bits(gotPt.Y) != math.Float64bits(wantPt.Y) {
		t.Fatalf("line %v, ray %v→%v: got (%v, %d, %v), reference (%v, %d, %v)",
			line, p1, p2, gotPt, gotEdge, gotOK, wantPt, wantEdge, wantOK)
	}
}

// TestLineStringIntersectsSegmentBoundaries pins the cases where the bound
// tests sit exactly on their limits: a crossing at either end of the ray or
// of the edge, a parallel edge, an edge whose cross product lands on the
// 1e-12 cut, and an edge that degenerates to a point.
func TestLineStringIntersectsSegmentBoundaries(t *testing.T) {
	t.Parallel()

	origin := Point2D{X: 0, Y: 0}
	east := Point2D{X: 10, Y: 0}

	cases := map[string][]Point2D{
		"crossing at the ray's start (t = 0)":       {{X: 0, Y: -1}, {X: 0, Y: 1}},
		"crossing at the ray's end (t = 1)":         {{X: 10, Y: -1}, {X: 10, Y: 1}},
		"crossing at the edge's start (u = 0)":      {{X: 5, Y: 0}, {X: 5, Y: 1}},
		"crossing at the edge's end (u = 1)":        {{X: 5, Y: -1}, {X: 5, Y: 0}},
		"parallel edge":                             {{X: 0, Y: 1}, {X: 10, Y: 1}},
		"collinear edge":                            {{X: 2, Y: 0}, {X: 8, Y: 0}},
		"cross product exactly at the cut":          {{X: 5, Y: -1}, {X: 5, Y: -1 + 1e-13}},
		"cross product just above the cut":          {{X: 5, Y: -1}, {X: 5, Y: -1 + 1.0000001e-13}},
		"degenerate edge":                           {{X: 5, Y: 0}, {X: 5, Y: 0}},
		"two crossings, nearer one second":          {{X: 8, Y: -1}, {X: 8, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: -1}},
		"two crossings at the same t":               {{X: 4, Y: -1}, {X: 4, Y: 0}, {X: 4, Y: 1}},
		"crossing just past the ray's end":          {{X: math.Nextafter(10, 11), Y: -1}, {X: math.Nextafter(10, 11), Y: 1}},
		"crossing just before the ray's start":      {{X: math.Nextafter(0, -1), Y: -1}, {X: math.Nextafter(0, -1), Y: 1}},
		"vertex on the ray, touching from one side": {{X: 4, Y: 1}, {X: 5, Y: 0}, {X: 6, Y: 1}},
	}

	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertSameLineStringCrossing(t, line, origin, east)
			assertSameLineStringCrossing(t, line, east, origin)
		})
	}
}

// TestLineStringIntersectsSegmentMatchesReferenceOnCityRays throws rays at
// building-sized rings placed at UTM magnitudes, where the cross products
// carry the cancellation a real scene does. Half the rays end on a vertex, so
// the t and u bounds are hit far more often than at random.
func TestLineStringIntersectsSegmentMatchesReferenceOnCityRays(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(19, 2026))

	const baseX, baseY = 550_000.0, 5_800_000.0

	for range 200_000 {
		cx := baseX + rng.Float64()*500
		cy := baseY + rng.Float64()*500
		n := 3 + rng.IntN(10)
		line := make([]Point2D, 0, n+1)

		for k := range n {
			a := 2 * math.Pi * float64(k) / float64(n)
			r := 5 + rng.Float64()*15
			line = append(line, Point2D{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)})
		}

		line = append(line, line[0])

		p1 := Point2D{X: baseX + rng.Float64()*500, Y: baseY + rng.Float64()*500}
		p2 := Point2D{X: baseX + rng.Float64()*500, Y: baseY + rng.Float64()*500}

		if rng.IntN(2) == 0 {
			p2 = line[rng.IntN(n)]
		}

		assertSameLineStringCrossing(t, line, p1, p2)
	}
}

func FuzzLineStringIntersectsSegmentMatchesReference(f *testing.F) {
	f.Add(0.0, 0.0, 10.0, 0.0, 5.0, -1.0, 5.0, 1.0, 6.0, 2.0)
	f.Add(0.0, 0.0, 10.0, 0.0, 0.0, -1.0, 0.0, 1.0, 10.0, 1.0)
	f.Add(550_000.0, 5_800_000.0, 550_100.0, 5_800_050.0, 550_050.0, 5_800_000.0, 550_050.0, 5_800_100.0, 550_060.0, 5_800_000.0)
	f.Add(0.0, 0.0, 1e-160, 1e-160, 0.0, 1e-160, 1e-160, 0.0, 0.0, 0.0)
	f.Add(0.0, 0.0, 1e300, 1e300, 0.0, 1e300, 1e300, 0.0, 0.0, 0.0)

	f.Fuzz(func(t *testing.T, p1x, p1y, p2x, p2y, ax, ay, bx, by, cx, cy float64) {
		line := []Point2D{{X: ax, Y: ay}, {X: bx, Y: by}, {X: cx, Y: cy}}
		assertSameLineStringCrossing(t, line, Point2D{X: p1x, Y: p1y}, Point2D{X: p2x, Y: p2y})
	})
}

// TestAppendRayEdgeCrossingsMatchesAppendRayCrossings holds the RayEdges path
// to the polyline path over city-like obstacle sets: same crossings, same
// bits, same order. The sets include obstacles with no edge at all, and rays
// that end on a vertex, so ties and grazes are exercised.
func TestAppendRayEdgeCrossingsMatchesAppendRayCrossings(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(3, 2026))
	polyline := func(line []Point2D) []Point2D { return line }

	const baseX, baseY = 550_000.0, 5_800_000.0

	for range 2_000 {
		obstacles := make([][]Point2D, 1+rng.IntN(40))

		for i := range obstacles {
			cx := baseX + rng.Float64()*300
			cy := baseY + rng.Float64()*300
			n := rng.IntN(12)
			line := make([]Point2D, 0, n+1)

			for k := range n {
				a := 2 * math.Pi * float64(k) / float64(max(n, 1))
				r := 5 + rng.Float64()*15
				line = append(line, Point2D{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)})
			}

			if n > 2 && rng.IntN(2) == 0 {
				line = append(line, line[0])
			}

			obstacles[i] = line
		}

		edges := NewRayEdges(obstacles, polyline)
		if edges.Len() != len(obstacles) {
			t.Fatalf("RayEdges.Len() = %d, want %d", edges.Len(), len(obstacles))
		}

		var candidates []int

		for i := range obstacles {
			if rng.IntN(4) != 0 {
				candidates = append(candidates, i)
			}
		}

		for range 20 {
			source := Point2D{X: baseX + rng.Float64()*300, Y: baseY + rng.Float64()*300}
			receiver := Point2D{X: baseX + rng.Float64()*300, Y: baseY + rng.Float64()*300}

			if line := obstacles[rng.IntN(len(obstacles))]; len(line) > 0 && rng.IntN(3) == 0 {
				receiver = line[rng.IntN(len(line))]
			}

			for _, tolerance := range []float64{0, 1e-6} {
				want := AppendRayCrossings(nil, source, receiver, obstacles, candidates, polyline, tolerance)
				got := AppendRayEdgeCrossings(nil, source, receiver, edges, candidates, tolerance)

				if len(got) != len(want) {
					t.Fatalf("ray %v→%v: %d crossings, want %d", source, receiver, len(got), len(want))
				}

				for k := range want {
					g, w := got[k], want[k]
					if g.ObstacleIndex != w.ObstacleIndex || g.SegmentIndex != w.SegmentIndex ||
						math.Float64bits(g.Point.X) != math.Float64bits(w.Point.X) ||
						math.Float64bits(g.Point.Y) != math.Float64bits(w.Point.Y) ||
						math.Float64bits(g.DistFromSource) != math.Float64bits(w.DistFromSource) {
						t.Fatalf("ray %v→%v, crossing %d: got %+v, want %+v", source, receiver, k, g, w)
					}
				}
			}
		}
	}
}
