package geo

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// sortStableRayCrossings is the implementation sortRayCrossings had before the
// insertion path went in, kept verbatim as the oracle every test below holds
// the new one against. Determinism policy allows no output tolerance, so the
// bar is the same permutation, element for element, and not merely a correctly
// sorted one.
func sortStableRayCrossings(crossings []RayCrossing) {
	slices.SortStableFunc(crossings, func(a, b RayCrossing) int {
		switch {
		case a.DistFromSource < b.DistFromSource:
			return -1
		case a.DistFromSource > b.DistFromSource:
			return 1
		default:
			return 0
		}
	})
}

// sameCrossings compares two sorted results element for element. It cannot be
// slices.Equal: RayCrossing is comparable, but a NaN key makes == false against
// itself, and the NaN cases below are exactly the ones worth comparing. Keys are
// held against each other by bit pattern instead, which is stricter than == and
// is what "identical permutation" means when the payload has been moved rather
// than recomputed.
func sameCrossings(a, b []RayCrossing) bool {
	return slices.EqualFunc(a, b, func(x, y RayCrossing) bool {
		return x.Point == y.Point &&
			math.Float64bits(x.DistFromSource) == math.Float64bits(y.DistFromSource) &&
			x.ObstacleIndex == y.ObstacleIndex &&
			x.SegmentIndex == y.SegmentIndex
	})
}

// randomCrossings builds a slice whose ObstacleIndex records each element's
// input position, so that a comparison of the two sorts sees the permutation
// and not just the keys. tieBuckets says how many distinct distances the keys
// are drawn from: 1 makes every element a tie, len(crossings) makes ties rare.
func randomCrossings(rng *rand.Rand, length, tieBuckets int) []RayCrossing {
	crossings := make([]RayCrossing, 0, length)

	for i := range length {
		bucket := rng.IntN(tieBuckets)

		crossings = append(crossings, RayCrossing{
			Point:          Point2D{X: float64(i), Y: float64(bucket)},
			DistFromSource: float64(bucket) * 0.25,
			ObstacleIndex:  i,
			SegmentIndex:   i % 3,
		})
	}

	return crossings
}

// TestSortRayCrossingsMatchesStableSort is the core equivalence: over lengths
// on both sides of rayCrossingInsertionMax and tie densities from "all keys
// equal" to "no ties", the two sorts must produce the identical slice.
func TestSortRayCrossingsMatchesStableSort(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(11, 13))

	for _, length := range []int{0, 1, 2, 3, 4, 8, 16, 32, 63, 64, 65, 128, 256} {
		for _, buckets := range []int{1, 2, 3, max(1, length/2), max(1, length), max(1, 4*length)} {
			for range 40 {
				want := randomCrossings(rng, length, buckets)
				got := slices.Clone(want)

				sortStableRayCrossings(want)
				sortRayCrossings(got)

				if !sameCrossings(got, want) {
					t.Fatalf("length %d, %d tie buckets: got %v, want %v", length, buckets, got, want)
				}
			}
		}
	}
}

// TestInsertionSortRayCrossingsIsStable pins the insertion loop itself, not
// just sortRayCrossings' dispatch: the guard has to stay strictly `>`, and a
// `>=` slipped in later would reverse every run of equal keys here.
func TestInsertionSortRayCrossingsIsStable(t *testing.T) {
	t.Parallel()

	crossings := []RayCrossing{
		{Point: Point2D{}, DistFromSource: 5, ObstacleIndex: 0, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 1, ObstacleIndex: 1, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 5, ObstacleIndex: 2, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 1, ObstacleIndex: 3, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 5, ObstacleIndex: 4, SegmentIndex: 0},
	}

	insertionSortRayCrossings(crossings)

	order := make([]int, 0, len(crossings))
	for _, crossing := range crossings {
		order = append(order, crossing.ObstacleIndex)
	}

	if want := []int{1, 3, 0, 2, 4}; !slices.Equal(order, want) {
		t.Fatalf("ties did not keep input order: got %v, want %v", order, want)
	}
}

// TestSortRayCrossingsSendsNonFiniteKeysToTheStableSort documents the one case
// the insertion argument does not cover. A NaN key is not a total preorder
// under `>`, so such a slice must take the path it took before the change —
// which is trivially identical to itself, and is the point of the guard.
func TestSortRayCrossingsSendsNonFiniteKeysToTheStableSort(t *testing.T) {
	t.Parallel()

	base := []RayCrossing{
		{Point: Point2D{}, DistFromSource: 3, ObstacleIndex: 0, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: math.NaN(), ObstacleIndex: 1, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 1, ObstacleIndex: 2, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: math.Inf(1), ObstacleIndex: 3, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 2, ObstacleIndex: 4, SegmentIndex: 0},
	}

	if rayCrossingKeysOrdered(base) {
		t.Fatal("a NaN key must be reported as unorderable")
	}

	got, want := slices.Clone(base), slices.Clone(base)

	sortRayCrossings(got)
	sortStableRayCrossings(want)

	if !sameCrossings(got, want) {
		t.Fatalf("a NaN key diverged from the stable sort: got %v, want %v", got, want)
	}

	// An infinity on its own is perfectly orderable and must not be diverted.
	finite := []RayCrossing{
		{Point: Point2D{}, DistFromSource: math.Inf(1), ObstacleIndex: 0, SegmentIndex: 0},
		{Point: Point2D{}, DistFromSource: 1, ObstacleIndex: 1, SegmentIndex: 0},
	}
	if !rayCrossingKeysOrdered(finite) {
		t.Fatal("an infinite key is still totally ordered")
	}
}

// FuzzSortRayCrossingsMatchesStableSort drives the equivalence from fuzzer
// bytes rather than a seeded generator, so a key pattern the generator never
// emits — a signed zero next to a zero, a denormal, a NaN among finites —
// still has to agree.
func FuzzSortRayCrossingsMatchesStableSort(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{9, 9, 9, 9})
	f.Add(make([]byte, 200))

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 512 {
			raw = raw[:512]
		}

		crossings := make([]RayCrossing, 0, len(raw))

		for i, b := range raw {
			key := float64(b%17) * 0.5

			switch b {
			case 255:
				key = math.NaN()
			case 254:
				key = math.Inf(1)
			case 253:
				key = math.Copysign(0, -1)
			}

			crossings = append(crossings, RayCrossing{
				Point:          Point2D{X: float64(i), Y: 0},
				DistFromSource: key,
				ObstacleIndex:  i,
				SegmentIndex:   0,
			})
		}

		got := slices.Clone(crossings)
		want := slices.Clone(crossings)

		sortRayCrossings(got)
		sortStableRayCrossings(want)

		if !sameCrossings(got, want) {
			t.Fatalf("diverged from the stable sort: got %v, want %v", got, want)
		}
	})
}

// FuzzRayCrossingDistanceIsFinite backs the reachability claim in
// sortRayCrossings' doc comment: over coordinates of the magnitude a projected
// CRS produces, the distance a crossing carries is never NaN, so the guarded
// path is insurance and not the common case. It deliberately does not claim
// more than that — the guard exists because "never at these magnitudes" is not
// "never".
func FuzzRayCrossingDistanceIsFinite(f *testing.F) {
	f.Add(0.0, 0.0, 100.0, 0.0, 50.0, -10.0, 50.0, 10.0)
	f.Add(1e6, 5e6, 1e6, 5e6, 1e6, 5e6, 1e6, 5e6)

	f.Fuzz(func(t *testing.T, sx, sy, rx, ry, ax, ay, bx, by float64) {
		for _, v := range []float64{sx, sy, rx, ry, ax, ay, bx, by} {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e9 {
				t.Skip("outside the coordinate range a projected CRS produces")
			}
		}

		source := Point2D{X: sx, Y: sy}
		receiver := Point2D{X: rx, Y: ry}
		wall := [][]Point2D{{{X: ax, Y: ay}, {X: bx, Y: by}}}

		for _, crossing := range RayCrossings(source, receiver, wall, func(w []Point2D) []Point2D { return w }, 0) {
			if math.IsNaN(crossing.DistFromSource) {
				t.Fatalf("NaN distance from source %v, receiver %v, wall %v", source, receiver, wall)
			}
		}
	})
}
