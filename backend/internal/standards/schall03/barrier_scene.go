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

// ComputeBarrierGeometryFromEdges builds a BarrierGeometry from the diffraction
// edges selected by the rubber band method.
//
// For 1 edge: single diffraction (IsDouble=false, E=0).
// For 2 edges: double diffraction (IsDouble=true, E=distance between edges).
// For >2 edges: uses the outermost two edges (the standard caps D_z at 25 dB
// for double diffraction; intermediate edges are subsumed by the hull).
//
// sourceHeightM and receiverHeightM are heights above ground [m].
// totalHorizDistM is the horizontal source→receiver distance [m].
func ComputeBarrierGeometryFromEdges(
	edges []DiffractionEdge,
	sourceHeightM, receiverHeightM, totalHorizDistM float64,
) BarrierGeometry {
	if len(edges) == 0 {
		return BarrierGeometry{}
	}

	// Direct 3D distance source→receiver.
	dh := receiverHeightM - sourceHeightM
	d := math.Sqrt(totalHorizDistM*totalHorizDistM + dh*dh)

	if len(edges) == 1 {
		return singleEdgeGeometry(edges[0], sourceHeightM, receiverHeightM, totalHorizDistM, d)
	}

	// Double diffraction: use first and last edge.
	first := edges[0]
	last := edges[len(edges)-1]

	return doubleEdgeGeometry(first, last, sourceHeightM, receiverHeightM, totalHorizDistM, d)
}

// singleEdgeGeometry computes BarrierGeometry for a single diffraction edge.
func singleEdgeGeometry(
	edge DiffractionEdge,
	sourceH, receiverH, totalHorizDist, directDist float64,
) BarrierGeometry {
	// 3D distance source→edge top.
	dhS := edge.HeightM - sourceH
	ds := math.Sqrt(edge.DistFromSource*edge.DistFromSource + dhS*dhS)

	// 3D distance edge top→receiver.
	horizDR := totalHorizDist - edge.DistFromSource
	dhR := receiverH - edge.HeightM
	dr := math.Sqrt(horizDR*horizDR + dhR*dhR)

	// Path difference z per Gl. 26 (non-parallel, no lateral offset for top diffraction).
	z := ds + dr - directDist

	return BarrierGeometry{
		Ds:             ds,
		Dr:             dr,
		D:              directDist,
		Z:              z,
		E:              0,
		Habs:           edge.Barrier.BaseHeightM,
		IsDouble:       false,
		TopDiffraction: true,
	}
}

// doubleEdgeGeometry computes BarrierGeometry for double diffraction using the
// first and last selected edges.
func doubleEdgeGeometry(
	first, last DiffractionEdge,
	sourceH, receiverH, totalHorizDist, directDist float64,
) BarrierGeometry {
	// 3D distance source→first edge top.
	dhS := first.HeightM - sourceH
	ds := math.Sqrt(first.DistFromSource*first.DistFromSource + dhS*dhS)

	// 3D distance last edge top→receiver.
	horizDR := totalHorizDist - last.DistFromSource
	dhR := receiverH - last.HeightM
	dr := math.Sqrt(horizDR*horizDR + dhR*dhR)

	// Distance between the two edges (barrier "thickness" e).
	horizE := last.DistFromSource - first.DistFromSource
	dhE := last.HeightM - first.HeightM
	e := math.Sqrt(horizE*horizE + dhE*dhE)

	// Path difference z per Gl. 26 (non-parallel top diffraction).
	z := ds + dr + e - directDist

	// Use the larger BaseHeightM for D_refl (conservative: less correction).
	habs := math.Max(first.Barrier.BaseHeightM, last.Barrier.BaseHeightM)

	// Check if the two barriers have parallel edges.
	isParallel := first.Barrier.IsParallel && last.Barrier.IsParallel

	if isParallel {
		// Gl. 25: z = sqrt((ds+dr+e)² + dPar²) - d.
		// For top diffraction between two parallel barriers, dPar = 0.
		z = pathDifferenceParallel(ds, dr, e, 0, directDist)
	}

	return BarrierGeometry{
		Ds:             ds,
		Dr:             dr,
		D:              directDist,
		Z:              z,
		E:              e,
		Habs:           habs,
		IsDouble:       true,
		TopDiffraction: true,
	}
}

// ComputeLateralDiffraction computes the barrier attenuation for lateral
// diffraction around the endpoints of a barrier segment.  It evaluates
// diffraction paths around both ends (A and B) and returns the one with
// the least attenuation (the dominant path).
//
// Lateral diffraction uses Gl. 18: A_bar = D_z ≥ 0 (no D_refl or A_gr
// subtraction).  The path difference z is computed per Gl. 26 as the detour
// source → barrier endpoint → receiver minus the direct distance.
//
// Returns (abar, true) if at least one lateral path exists, or (zero, false)
// if neither endpoint provides a valid lateral path.
func ComputeLateralDiffraction(
	source, receiver geo.Point2D,
	sourceHeightM, receiverHeightM float64,
	barrier BarrierSegment,
) (BeiblattSpectrum, bool) {
	dh := receiverHeightM - sourceHeightM
	directDist := math.Sqrt(
		geo.Distance(source, receiver)*geo.Distance(source, receiver) + dh*dh,
	)

	if directDist <= 0 {
		return BeiblattSpectrum{}, false
	}

	var bestAbar BeiblattSpectrum
	found := false

	for _, endpoint := range [2]geo.Point2D{barrier.A, barrier.B} {
		abar, ok := lateralPathAbar(source, receiver, endpoint, sourceHeightM, receiverHeightM, barrier.TopHeightM, directDist)
		if !ok {
			continue
		}

		if !found || energeticTotalSpectrum(abar) < energeticTotalSpectrum(bestAbar) {
			bestAbar = abar
			found = true
		}
	}

	return bestAbar, found
}

// lateralPathAbar computes A_bar for one lateral diffraction path around a
// barrier endpoint.
func lateralPathAbar(
	source, receiver, endpoint geo.Point2D,
	sourceH, receiverH, barrierTopH, directDist float64,
) (BeiblattSpectrum, bool) {
	// Horizontal distances.
	horizSE := geo.Distance(source, endpoint)
	horizER := geo.Distance(endpoint, receiver)

	// 3D distances: use barrier top height at the endpoint.
	dhS := barrierTopH - sourceH
	ds := math.Sqrt(horizSE*horizSE + dhS*dhS)

	dhR := receiverH - barrierTopH
	dr := math.Sqrt(horizER*horizER + dhR*dhR)

	// Path difference z per Gl. 26.
	z := ds + dr - directDist

	if z <= 0 {
		return BeiblattSpectrum{}, false // no screening effect
	}

	// Compute D_z per band using Gl. 21 with C₂=40 (Strecke), C₃=1 (single edge).
	// Lateral diffraction: A_bar = D_z ≥ 0 (Gl. 18, no D_refl, no A_gr).
	km := kmet(ds, dr, directDist, z)

	var abar BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		fm := BeiblattOctaveBandFrequencies[f]
		lam := wavelength(fm)
		dz := barrierDz(lam, 1.0, z, km)

		if dz > DzCapSingle {
			dz = DzCapSingle
		}

		abar[f] = math.Max(dz, 0)
	}

	return abar, true
}

// energeticTotalSpectrum returns the A-weighted energetic sum of a BeiblattSpectrum
// for comparison purposes (lower = less attenuation = dominant path).
func energeticTotalSpectrum(s BeiblattSpectrum) float64 {
	sum := 0.0

	for f := range NumBeiblattOctaveBands {
		sum += s[f]
	}

	return sum
}

// ComputePathBarrierAttenuation computes the barrier attenuation A_bar for a
// single source→receiver propagation path, considering all barriers in the scene.
// Returns a per-band BeiblattSpectrum of attenuation values (dB, ≥ 0).
// Returns a zero spectrum if no barriers obstruct the path.
func ComputePathBarrierAttenuation(
	source, receiver geo.Point2D,
	sourceHeightM, receiverHeightM float64,
	barriers []BarrierSegment,
	agrBandValues BeiblattSpectrum,
) BeiblattSpectrum {
	if len(barriers) == 0 {
		return BeiblattSpectrum{}
	}

	totalHorizDist := geo.Distance(source, receiver)
	if totalHorizDist <= 0 {
		return BeiblattSpectrum{}
	}

	// 1. Find all crossings in plan view.
	crossings := FindBarrierCrossings(source, receiver, barriers)
	if len(crossings) == 0 {
		return BeiblattSpectrum{}
	}

	// 2. Filter to only obstructing crossings.
	var obstructing []BarrierCrossing

	for _, c := range crossings {
		if IsObstructing(c, sourceHeightM, receiverHeightM, totalHorizDist) {
			obstructing = append(obstructing, c)
		}
	}

	if len(obstructing) == 0 {
		return BeiblattSpectrum{}
	}

	// 3. Select significant diffraction edges via rubber band.
	edges := SelectDiffractionEdges(sourceHeightM, receiverHeightM, totalHorizDist, obstructing)
	if len(edges) == 0 {
		return BeiblattSpectrum{}
	}

	// 4. Compute top-diffraction BarrierGeometry and A_bar.
	geom := ComputeBarrierGeometryFromEdges(edges, sourceHeightM, receiverHeightM, totalHorizDist)
	topAbar := ComputeAbar(geom, agrBandValues)

	// 5. Compute lateral diffraction for each obstructing barrier.
	// Use the minimum A_bar per band across top and all lateral paths.
	bestAbar := topAbar

	for _, c := range obstructing {
		latAbar, ok := ComputeLateralDiffraction(source, receiver, sourceHeightM, receiverHeightM, c.Barrier)
		if !ok {
			continue
		}

		// Per-band minimum.
		for f := range NumBeiblattOctaveBands {
			if latAbar[f] < bestAbar[f] {
				bestAbar[f] = latAbar[f]
			}
		}
	}

	return bestAbar
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
