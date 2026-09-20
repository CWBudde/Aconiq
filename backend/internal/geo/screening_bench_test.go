package geo

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// BenchmarkSortRayCrossings is what picks rayCrossingInsertionMax. It runs the
// insertion sort and slices.SortStableFunc over the same inputs, at the ray
// crossing counts a propagation walk actually produces (a handful) and well
// past them, and at three tie densities, because a stable sort's cost depends
// on how many equal keys it has to keep in order.
//
// Each iteration copies the template back over the working slice, since both
// sorts work in place. That copy is timed, and identically in both arms, so
// the comparison stands; the copy-only arm is there to be subtracted when the
// absolute number matters.
func BenchmarkSortRayCrossings(b *testing.B) {
	ties := []struct {
		name    string
		buckets func(length int) int
	}{
		// Every crossing at the same distance: the stable sort's worst case
		// for tie handling, and the insertion sort's best, since nothing moves.
		{name: "ties=all", buckets: func(int) int { return 1 }},
		// Roughly two crossings per distance.
		{name: "ties=half", buckets: func(length int) int { return max(1, length/2) }},
		// Ties rare enough to be incidental, which is the realistic case.
		{name: "ties=none", buckets: func(length int) int { return max(1, 8*length) }},
	}

	impls := []struct {
		name string
		sort func([]RayCrossing)
	}{
		{name: "impl=insertion", sort: insertionSortRayCrossings},
		{name: "impl=stable", sort: sortStableRayCrossings},
		{name: "impl=dispatch", sort: sortRayCrossings},
		{name: "impl=copy-only", sort: func([]RayCrossing) {}},
	}

	for _, length := range []int{2, 4, 8, 16, 32, 64, 256} {
		for _, tie := range ties {
			rng := rand.New(rand.NewPCG(uint64(length), 4242))
			template := randomCrossings(rng, length, tie.buckets(length))
			working := make([]RayCrossing, length)

			for _, impl := range impls {
				b.Run(fmt.Sprintf("len=%d/%s/%s", length, tie.name, impl.name), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()

					for range b.N {
						copy(working, template)
						impl.sort(working)
					}

					// Keep the sorted slice observable so nothing above is
					// eliminated as dead.
					if !slices.IsSortedFunc(working, func(x, y RayCrossing) int {
						switch {
						case x.DistFromSource < y.DistFromSource:
							return -1
						case x.DistFromSource > y.DistFromSource:
							return 1
						default:
							return 0
						}
					}) && impl.name != "impl=copy-only" {
						b.Fatal("benchmark left an unsorted slice")
					}
				})
			}
		}
	}
}
