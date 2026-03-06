package geo

import "math"

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
		if len(rings[i]) >= 4 && pointInRing(p, rings[i]) {
			return false
		}
	}

	return true
}

func pointInRing(p Point2D, ring []Point2D) bool {
	inside := false

	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		pi := ring[i]
		pj := ring[j]

		// On-edge is treated as inside for stable receiver assignment.
		if DistancePointToSegment(p, pj, pi) < 1e-12 {
			return true
		}

		intersects := ((pi.Y > p.Y) != (pj.Y > p.Y)) &&
			(p.X < (pj.X-pi.X)*(p.Y-pi.Y)/(pj.Y-pi.Y)+pi.X)
		if intersects {
			inside = !inside
		}
	}

	return inside
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
