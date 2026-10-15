package geo

import (
	"math"
	"slices"
)

// This file holds the plan-view screening geometry that every standard with
// barrier diffraction needs and none of them owns: finding where a
// source→receiver ray crosses an obstacle, deciding which crossings actually
// break the line of sight, and selecting the significant diffraction edges by
// the rubber band method (Gummibandmethode).
//
// It is deliberately free of any acoustics. What a standard does with the
// selected edges — RLS-19 Eqs. 15-17, Schall 03 Gl. 20-26, ISO 9613-2
// Gl. 12-18 — stays with that standard, and so does any rule about which
// vertex a path may round: only Schall 03 has lateral diffraction, so only
// Schall 03 needs to group panels into obstacles.

// ScreeningPoint is one candidate diffraction edge in the vertical
// source→receiver cross-section: how far along the ray it sits, and how high
// its top edge stands above the same datum as the source and receiver heights.
type ScreeningPoint struct {
	// DistFromSource is the plan-view distance from the source [m].
	DistFromSource float64
	// TopHeightM is the top-of-obstacle height [m].
	TopHeightM float64
}

// HullPoint is a point in the vertical source→receiver cross-section.
//
// Index identifies the caller's candidate that the point came from, and is
// negative for the source and receiver bookends, which are never diffraction
// edges.
type HullPoint struct {
	Dist   float64
	Height float64
	Index  int
}

// HullCollinearToleranceM is how far above the line between its neighbours a
// point must stand, measured vertically, for the rubber band to rest on it.
//
// A point on the line is a tie the band may resolve either way, and the
// standards' diffraction terms are not continuous across it: a second edge
// switches multiple diffraction on, which in Schall 03 Gl. 25/26 and RLS-19
// raises A_bar by up to 10 lg 3 at once. A cross-section with an edge exactly
// on the line is not rare — a source and two barrier tops at heights in
// arithmetic step stand on one line for every ray across them — and the
// computed cross product of such a triple is zero only as long as every
// rounding falls the same way. One rounding difference anywhere upstream (a
// fused multiply-add on arm64, a refactored midpoint) moved it to either side,
// and A_bar with it. A point within this tolerance of the band is therefore
// dropped, as an exactly collinear one always was.
//
// The tolerance is held against the band that results, not against the chords
// the hull passes through while it is being built: dropping points one chord
// at a time would let deviations add up along a near-collinear chain, until a
// point well clear of the final band went with the rest. See
// thinUpperConvexHull.
//
// It is a vertical distance on purpose: rescaling the distance axis, as
// ISO 9613-2 does to measure perpendicular to an edge, scales the cross
// product and the chord's run alike and leaves the selection unchanged. A
// micrometre is far beyond the rounding of any coordinate the engine computes
// in, and far below anything a barrier height is surveyed to.
const HullCollinearToleranceM = 1e-6

// UpperConvexHull returns the upper convex hull of points already sorted by
// Dist, by Andrew's monotone chain run over the upper side only, with every
// vertex that stands no more than HullCollinearToleranceM above the resulting
// band thinned out.
//
// A point is dropped when it lies on or below the line between its
// neighbours, which is exactly the rubber band laid over the cross-section:
// what survives is what the band rests on.
func UpperConvexHull(points []HullPoint) []HullPoint {
	if len(points) <= 2 {
		return points
	}

	return appendUpperConvexHull(make([]HullPoint, 0, len(points)), points)
}

// appendUpperConvexHull is UpperConvexHull into a caller's buffer: the hull of
// points, which must number more than two, is written to hull — pass a
// zero-length slice — and returned.
func appendUpperConvexHull(hull, points []HullPoint) []HullPoint {
	n := len(points)

	for i := range n {
		for len(hull) >= 2 {
			a := hull[len(hull)-2]
			b := hull[len(hull)-1]
			c := points[i]

			// Cross product of (a→b) × (a→c). If it is >= 0, b lies on or
			// below the line a→c and the band does not touch it.
			cross := (b.Dist-a.Dist)*(c.Height-a.Height) - (b.Height-a.Height)*(c.Dist-a.Dist)

			if cross < 0 {
				break // b protrudes above a→c, so the band rests on it.
			}

			hull = hull[:len(hull)-1]
		}

		hull = append(hull, points[i])
	}

	return thinUpperConvexHull(hull)
}

// droppedHullIndex marks a vertex thinUpperConvexHull has dropped. Bookends
// carry -1 and candidates their index, so no point ever carries it.
const droppedHullIndex = math.MinInt

// thinUpperConvexHull drops, from an exact upper hull, every vertex that stands
// no more than HullCollinearToleranceM above the band through the vertices it
// keeps, and returns the hull compacted in place.
//
// It works top-down, as Douglas–Peucker does: between two kept vertices it
// finds the one that stands highest above their chord, keeps it if it clears
// the tolerance and splits there, and otherwise drops everything in between.
// So every dropped vertex is measured against the very chord of the band that
// passes over it, and nothing accumulates. A kept vertex cleared the chord it
// was tested against; the neighbours it ends up with stand above that chord,
// so it may stand closer to theirs, but the band still rests on it — it is a
// vertex of the exact hull, never a point the band would pass under.
//
// A point exactly on a chord is dropped by the exact hull already, and one a
// rounding above it here; both leave the same band.
func thinUpperConvexHull(hull []HullPoint) []HullPoint {
	if len(hull) <= 2 {
		return hull
	}

	markDroppedHullVertices(hull, 0, len(hull)-1)

	kept := hull[:0]

	for _, point := range hull {
		if point.Index != droppedHullIndex {
			kept = append(kept, point)
		}
	}

	return kept
}

// markDroppedHullVertices marks the vertices strictly between hull[lo] and
// hull[hi], both kept, that the band between them does not rest on.
func markDroppedHullVertices(hull []HullPoint, lo, hi int) {
	if hi-lo < 2 {
		return
	}

	a, c := hull[lo], hull[hi]

	// -cross is h·(c.Dist-a.Dist), h the height of a vertex above the chord
	// a→c at its distance; the run is the same for every vertex here, so the
	// highest vertex has the largest -cross. The first of equals wins.
	best, bestRise := -1, 0.0

	for k := lo + 1; k < hi; k++ {
		b := hull[k]

		rise := (b.Height-a.Height)*(c.Dist-a.Dist) - (b.Dist-a.Dist)*(c.Height-a.Height)
		if best < 0 || rise > bestRise {
			best, bestRise = k, rise
		}
	}

	if bestRise > HullCollinearToleranceM*(c.Dist-a.Dist) {
		markDroppedHullVertices(hull, lo, best)
		markDroppedHullVertices(hull, best, hi)

		return
	}

	for k := lo + 1; k < hi; k++ {
		hull[k].Index = droppedHullIndex
	}
}

// ObstructsLineOfSight reports whether an obstacle standing topHeightM high at
// distFromSource along the ray breaks the straight line from a source at
// sourceHeightM to a receiver at receiverHeightM, totalDistM away.
//
// Heights are compared against one datum and the function does not care which,
// as long as all four use the same one. A degenerate ray obstructs nothing.
func ObstructsLineOfSight(distFromSource, topHeightM, sourceHeightM, receiverHeightM, totalDistM float64) bool {
	if totalDistM <= 0 {
		return false
	}

	frac := distFromSource / totalDistM
	lineOfSightHeight := sourceHeightM + frac*(receiverHeightM-sourceHeightM)

	return topHeightM > lineOfSightHeight
}

// SelectDiffractionEdges implements the rubber band method and returns the
// indices, into candidates, of the significant diffraction edges in order from
// source to receiver.
//
// candidates must be sorted by DistFromSource and pre-filtered with
// ObstructsLineOfSight: a candidate that does not break the line of sight is
// not a diffraction edge, and leaving it in would let the hull select it.
// Filtering is the caller's step because a caller may need to do something
// between the two — Schall 03 merges the two crossings a ray makes through a
// footprint corner, which it can only do once the crossings are known.
//
// The returned slice is nil when nothing is selected, so a caller can treat
// "no candidates" and "no edges" alike.
func SelectDiffractionEdges(
	sourceHeightM, receiverHeightM, totalDistM float64,
	candidates []ScreeningPoint,
) []int {
	return AppendDiffractionEdges(nil, &HullBuffer{}, sourceHeightM, receiverHeightM, totalDistM, candidates)
}

// HullBuffer is the working memory of AppendDiffractionEdges, for a caller
// that selects edges in a loop and would otherwise allocate it every time.
// The zero value is ready to use; it must not be shared between goroutines.
type HullBuffer struct {
	points []HullPoint
	hull   []HullPoint
}

// AppendDiffractionEdges is SelectDiffractionEdges appending to dst and
// working in buf, so that a hot loop allocates nothing once the buffers have
// grown. It selects exactly what SelectDiffractionEdges selects — that
// function is this one with fresh buffers — and appends nothing when nothing
// is selected, so a nil dst stays nil.
func AppendDiffractionEdges(
	dst []int,
	buf *HullBuffer,
	sourceHeightM, receiverHeightM, totalDistM float64,
	candidates []ScreeningPoint,
) []int {
	if len(candidates) == 0 {
		return dst
	}

	// The source and receiver bookend the section so the hull has something to
	// rest on at both ends; they carry index -1 and are dropped again below.
	points := buf.points[:0]
	points = append(points, HullPoint{Dist: 0, Height: sourceHeightM, Index: -1})

	for i, candidate := range candidates {
		points = append(points, HullPoint{
			Dist:   candidate.DistFromSource,
			Height: candidate.TopHeightM,
			Index:  i,
		})
	}

	points = append(points, HullPoint{Dist: totalDistM, Height: receiverHeightM, Index: -1})
	buf.points = points

	// At least three points here — two bookends and a candidate — which is
	// what appendUpperConvexHull requires.
	hull := appendUpperConvexHull(buf.hull[:0], points)
	buf.hull = hull

	for _, point := range hull {
		if point.Index < 0 {
			continue
		}

		dst = append(dst, point.Index)
	}

	return dst
}

// RayCrossing records where a source→receiver ray crosses one obstacle in plan
// view. ObstacleIndex indexes the caller's own slice, so the caller recovers
// its own type rather than this package learning about barriers.
type RayCrossing struct {
	Point          Point2D
	DistFromSource float64
	ObstacleIndex  int

	// SegmentIndex is the index, within that obstacle's polyline, of the
	// segment the ray crosses. The crossed segment's orientation is what a
	// standard needs to decompose the path into the plane perpendicular to
	// the diffraction edge and the component parallel to it; without it a
	// caller can only treat every screen as if it stood square to the ray.
	SegmentIndex int
}

// RayCrossings returns every obstacle whose plan-view polyline the line from
// source to receiver crosses, nearest first.
//
// polyline reads the plan-view geometry off one obstacle. An obstacle is
// counted at most once, at the crossing nearest the source, because a polyline
// that doubles back is one obstacle and not two diffraction edges at the same
// height.
//
// endpointToleranceM drops a crossing that close to either endpoint: a ray
// that starts or ends on the obstacle grazes it rather than passing behind it.
// Pass 0 to keep every crossing, which is what a caller wants when it merges
// coincident crossings itself.
//
// The sort is stable, so obstacles crossed at the same distance stay in input
// order and the result does not depend on the slice's iteration having been
// shuffled.
func RayCrossings[T any](
	source, receiver Point2D,
	obstacles []T,
	polyline func(T) []Point2D,
	endpointToleranceM float64,
) []RayCrossing {
	var crossings []RayCrossing

	for i := range obstacles {
		crossings = appendRayCrossing(crossings, source, receiver, obstacles[i], i, polyline, endpointToleranceM)
	}

	sortRayCrossings(crossings)

	return crossings
}

// AppendRayCrossings is RayCrossings over a caller-supplied candidate set,
// appending to dst so that a hot loop can reuse one buffer.
//
// candidates indexes obstacles and **must be in ascending order**. The sort
// below is stable, so obstacles crossed at the same distance keep their slice
// order; walking the candidates in any other order would reorder exactly those
// ties, silently and reproducibly. A spatial index answers in an order that is
// deterministic but is not slice order, which is why BBoxGridCursor.Query
// sorts before returning — do not undo that.
//
// candidates must also be a superset of the obstacles the ray can cross. That
// is the caller's proof to make, not this function's: everything here does is
// skip the obstacles it was not handed.
func AppendRayCrossings[T any](
	dst []RayCrossing,
	source, receiver Point2D,
	obstacles []T,
	candidates []int,
	polyline func(T) []Point2D,
	endpointToleranceM float64,
) []RayCrossing {
	start := len(dst)

	for _, i := range candidates {
		dst = appendRayCrossing(dst, source, receiver, obstacles[i], i, polyline, endpointToleranceM)
	}

	sortRayCrossings(dst[start:])

	return dst
}

// appendRayCrossing tests one obstacle and appends its crossing, if any.
func appendRayCrossing[T any](
	dst []RayCrossing,
	source, receiver Point2D,
	obstacle T,
	index int,
	polyline func(T) []Point2D,
	endpointToleranceM float64,
) []RayCrossing {
	point, segment, ok := LineStringIntersectsSegment(polyline(obstacle), source, receiver)
	if !ok {
		return dst
	}

	return appendCrossingAt(dst, source, receiver, point, index, segment, endpointToleranceM)
}

// appendCrossingAt appends the crossing found at point, unless it grazes
// either end of the ray.
func appendCrossingAt(
	dst []RayCrossing,
	source, receiver, point Point2D,
	index, segment int,
	endpointToleranceM float64,
) []RayCrossing {
	distFromSource := Distance(source, point)
	if endpointToleranceM > 0 &&
		(distFromSource < endpointToleranceM || Distance(point, receiver) < endpointToleranceM) {
		return dst
	}

	return append(dst, RayCrossing{
		Point:          point,
		DistFromSource: distFromSource,
		ObstacleIndex:  index,
		SegmentIndex:   segment,
	})
}

// RayEdges holds the plan-view edges of an obstacle slice, laid out for the
// ray test rather than for reading: one array per quantity, each obstacle's
// edges contiguous, and each edge's direction computed once instead of once
// per ray. A scene builds it once; every ray of every receiver reads it.
type RayEdges struct {
	x, y   []float64 // the edge's start vertex
	dx, dy []float64 // end minus start

	// first[i] is obstacle i's first edge, and first[i+1] is one past its
	// last; an obstacle with fewer than two vertices has none.
	first []int
}

// NewRayEdges lays out the edges of obstacles, in slice order.
func NewRayEdges[T any](obstacles []T, polyline func(T) []Point2D) *RayEdges {
	n := 0
	for _, obstacle := range obstacles {
		n += max(len(polyline(obstacle))-1, 0)
	}

	edges := &RayEdges{
		x:     make([]float64, 0, n),
		y:     make([]float64, 0, n),
		dx:    make([]float64, 0, n),
		dy:    make([]float64, 0, n),
		first: make([]int, 0, len(obstacles)+1),
	}

	for _, obstacle := range obstacles {
		edges.first = append(edges.first, len(edges.x))

		line := polyline(obstacle)
		for k := 1; k < len(line); k++ {
			a, b := line[k-1], line[k]
			edges.x = append(edges.x, a.X)
			edges.y = append(edges.y, a.Y)
			edges.dx = append(edges.dx, b.X-a.X)
			edges.dy = append(edges.dy, b.Y-a.Y)
		}
	}

	edges.first = append(edges.first, len(edges.x))

	return edges
}

// Len is the number of obstacles the edges were laid out from.
func (e *RayEdges) Len() int { return len(e.first) - 1 }

// AppendRayEdgeCrossings is AppendRayCrossings over obstacles already laid
// out as RayEdges. It answers with the same crossings, to the bit and in the
// same order, and carries the same contract on candidates: ascending, and a
// superset of the obstacles the ray can cross.
func AppendRayEdgeCrossings(
	dst []RayCrossing,
	source, receiver Point2D,
	edges *RayEdges,
	candidates []int,
	endpointToleranceM float64,
) []RayCrossing {
	start := len(dst)
	dx1 := receiver.X - source.X
	dy1 := receiver.Y - source.Y

	for _, i := range candidates {
		lo, hi := edges.first[i], edges.first[i+1]

		t, segment := nearestEdgeCrossing(
			edges.x[lo:hi], edges.y[lo:hi], edges.dx[lo:hi], edges.dy[lo:hi],
			source, dx1, dy1,
		)
		if segment < 0 {
			continue
		}

		point := Point2D{X: source.X + t*dx1, Y: source.Y + t*dy1}
		dst = appendCrossingAt(dst, source, receiver, point, i, segment, endpointToleranceM)
	}

	sortRayCrossings(dst[start:])

	return dst
}

// nearestEdgeCrossing is LineStringIntersectsSegment's loop over one
// obstacle's edges in the RayEdges layout. It returns the ray parameter of the
// nearest crossing and the crossed edge, or -1 for none.
func nearestEdgeCrossing(x, y, dx, dy []float64, p1 Point2D, dx1, dy1 float64) (float64, int) {
	y = y[:len(x)]
	dx = dx[:len(x)]
	dy = dy[:len(x)]

	bestT := math.MaxFloat64
	bestEdge := -1

	for k := range x {
		t, ok := rayEdgeT(dx1, dy1, dx[k], dy[k], x[k]-p1.X, y[k]-p1.Y)
		if ok && t < bestT {
			bestT = t
			bestEdge = k
		}
	}

	return bestT, bestEdge
}

// rayCrossingInsertionMax is the length up to which sortRayCrossings sorts by
// insertion rather than by slices.SortStableFunc. It is a measured number, not
// a guess: BenchmarkSortRayCrossings runs both implementations over the same
// inputs, and insertion wins by a wide margin to 64 and is still ahead at 256.
// The cut is set at 64 anyway, because insertion is O(m²) and a ray through a
// dense city can cross far more barriers than the handful that is typical —
// past the cut the asymptotics have to win eventually, and there is no reason
// to find out where by shipping it.
const rayCrossingInsertionMax = 64

// sortRayCrossings orders crossings by distance from the source, stably, so
// that equidistant obstacles keep the order the caller listed them in.
//
// The permutation is the one slices.SortStableFunc produces, to the bit, and
// not merely an equally valid sort of the same keys. Determinism policy leaves
// no output tolerance here, so that is worth spelling out rather than trusting:
//
//   - A stable sort's output is uniquely determined. For any two elements a and
//     b, a precedes b in the result exactly when key(a) < key(b), or when the
//     keys are equal and a came earlier in the input. That is a total order on
//     the input positions, so exactly one arrangement satisfies it.
//   - The insertion loop below shifts a predecessor rightwards only while its
//     key is **strictly** greater than the key being placed. It therefore stops
//     at the first predecessor that is not strictly greater — one with a
//     smaller key, or one with an equal key — and drops the element after it.
//     An element thus lands after every earlier element with an equal key and
//     after every element with a smaller key, which is the arrangement above.
//
// The argument needs a comparator that is a total preorder, and a NaN key is
// not: `>` is false in both directions, so an insertion sort leaves a NaN
// wherever it found it while SortStableFunc over an inconsistent comparator
// promises nothing at all. NaN is not reachable from any input a propagation
// walk produces — DistFromSource is Hypot of two differences of coordinates
// that LineStringIntersectsSegment has already discarded a non-finite
// parameter for, and a projected CRS puts those coordinates around 1e6, ten
// orders short of where a coordinate difference could overflow to an infinity
// and make 0*Inf out of the intersection point. "Not reachable in practice"
// is not "not reachable", though, and a divergence here would be silent, so
// the check below is cheap insurance rather than a proof: a non-finite key
// sends the whole slice down the SortStableFunc path it took before, which
// cannot differ from itself.
func sortRayCrossings(crossings []RayCrossing) {
	if len(crossings) < 2 {
		return
	}

	if len(crossings) <= rayCrossingInsertionMax && rayCrossingKeysOrdered(crossings) {
		insertionSortRayCrossings(crossings)

		return
	}

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

// rayCrossingKeysOrdered reports whether every key admits a total preorder
// under `<`, which for a float64 means only that none of them is NaN.
func rayCrossingKeysOrdered(crossings []RayCrossing) bool {
	for i := range crossings {
		if math.IsNaN(crossings[i].DistFromSource) {
			return false
		}
	}

	return true
}

// insertionSortRayCrossings is the stable sort of the doc comment above, kept
// separate so that a test can hold it against slices.SortStableFunc directly.
// The inner guard must stay strictly `>`; `>=` would reverse equal keys and
// silently reorder equidistant obstacles.
func insertionSortRayCrossings(crossings []RayCrossing) {
	for i := 1; i < len(crossings); i++ {
		placed := crossings[i]

		j := i - 1
		for j >= 0 && crossings[j].DistFromSource > placed.DistFromSource {
			crossings[j+1] = crossings[j]
			j--
		}

		crossings[j+1] = placed
	}
}
