package runestimate

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// DefaultMaxProbeReceivers is how many receivers the probe times at most.
//
// Sixteen, as PLAN.md names it: enough strata that a grid half in a courtyard
// and half on an open road is seen as both, few enough that each one gets a
// real slice of a three-second budget.
const DefaultMaxProbeReceivers = 16

// ProbeReceivers picks at most limit receivers, spread over the receiver set,
// and returns their indices in the order they will be probed.
//
// The extent is divided into strata — a grid of at most limit cells, shaped
// after the extent so a long, thin calculation area is cut along its length —
// and every stratum that holds a receiver contributes the one nearest its own
// receivers' centroid. A receiver set no larger than limit is probed whole.
//
// It is a pure function of the positions: no map iteration, no randomness,
// ties to the lower index. That is what makes two estimates of the same run
// time the same receivers.
func ProbeReceivers(points []geo.Point2D, limit int) []int {
	if limit <= 0 {
		limit = DefaultMaxProbeReceivers
	}

	if len(points) <= limit {
		all := make([]int, len(points))
		for i := range all {
			all[i] = i
		}

		return all
	}

	bounds := extentOf(points)
	cols, rows := strataShape(bounds, limit)

	type stratum struct {
		sumX, sumY float64
		members    []int
	}

	strata := make([]stratum, cols*rows)

	for i, p := range points {
		cell := stratumOf(p, bounds, cols, rows)
		strata[cell].sumX += p.X
		strata[cell].sumY += p.Y
		strata[cell].members = append(strata[cell].members, i)
	}

	picked := make([]int, 0, limit)

	for _, s := range strata {
		if len(s.members) == 0 {
			continue
		}

		centroid := geo.Point2D{X: s.sumX / float64(len(s.members)), Y: s.sumY / float64(len(s.members))}

		best, bestDist := s.members[0], math.Inf(1)

		for _, i := range s.members {
			d := squaredDistance(points[i], centroid)
			if d < bestDist {
				best, bestDist = i, d
			}
		}

		picked = append(picked, best)
	}

	return picked
}

func extentOf(points []geo.Point2D) geo.BBox {
	bounds := geo.BBox{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}

	for _, p := range points {
		bounds.MinX = math.Min(bounds.MinX, p.X)
		bounds.MinY = math.Min(bounds.MinY, p.Y)
		bounds.MaxX = math.Max(bounds.MaxX, p.X)
		bounds.MaxY = math.Max(bounds.MaxY, p.Y)
	}

	return bounds
}

// strataShape divides limit cells between the two axes in proportion to the
// extent, so that the cells come out roughly square.
func strataShape(bounds geo.BBox, limit int) (int, int) {
	width := bounds.MaxX - bounds.MinX
	height := bounds.MaxY - bounds.MinY

	switch {
	case width <= 0 && height <= 0:
		return 1, 1
	case height <= 0:
		return limit, 1
	case width <= 0:
		return 1, limit
	}

	cols := int(math.Round(math.Sqrt(float64(limit) * width / height)))
	cols = min(max(cols, 1), limit)
	rows := max(limit/cols, 1)

	return cols, rows
}

func stratumOf(p geo.Point2D, bounds geo.BBox, cols, rows int) int {
	return cellIndex(p.X, bounds.MinX, bounds.MaxX, cols) + cols*cellIndex(p.Y, bounds.MinY, bounds.MaxY, rows)
}

func cellIndex(v, lo, hi float64, cells int) int {
	if cells <= 1 || hi <= lo {
		return 0
	}

	return min(int(float64(cells)*(v-lo)/(hi-lo)), cells-1)
}

func squaredDistance(a, b geo.Point2D) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y

	return dx*dx + dy*dy
}

// sampleOrder visits the units 0..n-1 of one receiver in bit-reversed order.
//
// Any prefix of that order is spread over the whole range: the first two
// units are the first and the middle one, the first four split it in
// quarters, and so on. A budget that stops the probe after m units has
// therefore timed m units spread along every source of the scene rather than
// the first m Teilstücke of the first road — and the units of one source are
// adjacent in the walk, so "the first m" would have been one road. It visits
// every unit exactly once before it repeats.
type sampleOrder struct {
	n     int
	width int
	next  int
}

func newSampleOrder(n int) *sampleOrder {
	width := 0
	if n > 1 {
		for 1<<width < n {
			width++
		}
	}

	return &sampleOrder{n: n, width: width}
}

// take returns the next unit index.
func (o *sampleOrder) take() int {
	for {
		m := o.next
		o.next++

		// m's low width bits, reversed.
		index := 0
		for range o.width {
			index = index<<1 | m&1
			m >>= 1
		}

		if index < o.n {
			return index
		}
	}
}

// vanDerCorput is the k-th point of the base-2 van der Corput sequence in
// [0, 1): 0, 1/2, 1/4, 3/4, 1/8, … — k's binary digits mirrored behind the
// point, so every prefix of the sequence is spread over the interval.
func vanDerCorput(k int) float64 {
	point, scale := 0.0, 0.5

	for ; k > 0; k >>= 1 {
		if k&1 == 1 {
			point += scale
		}

		scale /= 2
	}

	return point
}

// PlannedReceiver is one probe receiver and the units it visits first.
type PlannedReceiver struct {
	Receiver int
	Units    []int
}

// SamplePlan is the work an estimate times before the budget cuts it short:
// the probe receivers in probe order, and for each the first units it would
// visit (at most perReceiver of them). It exists so a test can hold the
// sample fixed; the probe walks the same sequences.
func SamplePlan(points []geo.Point2D, units func(receiver int) int, limit, perReceiver int) []PlannedReceiver {
	probes := ProbeReceivers(points, limit)
	plan := make([]PlannedReceiver, 0, len(probes))

	for _, r := range probes {
		n := min(units(r), perReceiver)
		order := newSampleOrder(units(r))
		visit := make([]int, 0, n)

		for range n {
			visit = append(visit, order.take())
		}

		plan = append(plan, PlannedReceiver{Receiver: r, Units: visit})
	}

	return plan
}
