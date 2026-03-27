package road

import (
	"errors"
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
		return errors.New("barrier id is required")
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

		// Compute single-edge diffraction geometry and insertion loss (RLS-19 Eq. 15/17).
		diff := computeDiffraction(
			dSourceBarrier, sourceHeightM,
			dBarrierReceiver, receiverHeightM,
			b.HeightM,
		)

		if diff.Z <= 0 {
			// Barrier top is below the line of sight — no shielding.
			continue
		}

		loss := rls19BarrierLoss(diff)

		if loss > best.InsertionLoss {
			best = ShieldingResult{
				Shielded:       true,
				InsertionLoss:  loss,
				PathDifference: diff.Z,
				BarrierID:      b.ID,
			}
		}
	}

	return best
}

// diffractionGeometry holds the 3D path lengths for single-edge diffraction.
// All distances are in metres.
type diffractionGeometry struct {
	Z float64 // path difference: A + B − s  (> 0 means barrier is above line-of-sight)
	A float64 // source → edge top (3D distance)
	B float64 // edge top → receiver (3D distance)
	S float64 // source → receiver direct (3D distance)
}

// computeDiffraction returns the single-edge diffraction geometry for the
// vertical cross-section through source, barrier, and receiver.
//
// Parameters are the plan-view horizontal distances and heights:
//
//	dSB  – plan distance source → barrier crossing
//	hS   – source height (absolute Z or relative, but consistent with hR, hEdge)
//	dBR  – plan distance barrier crossing → receiver
//	hR   – receiver height
//	hEdge – barrier/edge height
//
// Z > 0 indicates the edge is above the source-receiver line-of-sight (shielding).
// Z ≤ 0 means no shielding.
func computeDiffraction(dSB, hS, dBR, hR, hEdge float64) diffractionGeometry {
	dTotal := dSB + dBR
	A := math.Sqrt(dSB*dSB + (hEdge-hS)*(hEdge-hS))
	B := math.Sqrt(dBR*dBR + (hEdge-hR)*(hEdge-hR))
	S := math.Sqrt(dTotal*dTotal + (hR-hS)*(hR-hS))
	Z := A + B - S

	// Negate if edge is below line-of-sight (no shielding).
	hLOS := hS + (hR-hS)*dSB/dTotal
	if hEdge < hLOS {
		Z = -Z
	}

	return diffractionGeometry{Z: Z, A: A, B: B, S: S}
}

// rls19BarrierLoss computes the barrier insertion loss D_z per RLS-19 Eqs. 15/17.
//
//	D_z  = 10·lg(3 + 80·z·K_w)                           (Eq. 15)
//	K_w  = exp(−1/2000 · sqrt(A·B·s / (2·z)))             (Eq. 17)
//
// K_w is a frequency-distance weighting factor that accounts for the broadband
// nature of A-weighted road traffic noise. Returns 0 when z ≤ 0.
func rls19BarrierLoss(d diffractionGeometry) float64 {
	if d.Z <= 0 {
		return 0
	}

	// Eq. 17: K_w frequency-distance weighting.
	kw := math.Exp(-math.Sqrt(d.A*d.B*d.S/(2*d.Z)) / 2000.0)

	// Eq. 15: barrier insertion loss.
	return 10 * math.Log10(3+80*d.Z*kw)
}
