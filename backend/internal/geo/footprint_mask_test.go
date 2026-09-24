package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

func courtyardBlock() [][]Point2D {
	return [][]Point2D{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
		{{3, 3}, {7, 3}, {7, 7}, {3, 7}, {3, 3}},
	}
}

func TestPointInPolygonInterior(t *testing.T) {
	t.Parallel()

	rings := courtyardBlock()

	cases := []struct {
		name string
		p    Point2D
		want bool
	}{
		{"inside the wing", Point2D{X: 1, Y: 1}, true},
		{"outside", Point2D{X: 11, Y: 5}, false},
		{"in the courtyard", Point2D{X: 5, Y: 5}, false},
		{"on the facade", Point2D{X: 0, Y: 5}, false},
		{"on a corner", Point2D{X: 10, Y: 10}, false},
		{"on the courtyard edge", Point2D{X: 3, Y: 5}, false},
		{"on a courtyard corner", Point2D{X: 7, Y: 7}, false},
	}

	for _, tc := range cases {
		if got := PointInPolygonInterior(tc.p, rings); got != tc.want {
			t.Errorf("%s: PointInPolygonInterior(%v) = %v, want %v", tc.name, tc.p, got, tc.want)
		}
	}

	// The boundary is the only place the two tests disagree.
	if !PointInPolygon(Point2D{X: 0, Y: 5}, rings) {
		t.Error("PointInPolygon must keep counting an edge as inside")
	}

	if PointInPolygonInterior(Point2D{X: 1, Y: 1}, [][]Point2D{{{0, 0}, {10, 0}, {10, 10}}}) {
		t.Error("an unclosed exterior must contain nothing")
	}
}

func TestMaskPointsInFootprints(t *testing.T) {
	t.Parallel()

	shifted := func(rings [][]Point2D, dx float64) [][]Point2D {
		out := make([][]Point2D, len(rings))
		for i, ring := range rings {
			out[i] = make([]Point2D, len(ring))
			for j, p := range ring {
				out[i][j] = Point2D{X: p.X + dx, Y: p.Y}
			}
		}

		return out
	}

	footprints := [][][]Point2D{
		courtyardBlock(),
		shifted(courtyardBlock(), 20), // a second part of the same MultiPolygon, say
		{{{0, 0}, {1, 0}}},            // degenerate: ignored
		{},                            // empty: ignored
	}

	points := []Point2D{
		{X: 1, Y: 1},             // wing of the first block
		{X: 5, Y: 5},             // courtyard
		{X: 0, Y: 5},             // facade
		{X: 21, Y: 9},            // wing of the second block
		{X: 15, Y: 5},            // between the blocks
		{X: math.NaN(), Y: 1},    // not a point
		{X: 25, Y: 5},            // second courtyard
		{X: 1e9, Y: -1e9},        // far outside
		{X: 29.999999, Y: 5.123}, // just inside the second block's east facade
	}
	want := []bool{true, false, false, true, false, false, false, false, true}

	got := MaskPointsInFootprints(points, footprints)
	if len(got) != len(points) {
		t.Fatalf("got %d answers for %d points", len(got), len(points))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d %v: masked = %v, want %v", i, points[i], got[i], want[i])
		}
	}

	if got := MaskPointsInFootprints(points, nil); len(got) != len(points) {
		t.Fatalf("no footprints: got %d answers, want %d", len(got), len(points))
	}
}

// The index is a prefilter only: whatever it prunes, the answer must be the
// one an exhaustive loop over every footprint gives.
func TestMaskPointsInFootprintsMatchesBruteForce(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 11))

	footprints := make([][][]Point2D, 0, 300)

	for range 300 {
		x, y := rng.Float64()*1000, rng.Float64()*1000
		w, h := 2+rng.Float64()*30, 2+rng.Float64()*30
		footprints = append(footprints, [][]Point2D{{
			{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + h}, {X: x, Y: y + h}, {X: x, Y: y},
		}})
	}

	// A regular grid, as the run pipeline sends it.
	points := make([]Point2D, 0, 101*101)

	for row := range 101 {
		for col := range 101 {
			points = append(points, Point2D{X: float64(col) * 10, Y: float64(row) * 10})
		}
	}

	got := MaskPointsInFootprints(points, footprints)

	masked := 0

	for i, p := range points {
		want := false

		for _, rings := range footprints {
			if PointInPolygonInterior(p, rings) {
				want = true

				break
			}
		}

		if got[i] != want {
			t.Fatalf("point %v: masked = %v, brute force says %v", p, got[i], want)
		}

		if want {
			masked++
		}
	}

	if masked == 0 {
		t.Fatal("the corpus masked nothing, so it tested nothing")
	}
}
