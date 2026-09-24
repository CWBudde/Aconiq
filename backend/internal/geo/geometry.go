package geo

import (
	"math"
	"slices"

	"github.com/aconiq/backend/internal/numeric"
)

// DistancePointToSegment returns the Euclidean distance from p to segment ab.
func DistancePointToSegment(p, a, b Point2D) float64 {
	if !p.IsFinite() || !a.IsFinite() || !b.IsFinite() {
		return math.NaN()
	}

	abx := b.X - a.X
	aby := b.Y - a.Y

	len2 := abx*abx + aby*aby
	if len2 == 0 {
		return Distance(p, a)
	}

	t := ((p.X-a.X)*abx + (p.Y-a.Y)*aby) / len2
	if t < 0 {
		t = 0
	}

	if t > 1 {
		t = 1
	}

	closest := Point2D{X: a.X + t*abx, Y: a.Y + t*aby}

	return Distance(p, closest)
}

// DistancePointToLineString returns the minimum distance from p to a polyline.
func DistancePointToLineString(p Point2D, line []Point2D) float64 {
	if len(line) == 0 {
		return math.NaN()
	}

	if len(line) == 1 {
		return Distance(p, line[0])
	}

	best := math.MaxFloat64

	for i := range len(line) - 1 {
		d := DistancePointToSegment(p, line[i], line[i+1])
		if d < best {
			best = d
		}
	}

	return best
}

// Distance returns Euclidean distance between two points.
func Distance(a, b Point2D) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y

	return math.Hypot(dx, dy)
}

// PointInPolygon reports whether p is inside polygon exterior and outside all holes.
// Rings format: rings[0] is exterior, rings[1:] are holes. Rings should be closed.
func PointInPolygon(p Point2D, rings [][]Point2D) bool {
	if len(rings) == 0 || len(rings[0]) < 4 {
		return false
	}

	if !pointInRing(p, rings[0]) {
		return false
	}

	for i := 1; i < len(rings); i++ {
		ring := rings[i]
		if len(ring) >= 4 && pointInRing(p, ring) {
			return false
		}
	}

	return true
}

// PointInPolygonInterior reports whether p lies strictly inside the polygon:
// inside the exterior, outside every hole, and on no ring's edge. Rings follow
// PointInPolygon's format.
//
// It differs from PointInPolygon only on the boundary, and deliberately. That
// function counts an edge as inside, which is right for assigning a receiver
// to an area; this one answers "is this point within a building", where a
// point on the facade line is not — a receiver grid aligned with a footprint
// would otherwise lose the whole row of cells along it.
func PointInPolygonInterior(p Point2D, rings [][]Point2D) bool {
	if len(rings) == 0 || len(rings[0]) < 4 {
		return false
	}

	inside, onEdge := pointInRingStrict(p, rings[0])
	if onEdge || !inside {
		return false
	}

	for i := 1; i < len(rings); i++ {
		ring := rings[i]
		if len(ring) < 4 {
			continue
		}

		inHole, onHoleEdge := pointInRingStrict(p, ring)
		if onHoleEdge || inHole {
			return false
		}
	}

	return true
}

// pointInRingStrict is pointInRing with the edge case reported rather than
// folded into the answer: inside is the even-odd test and is meaningless when
// onEdge is true.
func pointInRingStrict(p Point2D, ring []Point2D) (bool, bool) {
	inside := false

	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		pi := ring[i]
		pj := ring[j]

		if DistancePointToSegment(p, pj, pi) < 1e-12 {
			return false, true
		}

		intersects := ((pi.Y > p.Y) != (pj.Y > p.Y)) &&
			(p.X < (pj.X-pi.X)*(p.Y-pi.Y)/(pj.Y-pi.Y)+pi.X)
		if intersects {
			inside = !inside
		}
	}

	return inside, false
}

// pointInRing counts an on-edge point as inside, for stable receiver
// assignment.
func pointInRing(p Point2D, ring []Point2D) bool {
	inside, onEdge := pointInRingStrict(p, ring)

	return inside || onEdge
}

// SegmentIntersection computes the intersection point of segments (a1,a2) and (b1,b2).
// Returns the intersection point, the parameter t along segment a (0..1), and true
// if the segments intersect. Returns zero values and false if they are parallel or
// do not intersect within their extents.
func SegmentIntersection(a1, a2, b1, b2 Point2D) (Point2D, float64, bool) {
	dx1 := a2.X - a1.X
	dy1 := a2.Y - a1.Y
	dx2 := b2.X - b1.X
	dy2 := b2.Y - b1.Y

	denom := dx1*dy2 - dy1*dx2
	if math.Abs(denom) < 1e-12 {
		return Point2D{}, 0, false // parallel or coincident
	}

	dx3 := b1.X - a1.X
	dy3 := b1.Y - a1.Y

	t := (dx3*dy2 - dy3*dx2) / denom
	u := (dx3*dy1 - dy3*dx1) / denom

	if t < 0 || t > 1 || u < 0 || u > 1 {
		return Point2D{}, 0, false // intersection outside segment extents
	}

	return Point2D{
		X: a1.X + t*dx1,
		Y: a1.Y + t*dy1,
	}, t, true
}

// LineStringIntersectsSegment reports whether any edge of a polyline intersects
// the segment (p1,p2). If so, it returns the intersection point closest to p1
// and the intersected edge index. Returns false if no intersection.
func LineStringIntersectsSegment(line []Point2D, p1, p2 Point2D) (Point2D, int, bool) {
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

// BBoxFromPoints computes a bbox from a point slice.
func BBoxFromPoints(points []Point2D) (BBox, bool) {
	if len(points) == 0 {
		return BBox{}, false
	}

	b := BBox{MinX: points[0].X, MinY: points[0].Y, MaxX: points[0].X, MaxY: points[0].Y}
	for _, p := range points[1:] {
		b = b.ExpandToIncludePoint(p)
	}

	if !b.IsFinite() || !b.IsValid() {
		return BBox{}, false
	}

	return b, true
}

// BBoxFromLineString computes a bbox from a polyline.
func BBoxFromLineString(line []Point2D) (BBox, bool) {
	return BBoxFromPoints(line)
}

// BBoxFromPolygon computes a bbox from polygon rings.
func BBoxFromPolygon(rings [][]Point2D) (BBox, bool) {
	if len(rings) == 0 {
		return BBox{}, false
	}

	all := make([]Point2D, 0)
	for _, ring := range rings {
		all = append(all, ring...)
	}

	return BBoxFromPoints(all)
}

// PolygonArea returns the plan-view area of a polygon in square units of the
// projected CRS: the exterior ring less every hole. PolygonCentroid subtracts
// the same holes, so the pair describes one surface. Rings format: rings[0] is
// the exterior, rings[1:] are holes; rings should be closed. Winding order does
// not matter — each ring contributes its absolute area. A polygon whose holes
// exceed its exterior returns 0 rather than a negative area.
//
// The shoelace terms alternate in sign and their count scales with the ring, so
// the reduction is compensated per docs/policies/determinism.md §3.
// areaEpsilon is the threshold below which a shoelace area counts as zero: the
// geometry encloses nothing and has no centroid to report.
const areaEpsilon = 1e-12

func PolygonArea(rings [][]Point2D) float64 {
	if len(rings) == 0 {
		return 0
	}

	total := math.Abs(signedRingArea(rings[0]))
	for _, hole := range rings[1:] {
		total -= math.Abs(signedRingArea(hole))
	}

	if total < 0 {
		return 0
	}

	return total
}

// PolygonCentroid returns the area centroid of a polygon, and reports whether
// it is defined. It is false for an exterior ring with fewer than four points,
// for a degenerate ring enclosing no area, for a polygon whose holes cancel its
// exterior, and for any result that is not finite — the cases where a caller
// must refuse the geometry rather than place a source at an arbitrary point.
//
// Holes are subtracted, as PolygonArea subtracts them: the two must describe the
// same surface, or an area source is placed on a footprint of one size and
// propagated as another. RLS-19 Nr. 3.2 puts the substitute point source "im
// Flächenschwerpunkt jeder Teilfläche", and the Teilfläche of a lot with a
// courtyard in it is the exterior less that courtyard — an exterior-only
// centroid can sit inside the hole.
//
// The centroid of a region that is not simply connected may fall outside it.
// That is a property of the Flächenschwerpunkt the standard asks for, not of
// this implementation.
//
// Like PolygonArea, the shoelace accumulation is compensated.
func PolygonCentroid(rings [][]Point2D) (Point2D, bool) {
	if len(rings) == 0 {
		return Point2D{}, false
	}

	exteriorArea, exterior, ok := ringCentroid(rings[0])
	if !ok {
		return Point2D{}, false
	}

	var (
		area numeric.CompensatedSum
		mx   numeric.CompensatedSum
		my   numeric.CompensatedSum
	)

	area.Add(exteriorArea)
	mx.Add(exteriorArea * exterior.X)
	my.Add(exteriorArea * exterior.Y)

	for _, hole := range rings[1:] {
		holeArea, holeCentroid, holeOK := ringCentroid(hole)
		if !holeOK {
			continue // a degenerate hole removes no area and moves no moment
		}

		area.Add(-holeArea)
		mx.Add(-holeArea * holeCentroid.X)
		my.Add(-holeArea * holeCentroid.Y)
	}

	netArea := area.Sum()
	if math.Abs(netArea) < areaEpsilon {
		return Point2D{}, false
	}

	point := Point2D{X: mx.Sum() / netArea, Y: my.Sum() / netArea}
	if !point.IsFinite() {
		return Point2D{}, false
	}

	return point, true
}

// ringCentroid returns one closed ring's absolute area and its own centroid.
//
// The centroid is winding-independent: the moment sums and the signed double
// area flip sign together, so their quotient does not. The area is returned
// absolute so PolygonCentroid can decide what is exterior and what is a hole by
// ring position, exactly as PolygonArea does, rather than by trusting a winding
// convention the input may not follow.
func ringCentroid(ring []Point2D) (float64, Point2D, bool) {
	if len(ring) < 4 {
		return 0, Point2D{}, false
	}

	var (
		doubleArea numeric.CompensatedSum
		cx         numeric.CompensatedSum
		cy         numeric.CompensatedSum
	)

	for i := range len(ring) - 1 {
		if !ring[i].IsFinite() || !ring[i+1].IsFinite() {
			return 0, Point2D{}, false
		}

		cross := ring[i].X*ring[i+1].Y - ring[i+1].X*ring[i].Y

		doubleArea.Add(cross)
		cx.Add((ring[i].X + ring[i+1].X) * cross)
		cy.Add((ring[i].Y + ring[i+1].Y) * cross)
	}

	area2 := doubleArea.Sum()
	if math.Abs(area2) < areaEpsilon {
		return 0, Point2D{}, false
	}

	factor := 1.0 / (3.0 * area2)

	return math.Abs(area2) / 2.0, Point2D{X: cx.Sum() * factor, Y: cy.Sum() * factor}, true
}

// signedRingArea returns the signed shoelace area of one closed ring: positive
// for counter-clockwise winding, negative for clockwise. Callers that only want
// a magnitude take the absolute value.
func signedRingArea(ring []Point2D) float64 {
	if len(ring) < 3 {
		return 0
	}

	var sum numeric.CompensatedSum

	for i := range len(ring) - 1 {
		sum.Add(ring[i].X*ring[i+1].Y - ring[i+1].X*ring[i].Y)
	}

	return 0.5 * sum.Sum()
}

// spanParameterEpsilon is the smallest distinguishable interval, measured in
// the parameter t along a segment. Two crossings closer than this — a ray
// leaving a polygon through a vertex, say, which every adjacent edge reports —
// describe one cut, and an interval shorter than it has no midpoint worth
// classifying.
const spanParameterEpsilon = 1e-12

// SegmentSpan is one interval of a segment over which the covering polygon does
// not change, expressed in the parameter t along that segment: Start and End
// are in [0,1], and Polygon indexes the covering polygon or is -1 where no
// polygon covers it.
type SegmentSpan struct {
	Start   float64
	End     float64
	Polygon int
}

// Length returns the span's share of the segment, in the same t parameter.
func (s SegmentSpan) Length() float64 {
	return s.End - s.Start
}

// SegmentPolygonSpans splits the segment ab into the maximal intervals over
// which the covering polygon stays the same. Each polygon is a ring set in the
// PointInPolygon convention — rings[0] exterior, rings[1:] holes.
//
// Where polygons overlap the **first** one in slice order wins, so the answer
// is the caller's declared order and never map iteration; where none covers,
// the span carries -1 and the caller supplies its own fallback. Spans are
// returned in order from a to b and cover [0,1] exactly.
//
// A segment running exactly along a polygon edge is classified as covered:
// SegmentIntersection reports no crossing for collinear edges, and
// PointInPolygon treats an on-edge point as inside. That is the same rule
// receiver assignment already follows, rather than a second one.
func SegmentPolygonSpans(a, b Point2D, polygons [][][]Point2D) []SegmentSpan {
	cuts := segmentCutParameters(a, b, polygons)

	spans := make([]SegmentSpan, 0, len(cuts))

	for i := range len(cuts) - 1 {
		start, end := cuts[i], cuts[i+1]
		if end-start <= spanParameterEpsilon {
			continue
		}

		midpoint := Point2D{
			X: a.X + 0.5*(start+end)*(b.X-a.X),
			Y: a.Y + 0.5*(start+end)*(b.Y-a.Y),
		}

		span := SegmentSpan{Start: start, End: end, Polygon: coveringPolygon(midpoint, polygons)}

		// Merge with the previous span rather than emitting a boundary the
		// covering polygon does not actually change across: a crossing into a
		// hole of one polygon and out of its neighbour cuts the segment twice
		// without changing the answer.
		if len(spans) > 0 && spans[len(spans)-1].Polygon == span.Polygon {
			spans[len(spans)-1].End = span.End

			continue
		}

		spans = append(spans, span)
	}

	if len(spans) == 0 {
		return []SegmentSpan{{Start: 0, End: 1, Polygon: coveringPolygon(a, polygons)}}
	}

	return spans
}

// segmentCutParameters returns the sorted, deduplicated parameters along ab at
// which the covering polygon can change: 0, 1, and every ring-edge crossing.
func segmentCutParameters(a, b Point2D, polygons [][][]Point2D) []float64 {
	cuts := []float64{0, 1}

	// Edges are walked with the same wrap-around pointInRing uses, so a ring
	// written without its closing point is cut where it is tested.
	for _, rings := range polygons {
		for _, ring := range rings {
			for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
				_, t, ok := SegmentIntersection(a, b, ring[j], ring[i])
				if ok {
					cuts = append(cuts, t)
				}
			}
		}
	}

	slices.Sort(cuts)

	deduplicated := cuts[:1]

	for _, t := range cuts[1:] {
		if t-deduplicated[len(deduplicated)-1] > spanParameterEpsilon {
			deduplicated = append(deduplicated, t)
		}
	}

	return deduplicated
}

// coveringPolygon returns the index of the first polygon covering p, or -1.
func coveringPolygon(p Point2D, polygons [][][]Point2D) int {
	for index, rings := range polygons {
		if PointInPolygon(p, rings) {
			return index
		}
	}

	return -1
}
