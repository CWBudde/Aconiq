package geo

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// benchGridDomain is the side of the square the benchmark scatters its boxes
// over. It is held fixed while the box count grows, so the grid gets denser
// the way a model does when a district is added to it, rather than sparser.
const benchGridDomain = 10000.0

// benchBoxes scatters count small boxes over benchGridDomain. They are small
// relative to a cell on purpose: an obstacle in a propagation model — a
// building footprint, a barrier panel — is tiny against the scene, which is
// what makes the cell walk worth having at all.
func benchBoxes(rng *rand.Rand, count int) []BBox {
	boxes := make([]BBox, 0, count)

	for range count {
		x := rng.Float64() * benchGridDomain
		y := rng.Float64() * benchGridDomain
		w := 2 + rng.Float64()*18
		h := 2 + rng.Float64()*18

		boxes = append(boxes, BBox{MinX: x, MinY: y, MaxX: x + w, MaxY: y + h})
	}

	return boxes
}

// benchQueryExtent bisects for the query side length that makes a query answer
// with about target candidates, so that "candidates=10" means the same thing
// at 100 boxes and at 100000 and the numbers across box counts can be read as
// one table.
func benchQueryExtent(boxes []BBox, rng *rand.Rand, target int) float64 {
	measure := func(extent float64) float64 {
		var total int

		for range 40 {
			x := rng.Float64() * benchGridDomain
			y := rng.Float64() * benchGridDomain
			total += len(bruteForceQuery(boxes, BBox{MinX: x, MinY: y, MaxX: x + extent, MaxY: y + extent}))
		}

		return float64(total) / 40
	}

	low, high := 0.0, benchGridDomain

	for range 40 {
		mid := (low + high) / 2
		if measure(mid) < float64(target) {
			low = mid
		} else {
			high = mid
		}
	}

	return (low + high) / 2
}

// benchGridCase is one (box count, candidate count) point of the matrix, with
// the queries pre-generated so that only the query itself is timed.
type benchGridCase struct {
	name     string
	grid     *BBoxGrid
	boxes    []BBox
	queries  []BBox
	segments [][2]Point2D
	shadows  []SegmentShadow
}

func benchGridCases(b *testing.B) []benchGridCase {
	b.Helper()

	const queryCount = 256

	cases := make([]benchGridCase, 0, 12)

	for _, boxCount := range []int{100, 1_000, 10_000, 100_000} {
		rng := rand.New(rand.NewPCG(uint64(boxCount), 7))
		boxes := benchBoxes(rng, boxCount)

		grid := NewBBoxGrid(boxes)
		if grid == nil {
			b.Fatalf("no grid over %d boxes", boxCount)
		}

		for _, candidates := range []int{1, 10, 100} {
			extent := benchQueryExtent(boxes, rng, candidates)

			queries := make([]BBox, 0, queryCount)
			segments := make([][2]Point2D, 0, queryCount)
			shadows := make([]SegmentShadow, 0, queryCount)

			for range queryCount {
				x := rng.Float64() * benchGridDomain
				y := rng.Float64() * benchGridDomain
				queries = append(queries, BBox{MinX: x, MinY: y, MaxX: x + extent, MaxY: y + extent})

				// A segment across the same window, and a shadow cast from an
				// apex one window away — the shapes a propagation walk asks
				// for, at the scale the box query above was sized to.
				p := Point2D{X: x, Y: y}
				q := Point2D{X: x + extent, Y: y + extent}
				segments = append(segments, [2]Point2D{p, q})
				shadows = append(shadows, NewSegmentShadow(
					Point2D{X: x - extent, Y: y - extent}, p, q, 1e-3,
				))
			}

			cases = append(cases, benchGridCase{
				name:     fmt.Sprintf("boxes=%d/candidates=%d", boxCount, candidates),
				grid:     grid,
				boxes:    boxes,
				queries:  queries,
				segments: segments,
				shadows:  shadows,
			})
		}
	}

	return cases
}

func BenchmarkBBoxGridQuery(b *testing.B) {
	for _, tc := range benchGridCases(b) {
		b.Run(tc.name, func(b *testing.B) {
			cursor := tc.grid.NewCursor()
			buffer := make([]int, 0, 1024)

			b.ReportAllocs()
			b.ResetTimer()

			for i := range b.N {
				buffer = cursor.Query(tc.queries[i%len(tc.queries)], buffer[:0])
			}

			runtimeSink = len(buffer)
		})
	}
}

func BenchmarkBBoxGridQuerySegment(b *testing.B) {
	for _, tc := range benchGridCases(b) {
		b.Run(tc.name, func(b *testing.B) {
			cursor := tc.grid.NewCursor()
			buffer := make([]int, 0, 1024)

			b.ReportAllocs()
			b.ResetTimer()

			for i := range b.N {
				segment := tc.segments[i%len(tc.segments)]
				buffer = cursor.QuerySegment(segment[0], segment[1], 1e-3, buffer[:0])
			}

			runtimeSink = len(buffer)
		})
	}
}

func BenchmarkBBoxGridQueryShadow(b *testing.B) {
	for _, tc := range benchGridCases(b) {
		b.Run(tc.name, func(b *testing.B) {
			cursor := tc.grid.NewCursor()
			buffer := make([]int, 0, 1024)

			b.ReportAllocs()
			b.ResetTimer()

			for i := range b.N {
				shadow := tc.shadows[i%len(tc.shadows)]
				buffer = cursor.QueryShadow(&shadow, 1e-3, buffer[:0])
			}

			runtimeSink = len(buffer)
		})
	}
}

// runtimeSink keeps a benchmark's last answer reachable so the compiler cannot
// drop the query that produced it.
var runtimeSink int
