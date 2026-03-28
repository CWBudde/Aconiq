package road

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// barrierCrossing records where a barrier intersects the source→receiver
// line in plan view.
type barrierCrossing struct {
	point          geo.Point2D
	distFromSource float64 // 2D plan distance from source
	barrier        *Barrier
}

// findBarrierCrossings returns all barriers that intersect the line from
// source to receiver in plan view, sorted by distance from source.
func findBarrierCrossings(source, receiver geo.Point2D, barriers []Barrier) []barrierCrossing {
	var crossings []barrierCrossing

	for i := range barriers {
		b := &barriers[i]

		crossPt, _, intersects := geo.LineStringIntersectsSegment(b.Geometry, source, receiver)
		if !intersects {
			continue
		}

		d := dist2D(source, crossPt)
		if d < 1e-6 || dist2D(crossPt, receiver) < 1e-6 {
			continue // at endpoint, not truly between
		}

		crossings = append(crossings, barrierCrossing{
			point:          crossPt,
			distFromSource: d,
			barrier:        b,
		})
	}

	// Sort by distance from source (insertion sort — n is small).
	for i := 1; i < len(crossings); i++ {
		key := crossings[i]

		j := i - 1
		for j >= 0 && crossings[j].distFromSource > key.distFromSource {
			crossings[j+1] = crossings[j]
			j--
		}

		crossings[j+1] = key
	}

	return crossings
}

// diffractionEdge describes one significant diffraction edge selected by the
// Gummibandmethode (rubber band method) per RLS-19 §3.5.5.
type diffractionEdge struct {
	distFromSource float64  // 2D plan distance from source [m]
	heightM        float64  // barrier top height [m]
	barrier        *Barrier // source barrier
}

// hullPoint is a point in the vertical cross-section for the upper convex hull.
type hullPoint struct {
	dist        float64
	height      float64
	crossingIdx int // index into crossings; -1 for source/receiver
}

// selectDiffractionEdges implements the Gummibandmethode to select significant
// diffraction edges from barrier crossings.
//
// The method projects barrier tops into the vertical source→receiver
// cross-section and computes the upper convex hull. Hull vertices (excluding
// source and receiver) that lie above the line of sight are the significant
// edges, returned in order from source to receiver.
//
// crossings must be sorted by distFromSource and pre-filtered to only include
// barriers that actually intersect the source→receiver line.
func selectDiffractionEdges(
	sourceHeightM, receiverHeightM, totalDistM float64,
	crossings []barrierCrossing,
) []diffractionEdge {
	if len(crossings) == 0 {
		return nil
	}

	// Filter to obstructing crossings (barrier top above line of sight).
	var obstructing []barrierCrossing
	for _, c := range crossings {
		frac := c.distFromSource / totalDistM
		losHeight := sourceHeightM + frac*(receiverHeightM-sourceHeightM)
		if c.barrier.HeightM > losHeight {
			obstructing = append(obstructing, c)
		}
	}

	if len(obstructing) == 0 {
		return nil
	}

	// For a single obstructing barrier, skip the hull computation.
	if len(obstructing) == 1 {
		c := obstructing[0]
		return []diffractionEdge{{
			distFromSource: c.distFromSource,
			heightM:        c.barrier.HeightM,
			barrier:        c.barrier,
		}}
	}

	// Build points for upper convex hull: source, obstructing tops, receiver.
	points := make([]hullPoint, 0, len(obstructing)+2)
	points = append(points, hullPoint{dist: 0, height: sourceHeightM, crossingIdx: -1})
	for i, c := range obstructing {
		points = append(points, hullPoint{
			dist:        c.distFromSource,
			height:      c.barrier.HeightM,
			crossingIdx: i,
		})
	}
	points = append(points, hullPoint{dist: totalDistM, height: receiverHeightM, crossingIdx: -1})

	// Upper convex hull (Andrew's monotone chain, upper hull only).
	hull := upperConvexHull(points)

	// Extract edges: hull vertices that are not source or receiver.
	var edges []diffractionEdge
	for _, hp := range hull {
		if hp.crossingIdx < 0 {
			continue
		}
		c := obstructing[hp.crossingIdx]
		edges = append(edges, diffractionEdge{
			distFromSource: c.distFromSource,
			heightM:        c.barrier.HeightM,
			barrier:        c.barrier,
		})
	}

	return edges
}

// upperConvexHull computes the upper convex hull of points sorted by dist.
// Points on or below the line from their neighbours are removed.
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
			// Cross product: if >= 0, b is on or below line a→c → remove.
			cross := (b.dist-a.dist)*(c.height-a.height) -
				(b.height-a.height)*(c.dist-a.dist)
			if cross < 0 {
				break
			}
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, points[i])
	}

	return hull
}

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

// ComputeShielding determines barrier shielding for the path between source
// and receiver per RLS-19 §3.5.5, supporting both single-edge and multi-edge
// diffraction.
//
// The method finds all barrier crossings along the source→receiver line,
// selects significant diffraction edges via the Gummibandmethode (upper convex
// hull in the vertical cross-section, per Probst 2010), and computes the
// insertion loss using Eqs. 15-17 with the C term for multi-edge paths.
//
// For a single barrier, this reduces to the standard z = A + B - s formula
// (C=0), producing identical results to the previous single-edge implementation.
//
// Parameters:
//   - source: source point (2D plan position)
//   - sourceHeightM: source height above ground [m]
//   - receiver: receiver point (2D plan position)
//   - receiverHeightM: receiver height above ground [m]
//   - barriers: list of barriers to check
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

	// Step 1: find all barrier crossings sorted by distance from source.
	crossings := findBarrierCrossings(source, receiver, barriers)
	if len(crossings) == 0 {
		return ShieldingResult{}
	}

	// Step 2-3: select significant diffraction edges via Gummibandmethode.
	edges := selectDiffractionEdges(sourceHeightM, receiverHeightM, directDist, crossings)
	if len(edges) == 0 {
		return ShieldingResult{}
	}

	// Step 4-6: compute z and D_z via Eqs. 15-17 (with C term for multi-edge).
	z, loss := computeMultiEdgeLoss(edges, sourceHeightM, receiverHeightM, directDist)
	if z <= 0 || loss <= 0 {
		return ShieldingResult{}
	}

	// Report the first barrier as the effective one (it defines leg A).
	return ShieldingResult{
		Shielded:       true,
		InsertionLoss:  loss,
		PathDifference: z,
		BarrierID:      edges[0].barrier.ID,
	}
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

// computeMultiEdgeLoss computes the barrier insertion loss D_z for one or more
// diffraction edges per RLS-19 §3.5.5 Eqs. 15-17.
//
// For a single edge (len(edges)==1), this produces identical results to the
// existing computeDiffraction + rls19BarrierLoss path (C=0).
//
// For multiple edges, the C term (inter-edge path length) is included in z,
// and K_w is modified per the standard: C is added to whichever of A or B
// is larger.
//
// Parameters:
//   - edges: significant diffraction edges, sorted by distFromSource
//   - sourceHeightM: source height above ground [m]
//   - receiverHeightM: receiver height above ground [m]
//   - totalDistM: 2D plan distance source → receiver [m]
//
// Returns (z, D_z). z <= 0 means no shielding; D_z is 0 in that case.
func computeMultiEdgeLoss(
	edges []diffractionEdge,
	sourceHeightM, receiverHeightM, totalDistM float64,
) (float64, float64) {
	if len(edges) == 0 {
		return 0, 0
	}

	first := edges[0]
	last := edges[len(edges)-1]

	// A: 3D distance source → first edge.
	dSB := first.distFromSource
	dhA := first.heightM - sourceHeightM
	A := math.Sqrt(dSB*dSB + dhA*dhA)

	// B: 3D distance last edge → receiver.
	dBR := totalDistM - last.distFromSource
	dhB := receiverHeightM - last.heightM
	B := math.Sqrt(dBR*dBR + dhB*dhB)

	// C: sum of 3D distances between consecutive edges.
	C := 0.0
	for i := 1; i < len(edges); i++ {
		dH := edges[i].distFromSource - edges[i-1].distFromSource
		dV := edges[i].heightM - edges[i-1].heightM
		C += math.Sqrt(dH*dH + dV*dV)
	}

	// s: 3D direct distance source → receiver.
	dh := receiverHeightM - sourceHeightM
	s := math.Sqrt(totalDistM*totalDistM + dh*dh)

	// Eq. 16: z = A + B + C - s.
	z := A + B + C - s
	if z <= 0 {
		return z, 0
	}

	// Eq. 17: K_w with multi-diffraction modification.
	// C is added to the larger of A or B.
	Akw, Bkw := A, B
	if A >= B {
		Akw = A + C
	} else {
		Bkw = B + C
	}
	kw := math.Exp(-math.Sqrt(Akw*Bkw*s/(2*z)) / 2000.0)

	// Eq. 15: D_z = 10·lg(3 + 80·z·K_w).
	dz := 10 * math.Log10(3+80*z*kw)

	return z, dz
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
