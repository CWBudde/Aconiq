package geo

import "slices"

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

// UpperConvexHull returns the upper convex hull of points already sorted by
// Dist, by Andrew's monotone chain run over the upper side only.
//
// A point is dropped when it lies on or below the line between its
// neighbours, which is exactly the rubber band laid over the cross-section:
// what survives is what the band rests on.
func UpperConvexHull(points []HullPoint) []HullPoint {
	n := len(points)
	if n <= 2 {
		return points
	}

	hull := make([]HullPoint, 0, n)

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

	return hull
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
	if len(candidates) == 0 {
		return nil
	}

	// The source and receiver bookend the section so the hull has something to
	// rest on at both ends; they carry index -1 and are dropped again below.
	points := make([]HullPoint, 0, len(candidates)+2)
	points = append(points, HullPoint{Dist: 0, Height: sourceHeightM, Index: -1})

	for i, candidate := range candidates {
		points = append(points, HullPoint{
			Dist:   candidate.DistFromSource,
			Height: candidate.TopHeightM,
			Index:  i,
		})
	}

	points = append(points, HullPoint{Dist: totalDistM, Height: receiverHeightM, Index: -1})

	var edges []int

	for _, point := range UpperConvexHull(points) {
		if point.Index < 0 {
			continue
		}

		edges = append(edges, point.Index)
	}

	return edges
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

	for i, obstacle := range obstacles {
		point, segment, ok := LineStringIntersectsSegment(polyline(obstacle), source, receiver)
		if !ok {
			continue
		}

		distFromSource := Distance(source, point)
		if endpointToleranceM > 0 &&
			(distFromSource < endpointToleranceM || Distance(point, receiver) < endpointToleranceM) {
			continue
		}

		crossings = append(crossings, RayCrossing{
			Point:          point,
			DistFromSource: distFromSource,
			ObstacleIndex:  i,
			SegmentIndex:   segment,
		})
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

	return crossings
}
