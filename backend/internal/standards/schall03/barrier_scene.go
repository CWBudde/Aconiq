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
