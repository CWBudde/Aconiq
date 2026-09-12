package geo

import "math"

// LineStringLength returns the total length of a polyline, or NaN when any
// vertex is not finite.
func LineStringLength(line []Point2D) float64 {
	if len(line) < 2 {
		return 0
	}

	total := 0.0

	for i := range len(line) - 1 {
		if !line[i].IsFinite() || !line[i+1].IsFinite() {
			return math.NaN()
		}

		total += Distance(line[i], line[i+1])
	}

	return total
}

// ProjectPointOntoLineString returns the chainage of the point on line closest
// to p — the distance from the start of line, measured along it — together with
// the perpendicular distance from p to that point.
//
// ok is false when line has fewer than two vertices or any input is not finite.
// When several points are equally close the first one along the line wins, so
// the result is deterministic for self-touching geometry.
func ProjectPointOntoLineString(p Point2D, line []Point2D) (chainageM, distanceM float64, ok bool) {
	if len(line) < 2 || !p.IsFinite() {
		return 0, 0, false
	}

	bestChainage := 0.0
	bestDistance := math.MaxFloat64
	travelled := 0.0

	for i := range len(line) - 1 {
		a := line[i]
		b := line[i+1]

		if !a.IsFinite() || !b.IsFinite() {
			return 0, 0, false
		}

		abx := b.X - a.X
		aby := b.Y - a.Y
		len2 := abx*abx + aby*aby
		segLen := math.Sqrt(len2)

		t := 0.0
		if len2 > 0 {
			t = ((p.X-a.X)*abx + (p.Y-a.Y)*aby) / len2
			t = math.Min(math.Max(t, 0), 1)
		}

		closest := Point2D{X: a.X + t*abx, Y: a.Y + t*aby}

		d := Distance(p, closest)
		if d < bestDistance {
			bestDistance = d
			bestChainage = travelled + t*segLen
		}

		travelled += segLen
	}

	return bestChainage, bestDistance, true
}

// SliceLineString returns the portion of line between the chainages startM and
// endM, measured from the start of line. Both bounds are clamped to the line,
// and the endpoints are interpolated so the result covers exactly the requested
// span.
//
// ok is false when line has fewer than two vertices, any vertex is not finite,
// or the clamped span is empty.
func SliceLineString(line []Point2D, startM, endM float64) (slice []Point2D, ok bool) {
	total := LineStringLength(line)
	if len(line) < 2 || math.IsNaN(total) || math.IsNaN(startM) || math.IsNaN(endM) {
		return nil, false
	}

	from := math.Min(math.Max(startM, 0), total)
	to := math.Min(math.Max(endM, 0), total)

	if to <= from {
		return nil, false
	}

	out := []Point2D{PointAtChainage(line, from)}
	travelled := 0.0

	for i := range len(line) - 1 {
		segLen := Distance(line[i], line[i+1])
		vertexAt := travelled + segLen

		// Keep every original vertex strictly inside the span, so the slice
		// follows the same path as the source polyline.
		if vertexAt > from && vertexAt < to {
			out = append(out, line[i+1])
		}

		travelled = vertexAt
	}

	return append(out, PointAtChainage(line, to)), true
}

// PointAtChainage returns the point on line at the given distance from its
// start, clamped to the ends of the line.
func PointAtChainage(line []Point2D, chainageM float64) Point2D {
	if len(line) == 0 {
		return Point2D{}
	}

	if len(line) == 1 || chainageM <= 0 {
		return line[0]
	}

	travelled := 0.0

	for i := range len(line) - 1 {
		segLen := Distance(line[i], line[i+1])
		if segLen <= 0 {
			continue
		}

		if chainageM <= travelled+segLen {
			t := (chainageM - travelled) / segLen

			return Point2D{
				X: line[i].X + (line[i+1].X-line[i].X)*t,
				Y: line[i].Y + (line[i+1].Y-line[i].Y)*t,
			}
		}

		travelled += segLen
	}

	return line[len(line)-1]
}
