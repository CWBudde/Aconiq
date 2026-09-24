package geo

// MaskPointsInFootprints reports, per point, whether it lies strictly inside
// any footprint. Each footprint is one polygon in PointInPolygon's ring format
// (rings[0] exterior, rings[1:] holes); a MultiPolygon is passed as its parts.
//
// This is the one answer to "which receivers stand inside a building". The CLI
// calls it directly and the browser reaches it through the WASM kernel, so the
// two targets cannot disagree on a cell: there is no second implementation to
// drift. A point on a facade or a courtyard edge is not masked — see
// PointInPolygonInterior.
//
// Each point is answered independently, so the result does not depend on the
// order the footprints arrive in.
func MaskPointsInFootprints(points []Point2D, footprints [][][]Point2D) []bool {
	masked := make([]bool, len(points))
	if len(points) == 0 {
		return masked
	}

	polys, boxes := footprintBoxes(footprints)
	if len(polys) == 0 {
		return masked
	}

	// A nil grid (degenerate boxes) yields a cursor that is not Ready, and
	// the loop then tests every footprint: slower, same answer.
	cursor := NewBBoxGrid(boxes).NewCursor()

	var candidates []int

	for i, p := range points {
		if !p.IsFinite() {
			continue
		}

		if cursor.Ready() {
			candidates = cursor.Query(BBox{MinX: p.X, MinY: p.Y, MaxX: p.X, MaxY: p.Y}, candidates[:0])
		} else {
			candidates = allIndices(len(polys), candidates[:0])
		}

		for _, k := range candidates {
			if !boxes[k].ContainsPoint(p) {
				continue
			}

			if PointInPolygonInterior(p, polys[k]) {
				masked[i] = true

				break
			}
		}
	}

	return masked
}

// footprintBoxes drops the footprints that cannot contain anything — no
// closed exterior, or a non-finite vertex — and returns the rest beside
// their exterior's bounding box.
func footprintBoxes(footprints [][][]Point2D) ([][][]Point2D, []BBox) {
	polys := make([][][]Point2D, 0, len(footprints))
	boxes := make([]BBox, 0, len(footprints))

	for _, rings := range footprints {
		if len(rings) == 0 || len(rings[0]) < 4 {
			continue
		}

		box, ok := ringBBox(rings[0])
		if !ok {
			continue
		}

		polys = append(polys, rings)
		boxes = append(boxes, box)
	}

	return polys, boxes
}

func ringBBox(ring []Point2D) (BBox, bool) {
	box := BBox{MinX: ring[0].X, MinY: ring[0].Y, MaxX: ring[0].X, MaxY: ring[0].Y}

	for _, p := range ring {
		if !p.IsFinite() {
			return BBox{}, false
		}

		box = box.ExpandToIncludePoint(p)
	}

	return box, true
}

func allIndices(n int, dst []int) []int {
	for i := range n {
		dst = append(dst, i)
	}

	return dst
}
