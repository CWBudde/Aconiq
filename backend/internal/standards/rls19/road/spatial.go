package road

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// This file holds the spatial narrowing the propagation walk runs on before it
// touches an obstacle, and the scratch that narrowing reuses.
//
// Everything here is a *prune*, never an approximation. A prune may skip work
// only where the skipped work provably contributes exactly zero, which for
// these three is:
//
//   - A barrier whose bounding box the source→receiver ray does not pass
//     through cannot intersect that ray — the crossing point would have to
//     lie on both — so LineStringIntersectsSegment would have answered !ok
//     for it and geo.RayCrossings would have dropped it.
//   - A reflector wall further from both the source and the receiver than
//     RLS-19 Tabelle 8's height condition allows fails that condition, which
//     the reflection search already tests and rejects on.
//   - A wall outside the shadow the first reflector casts away from the image
//     source cannot hold the second bounce, because the second bounce lies on
//     the ray from the image source through the first bounce, extended.
//
// The order of what survives a prune matters as much as the set. Contributions
// are energy-summed in slice order and barrier crossings are sorted stably, so
// a candidate list in anything but ascending obstacle order would move results
// in the last bits without changing anything anyone could see. Every query
// here therefore answers ascending, and every filter preserves that.

// queryMarginM is how far outside the exact region a spatial prune still
// looks, in model units.
//
// Each prune is exact in exact arithmetic. The test it prunes against is not:
// geo.SegmentIntersection divides by a determinant, and a barrier's crossing
// point comes back parametrised along the ray rather than along the barrier,
// so an accepted crossing can land a rounding error off the geometry it
// belongs to. At UTM magnitudes that error is on the order of a micrometre.
// One millimetre is four orders of magnitude of headroom and is geometrically
// nothing: no acoustic result turns on a barrier being a millimetre wider.
const queryMarginM = 1e-3

// pathScratch is the per-walk buffer set: the index cursors and the slices the
// propagation walk would otherwise reallocate once per Teilstück and receiver.
//
// It is deliberately not held on the Scene. A Scene is immutable once prepared
// and can be shared between goroutines; this cannot, because a cursor stamps
// which items a query has already returned. One scratch per receiver walk.
//
// A nil *pathScratch is valid everywhere below and means "no index, no
// reuse" — the path a direct call to the exported ComputeShielding takes.
type pathScratch struct {
	barrierCursor *geo.BBoxGridCursor
	wallCursor    *geo.BBoxGridCursor

	barrierCandidates []int
	wallCandidates    []int

	rayCrossings     []geo.RayCrossing
	barrierCrossings []barrierCrossing
	reflectedPaths   []reflectedPath
}

// newScratch returns the scratch for one receiver walk over this scene.
func (s *Scene) newScratch() *pathScratch {
	return &pathScratch{
		barrierCursor:     s.barrierGrid.NewCursor(),
		wallCursor:        s.reflectors.grid.NewCursor(),
		barrierCandidates: nil,
		wallCandidates:    nil,
		rayCrossings:      nil,
		barrierCrossings:  nil,
		reflectedPaths:    nil,
	}
}

// segmentBBox is the bounding box of one two-point segment.
func segmentBBox(a, b geo.Point2D) geo.BBox {
	return geo.BBox{
		MinX: math.Min(a.X, b.X),
		MinY: math.Min(a.Y, b.Y),
		MaxX: math.Max(a.X, b.X),
		MaxY: math.Max(a.Y, b.Y),
	}
}

// appendAllIndices fills dst with 0..count-1, the candidate list that narrows
// nothing. It is what a caller without an index gets, so that the indexed and
// unindexed paths run the same code over the same order.
func appendAllIndices(dst []int, count int) []int {
	for i := range count {
		dst = append(dst, i)
	}

	return dst
}

// newBarrierGrid indexes the barrier polylines by their own bounding boxes.
func newBarrierGrid(barriers []Barrier) *geo.BBoxGrid {
	return buildBBoxGrid(len(barriers), func(i int) (geo.BBox, bool) {
		return geo.BBoxFromLineString(barriers[i].Geometry)
	})
}

// buildBBoxGrid indexes one box per item, or returns nil when any item has no
// usable box. Nil means "test everything", which is what the walk did before
// the index existed, so a malformed obstacle costs speed and never accuracy.
func buildBBoxGrid(count int, box func(int) (geo.BBox, bool)) *geo.BBoxGrid {
	if count == 0 {
		return nil
	}

	boxes := make([]geo.BBox, 0, count)

	for i := range count {
		b, ok := box(i)
		if !ok {
			return nil
		}

		boxes = append(boxes, b)
	}

	return geo.NewBBoxGrid(boxes)
}
