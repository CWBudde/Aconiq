package road

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// barrierFan is the receiver's index over the barriers: which barriers a ray
// ending at the receiver can cross, looked up by the ray's direction.
//
// Every path the walk shields ends at the receiver — the direct path from a
// Teilstück and each mirrored path from its image source alike — and a
// mirrored path's image source can be a kilometre away. The grid answers such
// a ray by walking every cell it passes through and returning every barrier
// listed there, most of which the ray misses by metres; on a building-dense
// model that candidate list, and the crossing test run on each of its edges,
// was most of the cost of shielding a mirrored path.
//
// Seen from the receiver, a barrier occupies an interval of directions: the
// angles its polyline sweeps. A ray from the receiver can only cross it in
// that interval and no nearer than the barrier's bounding box. The fan buckets
// the barriers by those intervals into fanBins equal sectors, once per
// receiver, and a query reads the one sector its direction falls in.
//
// It is a prune, and exact for the same reason the grid is: the candidates it
// answers are a superset of the barriers the crossing test can accept, in
// ascending order. The floating-point margin is the one argument to make. A
// crossing SegmentIntersection accepts lies within ε — a few units in the
// last place of the coordinates — of a point Y on the barrier, measured
// perpendicular to the ray, however grazing the ray. Y's direction is inside
// the barrier's interval, and the ray's direction differs from it by at most
// ε/|Y−R|. A barrier that comes within fanClearanceM of the receiver is
// therefore kept in every sector, which bounds that angle by ε/fanClearanceM —
// below 1e-7 rad for any ε under 1e-7 m — and every interval is widened by
// fanMarginRad, ten times that, before it is bucketed.
type barrierFan struct {
	barriers []Barrier
	receiver geo.Point2D
	ready    bool

	// start and items hold the sector lists in compressed-row form: sector s
	// owns items[start[s]:start[s+1]], ascending.
	start []int32
	items []int32

	// omni holds the barriers too close to the receiver, or wrapping too far
	// around it, to be given an interval; they are candidates for every ray.
	omni []int32

	// nearSq is, per barrier, the squared distance from the receiver to its
	// bounding box: no crossing on a ray shorter than that.
	nearSq []float64

	// lo and hi are each barrier's sector range, kept between the two passes
	// of the build. hi < lo marks an omni barrier.
	lo, hi []int32
}

const (
	// fanBins is the number of direction sectors. At a kilometre one sector
	// is ~1.5 m wide.
	fanBins = 4096

	// fanClearanceM is how close to the receiver a barrier may come and still
	// be given an interval.
	fanClearanceM = 1.0

	// fanMarginRad widens every interval on both sides.
	fanMarginRad = 1e-6

	// fanReachMarginM is the slack on the distance test, in metres.
	fanReachMarginM = 1e-3
)

// fanFor returns the fan for receiver over barriers, building it when the
// scratch last served another receiver or another barrier set, or nil when
// there is no scratch to hold one.
func (scratch *pathScratch) fanFor(receiver geo.Point2D, barriers []Barrier) *barrierFan {
	if scratch == nil || len(barriers) == 0 {
		return nil
	}

	fan := &scratch.fan
	if !fan.ready || fan.receiver != receiver || !sameBarriers(fan.barriers, barriers) {
		fan.build(receiver, barriers)
	}

	return fan
}

// sameBarriers reports whether two slices are the same slice — the same
// backing array and length — which is what the fan was built over.
func sameBarriers(a, b []Barrier) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// build buckets every barrier for one receiver.
func (f *barrierFan) build(receiver geo.Point2D, barriers []Barrier) {
	f.barriers = barriers
	f.receiver = receiver
	f.ready = true

	n := len(barriers)
	f.nearSq = growFloat64(f.nearSq, n)
	f.lo = growInt32(f.lo, n)
	f.hi = growInt32(f.hi, n)
	f.omni = f.omni[:0]
	f.start = growInt32(f.start, fanBins+1)
	clear(f.start)

	for k := range barriers {
		lo, hi, near, ok := fanInterval(receiver, barriers[k].Geometry)
		f.nearSq[k] = near

		if !ok {
			f.lo[k], f.hi[k] = 0, -1
			f.omni = append(f.omni, int32(k))

			continue
		}

		f.lo[k], f.hi[k] = lo, hi
		for s := lo; s <= hi; s++ {
			f.start[fanSector(s)+1]++
		}
	}

	for s := 1; s <= fanBins; s++ {
		f.start[s] += f.start[s-1]
	}

	f.items = growInt32(f.items, int(f.start[fanBins]))

	// Fill in barrier order, so every sector list comes out ascending. The
	// starts advance as the lists fill and are shifted back afterwards.
	for k := range barriers {
		for s := f.lo[k]; s <= f.hi[k]; s++ {
			sector := fanSector(s)
			f.items[f.start[sector]] = int32(k)
			f.start[sector]++
		}
	}

	copy(f.start[1:], f.start[:fanBins])
	f.start[0] = 0
}

// fanSector folds an unwrapped sector number into [0, fanBins).
func fanSector(s int32) int32 {
	s %= fanBins
	if s < 0 {
		s += fanBins
	}

	return s
}

// fanSectorOf returns the unwrapped sector an angle in radians falls in.
func fanSectorOf(angle float64) int32 {
	return int32(math.Floor((angle + math.Pi) * (fanBins / (2 * math.Pi))))
}

// fanInterval returns the sector range a polyline occupies as seen from
// receiver, unwrapped so that lo ≤ hi, and the squared distance from the
// receiver to its bounding box. It reports false when the polyline must be a
// candidate for every direction: it comes within fanClearanceM of the
// receiver, it wraps all the way around, or it is degenerate.
func fanInterval(receiver geo.Point2D, line []geo.Point2D) (int32, int32, float64, bool) {
	box, ok := geo.BBoxFromLineString(line)
	if !ok {
		return 0, 0, 0, false
	}

	near := box.SquaredDistanceToPoint(receiver)
	if len(line) < 2 {
		return 0, 0, near, false
	}

	// Walk the polyline as a continuous sweep of direction. No edge passes
	// within fanClearanceM of the receiver, so no edge sweeps as much as π
	// and the wrapped difference between its ends is its true sweep.
	first := math.Atan2(line[0].Y-receiver.Y, line[0].X-receiver.X)
	previous := first
	swept, low, high := 0.0, 0.0, 0.0

	for i := 1; i < len(line); i++ {
		if pointSegmentDistance(receiver, line[i-1], line[i]) < fanClearanceM {
			return 0, 0, near, false
		}

		current := math.Atan2(line[i].Y-receiver.Y, line[i].X-receiver.X)

		step := current - previous
		if step > math.Pi {
			step -= 2 * math.Pi
		} else if step < -math.Pi {
			step += 2 * math.Pi
		}

		swept += step
		low = math.Min(low, swept)
		high = math.Max(high, swept)
		previous = current
	}

	if high-low+2*fanMarginRad >= 2*math.Pi {
		return 0, 0, near, false
	}

	lo := fanSectorOf(first + low - fanMarginRad)
	hi := fanSectorOf(first + high + fanMarginRad)

	if hi-lo+1 >= fanBins {
		return 0, 0, near, false
	}

	return lo, hi, near, true
}

// pointSegmentDistance is the distance from p to segment [a,b].
func pointSegmentDistance(p, a, b geo.Point2D) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	len2 := dx*dx + dy*dy

	t := 0.0
	if len2 > 0 {
		t = ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / len2
		t = math.Max(0, math.Min(1, t))
	}

	return math.Hypot(a.X+t*dx-p.X, a.Y+t*dy-p.Y)
}

// appendCandidates appends, ascending, every barrier a ray from source to the
// fan's receiver can cross.
func (f *barrierFan) appendCandidates(dst []int, source geo.Point2D) []int {
	dx, dy := source.X-f.receiver.X, source.Y-f.receiver.Y

	reach := math.Sqrt(dx*dx+dy*dy) + fanReachMarginM
	reachSq := reach * reach

	sector := fanSector(fanSectorOf(math.Atan2(dy, dx)))
	listed := f.items[f.start[sector]:f.start[sector+1]]

	// Merge the sector's list with the omni list; both are ascending and no
	// barrier is in both.
	i, j := 0, 0
	for i < len(listed) || j < len(f.omni) {
		var k int32

		if j >= len(f.omni) || (i < len(listed) && listed[i] < f.omni[j]) {
			k = listed[i]
			i++
		} else {
			k = f.omni[j]
			j++
		}

		if f.nearSq[k] <= reachSq {
			dst = append(dst, int(k))
		}
	}

	return dst
}

func growFloat64(s []float64, n int) []float64 {
	if cap(s) < n {
		return make([]float64, n)
	}

	return s[:n]
}
