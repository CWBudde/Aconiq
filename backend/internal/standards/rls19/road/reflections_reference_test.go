package road

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// This file keeps the reflection search as it stood before the receiver-side
// precomputation, as the reference the production search is held to. It is
// the source-side search: every Teilstück mirrored across every wall, and for
// second order the shadow of each first wall queried for the second.
//
// It is test-only on purpose. The production search may only ever be a faster
// way of producing exactly these paths, in exactly this order — the property
// tests in reflections_receiver_test.go compare the two to the bit — so the
// reference must stay as it was written and must not be "fixed" to agree.
//
// One known way the two can legitimately differ, documented in
// reflections_receiver.go under "The one way it can differ": this reference
// prunes second bounces with a source-side shadow whose fixed 1 mm margin can
// round wrongly when the source stands a few millimetres off the first wall's
// line and the second wall is far away. The production search can then return
// one second-order path more than this reference — a path the unchanged exact
// test accepts and this reference pruned in error. A mismatch of that shape
// (an extra path, never a missing or changed one) is that, not a defect. None
// has been seen.

// referencePaths is the old appendPaths.
func (f *reflectorField) referencePaths(
	dst []reflectedPath,
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	scratch *pathScratch,
) []reflectedPath {
	if len(f.walls) == 0 {
		return dst
	}

	dz := receiverZ - sourceZ

	dst = f.referenceFirstOrder(dst, source, receiver, sourceZ, dz)

	return f.referenceSecondOrder(dst, source, receiver, sourceZ, dz, scratch)
}

// referenceFirstOrder is the old appendFirstOrder.
func (f *reflectorField) referenceFirstOrder(
	dst []reflectedPath,
	source, receiver geo.Point2D,
	sourceZ, dz float64,
) []reflectedPath {
	paths := dst

	for i := range f.walls {
		w := &f.walls[i]
		if !w.inTabelle8Range(w.bbox.SquaredDistanceToPoint(source), w.bbox.SquaredDistanceToPoint(receiver)) {
			continue
		}

		img := mirrorPoint(source, w.a, w.b)
		p, _, ok := geo.LineStringIntersectsSegment([]geo.Point2D{w.a, w.b}, img, receiver)

		if !ok {
			continue
		}

		aR := math.Min(dist2D(source, p), dist2D(p, receiver))
		if w.heightM < 1.0 || w.heightM < 0.3*math.Sqrt(aR) {
			continue
		}

		planDist := dist2D(img, receiver)
		if planDist > 0 {
			t := dist2D(img, p) / planDist

			heightAtP := sourceZ + dz*t
			if w.heightM < heightAtP {
				continue
			}
		}

		slantDist := math.Sqrt(planDist*planDist + dz*dz)
		paths = append(paths, reflectedPath{
			planDistM:    planDist,
			slantDistM:   slantDist,
			lossDB:       w.loss,
			imagePoint:   img,
			reflectorIDs: [2]string{w.ownerID, ""},
			bounces:      1,
		})
	}

	return paths
}

// referenceSecondOrder is the old appendSecondOrder: the shadow of every first
// wall, cast from the image source, queried for the second.
func (f *reflectorField) referenceSecondOrder(
	dst []reflectedPath,
	source, receiver geo.Point2D,
	sourceZ, dz float64,
	scratch *pathScratch,
) []reflectedPath {
	paths := dst

	for i := range f.walls {
		w1 := &f.walls[i]
		if w1.maxRangeM < 0 {
			continue
		}

		img1 := mirrorPoint(source, w1.a, w1.b)

		shadow := geo.NewSegmentShadow(img1, w1.a, w1.b, queryMarginM)

		candidates, reachable := f.referenceSecondBounceCandidates(&shadow, scratch)
		if !reachable {
			continue
		}

		sourceGapSq := w1.bbox.SquaredDistanceToPoint(source)

		for _, j := range candidates {
			if i == j {
				continue
			}

			w2 := &f.walls[j]
			if !shadow.MayContainSegment(w2.a, w2.b) {
				continue
			}

			gapSq := w1.bbox.SquaredDistanceToBBox(w2.bbox)
			if !w1.inTabelle8Range(sourceGapSq, gapSq) ||
				!w2.inTabelle8Range(gapSq, w2.bbox.SquaredDistanceToPoint(receiver)) {
				continue
			}

			path, ok := secondOrderPath(w1, w2, img1, source, receiver, sourceZ, dz)
			if ok {
				paths = append(paths, path)
			}
		}
	}

	return paths
}

// referenceSecondBounceCandidates is the old secondBounceCandidates.
func (f *reflectorField) referenceSecondBounceCandidates(
	shadow *geo.SegmentShadow,
	scratch *pathScratch,
) ([]int, bool) {
	if scratch == nil || !scratch.wallCursor.Ready() {
		return f.all, true
	}

	scratch.wallCandidates = scratch.wallCursor.QueryShadow(shadow, queryMarginM, scratch.wallCandidates[:0])

	return scratch.wallCandidates, len(scratch.wallCandidates) > 0
}
