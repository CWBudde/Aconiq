package schall03

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// BarrierSegment describes one noise barrier as a 2D line segment with
// height, thickness, and absorption properties.  This is the scene-level
// input type; the existing BarrierGeometry struct holds the pre-computed
// per-path diffraction geometry derived from BarrierSegment during
// propagation.
type BarrierSegment struct {
	// A is the first endpoint of the barrier in plan view.
	A geo.Point2D
	// B is the second endpoint of the barrier in plan view.
	B geo.Point2D
	// TopHeightM is the barrier top height above ground [m].
	TopHeightM float64
	// BaseHeightM is the height of the absorbing base (Sockel) above rail
	// level [m].  Used for D_refl correction (Gl. 20).  Set to 0 for a
	// fully reflective barrier.
	BaseHeightM float64
	// ThicknessM is the barrier thickness (distance between two diffraction
	// edges) [m].  0 for a thin single-edge barrier; >0 for a wide barrier
	// with double diffraction (Gl. 22, C₃ factor).
	ThicknessM float64
	// IsParallel indicates whether the barrier's two diffraction edges are
	// parallel (for Gl. 25 path difference).  Only relevant when
	// ThicknessM > 0.
	IsParallel bool
}

// Validate checks the barrier segment for geometric and physical validity.
func (b BarrierSegment) Validate() error {
	if !b.A.IsFinite() || !b.B.IsFinite() {
		return errors.New("BarrierSegment: endpoints must be finite")
	}

	if geo.Distance(b.A, b.B) < 1e-9 {
		return errors.New("BarrierSegment: barrier has zero length")
	}

	if math.IsNaN(b.TopHeightM) || math.IsInf(b.TopHeightM, 0) || b.TopHeightM <= 0 {
		return errors.New("BarrierSegment: TopHeightM must be finite and > 0")
	}

	if math.IsNaN(b.BaseHeightM) || math.IsInf(b.BaseHeightM, 0) || b.BaseHeightM < 0 {
		return errors.New("BarrierSegment: BaseHeightM must be finite and >= 0")
	}

	if b.BaseHeightM >= b.TopHeightM {
		return errors.New("BarrierSegment: BaseHeightM must be less than TopHeightM")
	}

	if math.IsNaN(b.ThicknessM) || math.IsInf(b.ThicknessM, 0) || b.ThicknessM < 0 {
		return errors.New("BarrierSegment: ThicknessM must be finite and >= 0")
	}

	return nil
}

// Length returns the barrier segment length in metres.
func (b BarrierSegment) Length() float64 {
	return geo.Distance(b.A, b.B)
}

// BarrierCrossing records where a source→receiver ray crosses a barrier
// segment in plan view.
type BarrierCrossing struct {
	// Point is the intersection point in plan view.
	Point geo.Point2D
	// BarrierIdx is the index into the barriers slice.
	BarrierIdx int
	// DistFromSource is the 2D distance from the source to the crossing point.
	DistFromSource float64
	// Barrier is a reference to the crossed barrier.
	Barrier BarrierSegment
}

// FindBarrierCrossings returns all barrier segments that the line from source
// to receiver crosses in plan view, sorted by distance from source (nearest
// first).
func FindBarrierCrossings(source, receiver geo.Point2D, barriers []BarrierSegment) []BarrierCrossing {
	var crossings []BarrierCrossing

	for i, b := range barriers {
		pt, _, ok := geo.SegmentIntersection(source, receiver, b.A, b.B)
		if !ok {
			continue
		}

		crossings = append(crossings, BarrierCrossing{
			Point:          pt,
			BarrierIdx:     i,
			DistFromSource: geo.Distance(source, pt),
			Barrier:        b,
		})
	}

	// Sort by distance from source (insertion sort — n is small).
	for i := 1; i < len(crossings); i++ {
		key := crossings[i]
		j := i - 1

		for j >= 0 && crossings[j].DistFromSource > key.DistFromSource {
			crossings[j+1] = crossings[j]
			j--
		}

		crossings[j+1] = key
	}

	return crossings
}

// hullPoint is a point in the vertical source→receiver cross-section used by
// the upper convex hull computation.
type hullPoint struct {
	dist        float64 // horizontal distance from source [m]
	height      float64 // height above ground [m]
	crossingIdx int     // index into crossings slice; -1 for source/receiver
}

// DiffractionEdge describes one significant diffraction edge selected by the
// rubber band method (Gummibandmethode).
type DiffractionEdge struct {
	// Point is the plan-view position of the edge.
	Point geo.Point2D
	// HeightM is the barrier top height at this edge [m].
	HeightM float64
	// DistFromSource is the 2D distance from the source [m].
	DistFromSource float64
	// BarrierIdx is the index into the original barriers slice.
	BarrierIdx int
	// Barrier is a reference to the barrier segment.
	Barrier BarrierSegment
}

// SelectDiffractionEdges implements the Gummibandmethode (rubber band method)
// to select the significant diffraction edges from a set of obstructing barrier
// crossings.
//
// The method projects barrier tops onto the vertical source→receiver
// cross-section plane and computes the upper convex hull.  Hull vertices
// (excluding source and receiver) are the significant edges, returned in order
// from source to receiver.
//
// crossings must be sorted by DistFromSource (as returned by
// FindBarrierCrossings) and pre-filtered to only include obstructing crossings.
func SelectDiffractionEdges(
	sourceHeightM, receiverHeightM, totalDistM float64,
	crossings []BarrierCrossing,
) []DiffractionEdge {
	if len(crossings) == 0 {
		return nil
	}

	// Build points in the vertical section: (distance, height).
	// Include source and receiver as bookends.
	points := make([]hullPoint, 0, len(crossings)+2)
	points = append(points, hullPoint{dist: 0, height: sourceHeightM, crossingIdx: -1})

	for i, c := range crossings {
		points = append(points, hullPoint{
			dist:        c.DistFromSource,
			height:      c.Barrier.TopHeightM,
			crossingIdx: i,
		})
	}

	points = append(points, hullPoint{dist: totalDistM, height: receiverHeightM, crossingIdx: -1})

	// Compute upper convex hull using Andrew's monotone chain (upper hull only).
	// Points are already sorted by dist (crossings are sorted, source/receiver
	// are at the ends).
	hull := upperConvexHull(points)

	// Extract edges: hull vertices that are not source or receiver.
	var edges []DiffractionEdge

	for _, hp := range hull {
		if hp.crossingIdx < 0 {
			continue // source or receiver
		}

		c := crossings[hp.crossingIdx]
		edges = append(edges, DiffractionEdge{
			Point:          c.Point,
			HeightM:        c.Barrier.TopHeightM,
			DistFromSource: c.DistFromSource,
			BarrierIdx:     c.BarrierIdx,
			Barrier:        c.Barrier,
		})
	}

	return edges
}

// upperConvexHull computes the upper convex hull of points sorted by dist.
//
// For the upper hull (keeping points that protrude above the line between
// their neighbours): we remove the last hull point when the cross product
// (a→b) × (a→c) is ≥ 0, meaning b lies on or below the line from a to c.
func upperConvexHull(points []hullPoint) []hullPoint {
	n := len(points)
	if n <= 2 {
		return points
	}

	hull := make([]hullPoint, 0, n)

	for i := range n {
		for len(hull) >= 2 {
			a := hull[len(hull)-2]
			b := hull[len(hull)-1]
			c := points[i]

			// Cross product of (a→b) × (a→c).
			// If cross ≥ 0, point b is on or below line a→c → remove b.
			// If cross < 0, point b is above line a→c → keep b.
			cross := (b.dist-a.dist)*(c.height-a.height) - (b.height-a.height)*(c.dist-a.dist)

			if cross < 0 {
				break // b protrudes above a→c, keep it
			}

			hull = hull[:len(hull)-1]
		}

		hull = append(hull, points[i])
	}

	return hull
}

// IsObstructing reports whether a barrier crossing actually obstructs the
// line of sight between source and receiver.  The barrier obstructs when its
// top height exceeds the line-of-sight height at the crossing point.
//
// The line-of-sight height is linearly interpolated between sourceHeightM and
// receiverHeightM based on the crossing's fractional position along the path.
func IsObstructing(crossing BarrierCrossing, sourceHeightM, receiverHeightM, totalDistM float64) bool {
	if totalDistM <= 0 {
		return false
	}

	frac := crossing.DistFromSource / totalDistM
	losHeight := sourceHeightM + frac*(receiverHeightM-sourceHeightM)

	return crossing.Barrier.TopHeightM > losHeight
}
