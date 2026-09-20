package geo

import "math"

// coneMaxVertices bounds the clipped polygon in Bounds: a rectangle has four
// corners and each of the three half-planes can add at most one, so seven is
// the most that can survive. The eighth slot is slack.
const coneMaxVertices = 8

// SegmentShadow is the region a segment [a,b] shadows from a point light at
// apex — formally
//
//	{ apex + λ·(q − apex) : q ∈ [a,b], λ ≥ 1 }
//
// which is the intersection of three half-planes: beyond the segment's own
// line away from the apex, and inside each of the two rays from the apex
// through an endpoint.
//
// It exists as a value rather than a function because a caller narrowing a
// search wants it twice over: once as a box, to ask an index which candidates
// to look at, and then once per candidate, to reject the ones the box let
// through. Both answers come from the same three half-planes, computed once.
//
// marginM shifts every boundary outwards by that distance. It is why the
// answers are conservative — a segment that only grazes the shadow is
// reported as inside it — which is what a caller pruning against a test
// computed in floating point needs.
type SegmentShadow struct {
	planes [3]shadowPlane
	count  int

	// degenerate marks an apex on (or within marginM of) the segment's own
	// line, where the construction collapses. Such a shadow excludes nothing:
	// a region that narrows nothing is always safe, and a wrong narrowing
	// would drop a real path.
	degenerate bool
}

// shadowPlane is the half-plane { q : nx·(q.X−o.X) + ny·(q.Y−o.Y) ≤ limit }.
type shadowPlane struct {
	origin Point2D
	nx, ny float64
	limit  float64
}

// at returns how far q is outside the plane; at most 0 means inside.
func (p shadowPlane) at(q Point2D) float64 {
	return p.nx*(q.X-p.origin.X) + p.ny*(q.Y-p.origin.Y) - p.limit
}

// NewSegmentShadow builds the shadow of segment [a,b] cast from apex.
func NewSegmentShadow(apex, a, b Point2D, marginM float64) SegmentShadow {
	shadow := SegmentShadow{planes: [3]shadowPlane{}, count: 0, degenerate: true}

	if !apex.IsFinite() || !a.IsFinite() || !b.IsFinite() {
		return shadow
	}

	dx, dy := b.X-a.X, b.Y-a.Y
	span := math.Hypot(dx, dy)

	// offset is twice the signed area of the triangle (a, b, apex): how far
	// off the segment's line the apex stands, scaled by that line's length.
	// It is also the test value for both wedge edges, since all three
	// triangles are the same one.
	offset := dx*(apex.Y-a.Y) - dy*(apex.X-a.X)
	if span <= 0 || math.Abs(offset) <= marginM*span {
		return shadow
	}

	shadow.degenerate = false

	// Beyond the segment's own line, on the far side from the apex. This is
	// what makes it a shadow rather than a full double cone.
	side := math.Copysign(1, offset)
	shadow.add(shadowPlane{origin: a, nx: -dy * side, ny: dx * side, limit: marginM * span})

	// One wedge edge per endpoint, each keeping the side the other endpoint
	// is on. An edge is dropped when that other endpoint lies within marginM
	// of it, because then the edge cuts through the geometry it should bound.
	shadow.addWedgeEdge(apex, a, offset, marginM)
	shadow.addWedgeEdge(apex, b, -offset, marginM)

	return shadow
}

// MayContainSegment reports whether any part of segment [p,q] can lie in the
// shadow.
//
// It is the cheap test: a segment with both endpoints strictly outside one
// half-plane lies wholly outside that half-plane, and therefore outside the
// shadow. A segment that straddles all three is reported as possible whether
// or not it truly reaches the shadow, which costs the caller one wasted
// candidate and never a lost one.
func (s *SegmentShadow) MayContainSegment(p, q Point2D) bool {
	if s.degenerate {
		return true
	}

	for i := range s.count {
		if s.planes[i].at(p) > 0 && s.planes[i].at(q) > 0 {
			return false
		}
	}

	return true
}

// Bounds returns the bounding box of the shadow clipped to bounds, grown by
// marginM, and reports false when nothing in bounds lies in the shadow.
//
// The shadow is unbounded, which is why the caller supplies the region to clip
// it against: nothing outside the extent of whatever is being searched can be
// a candidate anyway.
func (s *SegmentShadow) Bounds(bounds BBox, marginM float64) (BBox, bool) {
	if !bounds.IsFinite() || !bounds.IsValid() {
		return bounds, true
	}

	if s.degenerate {
		return bounds, true
	}

	var storeA, storeB [coneMaxVertices]Point2D

	poly := append(
		storeA[:0],
		Point2D{X: bounds.MinX, Y: bounds.MinY},
		Point2D{X: bounds.MaxX, Y: bounds.MinY},
		Point2D{X: bounds.MaxX, Y: bounds.MaxY},
		Point2D{X: bounds.MinX, Y: bounds.MaxY},
	)

	for i := range s.count {
		if i%2 == 0 {
			poly = clipConvexHalfPlane(poly, storeB[:0], s.planes[i])
		} else {
			poly = clipConvexHalfPlane(poly, storeA[:0], s.planes[i])
		}
	}

	box, ok := BBoxFromPoints(poly)
	if !ok {
		return BBox{}, false
	}

	return BBox{
		MinX: box.MinX - marginM,
		MinY: box.MinY - marginM,
		MaxX: box.MaxX + marginM,
		MaxY: box.MaxY + marginM,
	}, true
}

func (s *SegmentShadow) add(plane shadowPlane) {
	s.planes[s.count] = plane
	s.count++
}

func (s *SegmentShadow) addWedgeEdge(apex, end Point2D, keepSign, marginM float64) {
	dx, dy := end.X-apex.X, end.Y-apex.Y

	length := math.Hypot(dx, dy)
	if length <= 0 || math.Abs(keepSign) <= marginM*length {
		return
	}

	side := math.Copysign(1, keepSign)
	s.add(shadowPlane{origin: apex, nx: dy * side, ny: -dx * side, limit: marginM * length})
}

// clipConvexHalfPlane keeps the part of the convex polygon src inside plane,
// appending the result to dst (pass a zero-length slice) and returning it.
// Sutherland–Hodgman against a single half-plane, which for a convex input
// yields a convex output with at most one vertex more.
func clipConvexHalfPlane(src, dst []Point2D, plane shadowPlane) []Point2D {
	if len(src) == 0 {
		return dst
	}

	previous := src[len(src)-1]
	previousValue := plane.at(previous)

	for _, current := range src {
		currentValue := plane.at(current)

		// A sign change means the edge crosses the boundary; emit the crossing
		// before whichever endpoint is kept, so the ring stays in order.
		if (currentValue <= 0) != (previousValue <= 0) {
			t := previousValue / (previousValue - currentValue)
			dst = append(dst, Point2D{
				X: previous.X + t*(current.X-previous.X),
				Y: previous.Y + t*(current.Y-previous.Y),
			})
		}

		if currentValue <= 0 {
			dst = append(dst, current)
		}

		previous, previousValue = current, currentValue
	}

	return dst
}
