package road

import (
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// Barrier represents a noise barrier or shielding obstacle (wall, berm, building edge).
// In plan view it is a polyline; the height is uniform along its length.
type Barrier struct {
	ID       string        `json:"id"`
	Geometry []geo.Point2D `json:"geometry"` // polyline in plan view
	HeightM  float64       `json:"height_m"` // top-of-barrier height above ground
}

// Validate checks a barrier definition.
func (b Barrier) Validate() error {
	if b.ID == "" {
		return fmt.Errorf("barrier id is required")
	}

	if len(b.Geometry) < 2 {
		return fmt.Errorf("barrier %q geometry must contain at least 2 points", b.ID)
	}

	for i, pt := range b.Geometry {
		if !pt.IsFinite() {
			return fmt.Errorf("barrier %q geometry point[%d] is not finite", b.ID, i)
		}
	}

	if !isFinite(b.HeightM) || b.HeightM <= 0 {
		return fmt.Errorf("barrier %q height_m must be finite and > 0", b.ID)
	}

	return nil
}

// ShieldingResult holds the result of a shielding calculation for one
// source-receiver path.
type ShieldingResult struct {
	Shielded       bool    // true if a barrier blocks the direct path
	InsertionLoss  float64 // A_bar: barrier insertion loss [dB], >= 0
	PathDifference float64 // delta: path length difference [m]
	BarrierID      string  // ID of the effective barrier (empty if not shielded)
}

// ComputeShielding determines if any barrier shields the direct path between
// source and receiver, and computes the insertion loss using the
// Maekawa/Kurze-Anderson approximation.
//
// The method works in a vertical cross-section along the direct source-receiver
// line. For each barrier that intersects this line in plan view, we compute
// the path length difference (source→barrier-top→receiver vs source→receiver)
// and derive the insertion loss. The barrier producing the largest insertion
// loss is the effective one.
//
// Parameters:
//   - source: source point (2D plan position)
//   - sourceHeightM: source height above ground [m]
//   - receiver: receiver point (2D plan position)
//   - receiverHeightM: receiver height above ground [m]
//   - barriers: list of barriers to check
//
// The maximum insertion loss is capped at 20 dB (practical limit for
// single-edge diffraction).
func ComputeShielding(
	source geo.Point2D, sourceHeightM float64,
	receiver geo.Point2D, receiverHeightM float64,
	barriers []Barrier,
) ShieldingResult {
	if len(barriers) == 0 {
		return ShieldingResult{}
	}

	directDist := dist2D(source, receiver)
	if directDist < 1e-6 {
		return ShieldingResult{}
	}

	best := ShieldingResult{}

	for i := range barriers {
		b := &barriers[i]

		// Find where the barrier intersects the source-receiver line in plan view.
		crossPt, _, intersects := geo.LineStringIntersectsSegment(b.Geometry, source, receiver)
		if !intersects {
			continue
		}

		// Compute distances in plan view.
		dSourceBarrier := dist2D(source, crossPt)
		dBarrierReceiver := dist2D(crossPt, receiver)

		// Skip if intersection is at endpoints (barrier not truly between).
		if dSourceBarrier < 1e-6 || dBarrierReceiver < 1e-6 {
			continue
		}

		// Compute the path length difference in the vertical cross-section.
		delta := pathDifference(
			dSourceBarrier, sourceHeightM,
			dBarrierReceiver, receiverHeightM,
			b.HeightM,
		)

		if delta <= 0 {
			// Barrier top is below the line of sight — no shielding.
			continue
		}

		loss := maekawaInsertionLoss(delta)

		if loss > best.InsertionLoss {
			best = ShieldingResult{
				Shielded:       true,
				InsertionLoss:  loss,
				PathDifference: delta,
				BarrierID:      b.ID,
			}
		}
	}

	return best
}

// pathDifference computes the path length difference delta for single-edge
// diffraction in a vertical cross-section.
//
// The geometry in the vertical plane:
//
//	S (source) at height hS, horizontal distance dSB from barrier
//	B (barrier top) at height hB
//	R (receiver) at height hR, horizontal distance dBR from barrier
//
// delta = sqrt(dSB^2 + (hB-hS)^2) + sqrt(dBR^2 + (hB-hR)^2) - sqrt((dSB+dBR)^2 + (hR-hS)^2)
//
// If delta > 0, the barrier is above the line of sight and causes diffraction.
// If delta <= 0, the barrier is below the line of sight (no shielding).
func pathDifference(dSB, hS, dBR, hR, hB float64) float64 {
	// Path over barrier: S → barrier top → R.
	pathSB := math.Sqrt(dSB*dSB + (hB-hS)*(hB-hS))
	pathBR := math.Sqrt(dBR*dBR + (hB-hR)*(hB-hR))

	// Direct path: S → R.
	dTotal := dSB + dBR
	pathDirect := math.Sqrt(dTotal*dTotal + (hR-hS)*(hR-hS))

	return pathSB + pathBR - pathDirect
}

// maekawaInsertionLoss computes the barrier insertion loss using the
// Kurze-Anderson approximation of the Maekawa curve.
//
// For a path length difference delta > 0:
//
//	N = 2 * delta / lambda  (Fresnel number, using lambda = 0.34 m for ~1 kHz)
//	A_bar = 10 * lg(3 + 20*N)  (Kurze-Anderson, capped at 20 dB)
//
// The wavelength of ~0.34 m corresponds to approximately 1000 Hz, which is
// representative for A-weighted road traffic noise.
const referenceWavelengthM = 0.34 // ~1000 Hz

func maekawaInsertionLoss(delta float64) float64 {
	if delta <= 0 {
		return 0
	}

	fresnelN := 2 * delta / referenceWavelengthM
	loss := 10 * math.Log10(3+20*fresnelN)

	// Practical cap for single-edge diffraction.
	if loss > 20 {
		loss = 20
	}

	return loss
}
