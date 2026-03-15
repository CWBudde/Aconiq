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
