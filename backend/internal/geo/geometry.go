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
	for i := 0; i < len(line)-1; i++ {
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
