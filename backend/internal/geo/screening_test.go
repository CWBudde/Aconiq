package geo_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// wall is a stand-in for a standards module's own barrier type, so the tests
// exercise RayCrossings the way a caller does: through an accessor, recovering
// its own value by index.
type wall struct {
	name     string
	geometry []geo.Point2D
	heightM  float64
}

func wallGeometry(w wall) []geo.Point2D { return w.geometry }

func TestUpperConvexHull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		given []geo.HullPoint
		want  []int // Index of each surviving point, in order
	}{
		{
			name:  "empty stays empty",
			given: nil,
			want:  nil,
		},
		{
			name:  "two points are their own hull",
			given: []geo.HullPoint{{Dist: 0, Height: 2, Index: -1}, {Dist: 10, Height: 2, Index: -1}},
			want:  []int{-1, -1},
		},
		{
			name: "a point below the chord is dropped",
			given: []geo.HullPoint{
				{Dist: 0, Height: 10, Index: -1},
				{Dist: 5, Height: 1, Index: 0},
				{Dist: 10, Height: 10, Index: -1},
			},
			want: []int{-1, -1},
		},
		{
			name: "a point above the chord is kept",
			given: []geo.HullPoint{
				{Dist: 0, Height: 1, Index: -1},
				{Dist: 5, Height: 10, Index: 0},
				{Dist: 10, Height: 1, Index: -1},
			},
			want: []int{-1, 0, -1},
		},
		{
			name: "a point exactly on the chord is dropped",
			given: []geo.HullPoint{
				{Dist: 0, Height: 0, Index: -1},
				{Dist: 5, Height: 5, Index: 0},
				{Dist: 10, Height: 10, Index: -1},
			},
			want: []int{-1, -1},
		},
		{
			name: "the inner of two peaks is hidden by the outer pair",
			given: []geo.HullPoint{
				{Dist: 0, Height: 0, Index: -1},
				{Dist: 2, Height: 10, Index: 0},
				{Dist: 5, Height: 6, Index: 1},
				{Dist: 8, Height: 10, Index: 2},
				{Dist: 10, Height: 0, Index: -1},
			},
			want: []int{-1, 0, 2, -1},
		},
		{
			name: "three peaks on a convex arc all survive",
			given: []geo.HullPoint{
				{Dist: 0, Height: 0, Index: -1},
				{Dist: 2, Height: 6, Index: 0},
				{Dist: 5, Height: 9, Index: 1},
				{Dist: 8, Height: 6, Index: 2},
				{Dist: 10, Height: 0, Index: -1},
			},
			want: []int{-1, 0, 1, 2, -1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			hull := geo.UpperConvexHull(test.given)

			got := make([]int, 0, len(hull))
			for _, point := range hull {
				got = append(got, point.Index)
			}

			if len(got) != len(test.want) {
				t.Fatalf("hull has %d points, want %d (%v vs %v)", len(got), len(test.want), got, test.want)
			}

			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("hull index[%d] = %d, want %d (%v vs %v)", i, got[i], test.want[i], got, test.want)
				}
			}
		})
	}
}

func TestObstructsLineOfSight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                           string
		distFromSource, topHeightM     float64
		sourceHeightM, receiverHeightM float64
		totalDistM                     float64
		want                           bool
	}{
		{"above a level sight line", 50, 6, 4, 4, 100, true},
		{"below a level sight line", 50, 3, 4, 4, 100, false},
		{"exactly on the sight line is not obstruction", 50, 4, 4, 4, 100, false},
		// The sight line climbs from 2 m to 18 m, so it passes 6 m at d = 25
		// and 14 m at d = 75: the same 7 m obstacle screens near the source
		// and not further along.
		{"above a sloping sight line near the source", 25, 7, 2, 18, 100, true},
		{"below the same sloping sight line further along", 75, 7, 2, 18, 100, false},
		{"a degenerate ray obstructs nothing", 0, 100, 4, 4, 0, false},
		{"a negative total distance obstructs nothing", 50, 100, 4, 4, -1, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := geo.ObstructsLineOfSight(
				test.distFromSource, test.topHeightM,
				test.sourceHeightM, test.receiverHeightM, test.totalDistM,
			)
			if got != test.want {
				t.Fatalf("ObstructsLineOfSight = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSelectDiffractionEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                           string
		sourceHeightM, receiverHeightM float64
		totalDistM                     float64
		candidates                     []geo.ScreeningPoint
		want                           []int
	}{
		{
			name:            "no candidates selects nothing",
			sourceHeightM:   2,
			receiverHeightM: 4,
			totalDistM:      100,
			candidates:      nil,
			want:            nil,
		},
		{
			name:            "a single obstructing candidate is the edge",
			sourceHeightM:   2,
			receiverHeightM: 4,
			totalDistM:      100,
			candidates:      []geo.ScreeningPoint{{DistFromSource: 50, TopHeightM: 8}},
			want:            []int{0},
		},
		{
			name:            "two barriers both on the band survive",
			sourceHeightM:   1,
			receiverHeightM: 1,
			totalDistM:      100,
			candidates: []geo.ScreeningPoint{
				{DistFromSource: 30, TopHeightM: 8},
				{DistFromSource: 70, TopHeightM: 8},
			},
			want: []int{0, 1},
		},
		{
			name:            "a barrier the band spans over is dropped",
			sourceHeightM:   1,
			receiverHeightM: 1,
			totalDistM:      100,
			candidates: []geo.ScreeningPoint{
				{DistFromSource: 20, TopHeightM: 12},
				{DistFromSource: 50, TopHeightM: 6},
				{DistFromSource: 80, TopHeightM: 12},
			},
			want: []int{0, 2},
		},
		{
			name:            "three edges on a convex arc all survive",
			sourceHeightM:   0,
			receiverHeightM: 0,
			totalDistM:      100,
			candidates: []geo.ScreeningPoint{
				{DistFromSource: 20, TopHeightM: 6},
				{DistFromSource: 50, TopHeightM: 9},
				{DistFromSource: 80, TopHeightM: 6},
			},
			want: []int{0, 1, 2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := geo.SelectDiffractionEdges(
				test.sourceHeightM, test.receiverHeightM, test.totalDistM, test.candidates,
			)

			if len(got) != len(test.want) {
				t.Fatalf("selected %v, want %v", got, test.want)
			}

			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("selected %v, want %v", got, test.want)
				}
			}
		})
	}
}

// TestSelectDiffractionEdgesMatchesTheSingleCandidateShortcut pins the
// equivalence the lift relies on: RLS-19 used to return its one obstructing
// crossing without building a hull at all. It is the same answer, because an
// obstructing candidate is by definition above the chord the hull rests on.
func TestSelectDiffractionEdgesMatchesTheSingleCandidateShortcut(t *testing.T) {
	t.Parallel()

	const (
		sourceHeightM   = 1.5
		receiverHeightM = 4.0
		totalDistM      = 120.0
	)

	for _, dist := range []float64{1, 10, 60, 110, 119} {
		for _, height := range []float64{5, 9, 40} {
			if !geo.ObstructsLineOfSight(dist, height, sourceHeightM, receiverHeightM, totalDistM) {
				continue
			}

			edges := geo.SelectDiffractionEdges(
				sourceHeightM, receiverHeightM, totalDistM,
				[]geo.ScreeningPoint{{DistFromSource: dist, TopHeightM: height}},
			)

			if len(edges) != 1 || edges[0] != 0 {
				t.Fatalf("one obstructing candidate at d=%g h=%g selected %v, want [0]", dist, height, edges)
			}
		}
	}
}

func TestRayCrossings(t *testing.T) {
	t.Parallel()

	// Three parallel north-south walls at x = 20, 50 and 80, crossed by a ray
	// running east along y = 0.
	walls := []wall{
		{name: "far", geometry: []geo.Point2D{{X: 80, Y: -10}, {X: 80, Y: 10}}, heightM: 5},
		{name: "near", geometry: []geo.Point2D{{X: 20, Y: -10}, {X: 20, Y: 10}}, heightM: 5},
		{name: "middle", geometry: []geo.Point2D{{X: 50, Y: -10}, {X: 50, Y: 10}}, heightM: 5},
		{name: "aside", geometry: []geo.Point2D{{X: 50, Y: 30}, {X: 50, Y: 40}}, heightM: 5},
	}

	crossings := geo.RayCrossings(
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 100, Y: 0},
		walls, wallGeometry, 1e-6,
	)

	wantOrder := []string{"near", "middle", "far"}
	if len(crossings) != len(wantOrder) {
		t.Fatalf("got %d crossings, want %d", len(crossings), len(wantOrder))
	}

	for i, want := range wantOrder {
		if got := walls[crossings[i].ObstacleIndex].name; got != want {
			t.Fatalf("crossing[%d] is %q, want %q", i, got, want)
		}
	}

	wantDistances := []float64{20, 50, 80}
	for i, want := range wantDistances {
		if math.Abs(crossings[i].DistFromSource-want) > 1e-9 {
			t.Fatalf("crossing[%d] distance %.9g, want %.9g", i, crossings[i].DistFromSource, want)
		}
	}
}

func TestRayCrossingsCountsAPolylineOnce(t *testing.T) {
	t.Parallel()

	// A polyline that doubles back crosses the ray twice. It is one obstacle,
	// so it must produce one crossing — the one nearest the source.
	zigzag := []wall{{
		name: "zigzag",
		geometry: []geo.Point2D{
			{X: 30, Y: -10}, {X: 30, Y: 10}, {X: 60, Y: 10}, {X: 60, Y: -10},
		},
		heightM: 5,
	}}

	crossings := geo.RayCrossings(
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 100, Y: 0},
		zigzag, wallGeometry, 1e-6,
	)

	if len(crossings) != 1 {
		t.Fatalf("got %d crossings for one doubling-back polyline, want 1", len(crossings))
	}

	if math.Abs(crossings[0].DistFromSource-30) > 1e-9 {
		t.Fatalf("kept the crossing at %.9g m, want the nearest at 30 m", crossings[0].DistFromSource)
	}
}

func TestRayCrossingsEndpointTolerance(t *testing.T) {
	t.Parallel()

	// A wall standing exactly on the source and one exactly on the receiver.
	touching := []wall{
		{name: "on source", geometry: []geo.Point2D{{X: 0, Y: -10}, {X: 0, Y: 10}}, heightM: 5},
		{name: "on receiver", geometry: []geo.Point2D{{X: 100, Y: -10}, {X: 100, Y: 10}}, heightM: 5},
		{name: "between", geometry: []geo.Point2D{{X: 50, Y: -10}, {X: 50, Y: 10}}, heightM: 5},
	}

	source, receiver := geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 100, Y: 0}

	withTolerance := geo.RayCrossings(source, receiver, touching, wallGeometry, 1e-6)
	if len(withTolerance) != 1 || touching[withTolerance[0].ObstacleIndex].name != "between" {
		t.Fatalf("a tolerance must drop both endpoint grazes, got %d crossings", len(withTolerance))
	}

	withoutTolerance := geo.RayCrossings(source, receiver, touching, wallGeometry, 0)
	if len(withoutTolerance) != 3 {
		t.Fatalf("tolerance 0 must keep every crossing, got %d", len(withoutTolerance))
	}
}

func TestRayCrossingsIsStableAtEqualDistance(t *testing.T) {
	t.Parallel()

	// Two walls crossed at the same distance keep their input order, so the
	// result does not depend on how the caller happened to build the slice.
	coincident := []wall{
		{name: "first", geometry: []geo.Point2D{{X: 50, Y: -10}, {X: 50, Y: 10}}, heightM: 5},
		{name: "second", geometry: []geo.Point2D{{X: 50, Y: -20}, {X: 50, Y: 20}}, heightM: 9},
	}

	crossings := geo.RayCrossings(
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 100, Y: 0},
		coincident, wallGeometry, 1e-6,
	)

	if len(crossings) != 2 {
		t.Fatalf("got %d crossings, want 2", len(crossings))
	}

	if crossings[0].ObstacleIndex != 0 || crossings[1].ObstacleIndex != 1 {
		t.Fatalf("equal distances were reordered: %d then %d", crossings[0].ObstacleIndex, crossings[1].ObstacleIndex)
	}
}

func TestRayCrossingsNoObstacles(t *testing.T) {
	t.Parallel()

	crossings := geo.RayCrossings(
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 100, Y: 0},
		[]wall(nil), wallGeometry, 1e-6,
	)
	if crossings != nil {
		t.Fatalf("an empty obstacle slice must produce no crossings, got %v", crossings)
	}
}
