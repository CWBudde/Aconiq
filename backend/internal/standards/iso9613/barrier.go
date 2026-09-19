package iso9613

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// BarrierGeometry holds pre-computed diffraction path geometry.
type BarrierGeometry struct {
	Dss float64 // distance from source to first diffraction edge (m)
	Dsr float64 // distance from last diffraction edge to receiver (m)
	E   float64 // distance between first and last diffraction edge, 0 for single (m)
	A   float64 // component distance parallel to barrier edge (m)
	D   float64 // direct source-to-receiver distance (m)

	// LineOfSightClear reports that the straight line from the source to the
	// receiver passes above the top edge of the barrier, so the obstacle does
	// not screen this path. ISO 9613-2:1996, 7.4: "If the line of sight
	// between the source S and receiver R passes above the top edge of the
	// barrier, z is given a negative sign."
	//
	// The distances alone cannot express this: for any real diffraction
	// geometry the diffracted path is at least as long as the direct one, so
	// Eq. 16/17 always yield z >= 0. The flag carries the sign that the
	// standard assigns from the elevation view.
	LineOfSightClear bool
}

// IsDouble returns true if this represents double diffraction (e > 0).
func (g BarrierGeometry) IsDouble() bool {
	return g.E > 0
}

// Validate rejects barrier geometries that cannot describe a real propagation
// path. The diffracted path length of Eq. 16/17 is never shorter than the
// direct source-receiver distance d; a geometry that violates this would
// produce a spurious negative z indistinguishable from the signed
// line-of-sight case of 7.4, and silently switch off the screening.
func (g BarrierGeometry) Validate() error {
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"dss", g.Dss},
		{"dsr", g.Dsr},
		{"e", g.E},
		{"a", g.A},
		{"d", g.D},
	} {
		if math.IsNaN(field.value) || math.IsInf(field.value, 0) || field.value < 0 {
			return fmt.Errorf("%s must be finite and >= 0", field.name)
		}
	}

	if g.D <= 0 {
		return errors.New("d must be > 0")
	}

	if diffractedPathLength(g) < g.D {
		return fmt.Errorf("diffracted path length %.6g m is shorter than the direct distance d = %.6g m", diffractedPathLength(g), g.D)
	}

	return nil
}

// diffractedPathLength computes the diffracted path length of Eq. 16 (single)
// or Eq. 17 (double).
func diffractedPathLength(g BarrierGeometry) float64 {
	return math.Hypot(g.Dss+g.Dsr+g.E, g.A)
}

// pathDifference computes z from Eq. 16 (single) or Eq. 17 (double), signed
// negative when the line of sight clears the top edge of the barrier (7.4).
func pathDifference(g BarrierGeometry) float64 {
	z := diffractedPathLength(g) - g.D
	if g.LineOfSightClear {
		return -z
	}

	return z
}

// c3Factor computes C_3 from Eq. 15.
// For single diffraction (e=0), C_3 = 1.
// For double diffraction, C_3 = [1+(5λ/e)²] / [(1/3)+(5λ/e)²].
func c3Factor(e, freqHz float64) float64 {
	if e <= 0 {
		return 1
	}

	lambda := Wavelength(freqHz)
	ratio := 5 * lambda / e
	r2 := ratio * ratio

	return (1 + r2) / (1.0/3.0 + r2)
}

// kMet computes K_met from Eq. 18.
func kMet(g BarrierGeometry, z float64) float64 {
	if z <= 0 {
		return 1
	}

	return math.Exp(-(1.0 / 2000.0) * math.Sqrt(g.Dss*g.Dsr*g.D/(2*z)))
}

// BarrierDz computes the barrier attenuation D_z (Eq. 14) for one octave band.
// c2 is 20 when ground reflections are included, 40 when handled by image sources.
func BarrierDz(g BarrierGeometry, z, freqHz, c2 float64) float64 {
	if z <= 0 {
		return 0
	}

	lambda := Wavelength(freqHz)
	c3 := c3Factor(g.E, freqHz)
	km := kMet(g, z)

	dz := 10 * math.Log10(3+(c2/lambda)*c3*z*km)

	maxDz := 20.0
	if g.IsDouble() {
		maxDz = 25.0
	}

	if dz > maxDz {
		return maxDz
	}

	return dz
}

// BarrierAttenuationBands computes A_bar per octave band (Eq. 12).
// groundAtten is A_gr for the unscreened path (subtracted per Eq. 12).
// Returns zero bands if geometry is nil (no barrier).
func BarrierAttenuationBands(g *BarrierGeometry, groundAtten BandLevels, c2 float64) BandLevels {
	var result BandLevels
	if g == nil {
		return result
	}

	z := pathDifference(*g)

	// A non-positive path difference means the obstacle does not screen this
	// path (7.4). It is then not a screening obstacle at all, so Eq. 12 does
	// not apply: A_bar stays 0 and A_gr of Eq. 4 keeps its full effect instead
	// of being cancelled.
	if z <= 0 {
		return result
	}

	for i := range NumBands {
		dz := BarrierDz(*g, z, OctaveBandFrequencies[i], c2)

		abar := dz - groundAtten[i]
		if abar < 0 {
			abar = 0
		}

		result[i] = abar
	}

	return result
}

// barrierEndpointToleranceM is the distance from either end of the
// source→receiver line below which a crossing is a graze rather than an
// obstacle standing between the two. It matches the tolerance RLS-19 applies
// to the same question.
const barrierEndpointToleranceM = 1e-6

// DeriveBarrierGeometry builds the per-path diffraction geometry of Gl. 16/17
// by intersecting the source→receiver ray with a scene of barriers, selecting
// the significant diffraction edges with the rubber band method, and measuring
// the diffracted path over them.
//
// It returns nil when nothing screens the path, which is the same input
// BarrierAttenuationBands already reads as "no barrier" and which therefore
// leaves A_bar at 0 and A_gr of Gl. 4 at its full effect.
//
// sourceHeightM and receiverHeightM must be on the same datum as
// Barrier.HeightM.
//
// Two fields of the result are fixed by what this module computes, and both
// are boundaries rather than approximations:
//
//   - A is 0. Fig. 6's a is the component of the source→receiver distance
//     parallel to the barrier edge, and it is non-zero only on the *lateral*
//     diffraction path around the end of an obstacle. This module diffracts
//     over the top edge, in the vertical source→receiver plane, where a is
//     zero by construction.
//   - LineOfSightClear is false. That flag carries 7.4's negative sign for a
//     geometry the caller supplies whose sight line passes above the top
//     edge. An edge only reaches this function once ObstructsLineOfSight has
//     accepted it, so the case cannot arise on a derived path.
//
// screeningEdge is one selected diffraction edge: where the ray crosses it,
// how high it stands, and the plan-view orientation of the crossed segment.
//
// The orientation is what makes Gl. 16/17 evaluable. Those equations decompose
// the diffracted path into a component in the plane perpendicular to the
// diffraction edge — which is what d_ss and d_sr measure — and the component a
// parallel to that edge. Measuring the legs along the source-receiver ray
// instead, with a = 0, silently assumes every screen stands square to the ray.
type screeningEdge struct {
	point      geo.Point2D
	topHeightM float64
	segmentA   geo.Point2D
	segmentB   geo.Point2D
}

func DeriveBarrierGeometry(
	source geo.Point2D, sourceHeightM float64,
	receiver geo.Point2D, receiverHeightM float64,
	barriers []Barrier,
) *BarrierGeometry {
	if len(barriers) == 0 {
		return nil
	}

	horizontalDist := geo.Distance(source, receiver)
	if horizontalDist <= 0 {
		return nil
	}

	crossings := geo.RayCrossings(
		source, receiver, barriers,
		func(b Barrier) []geo.Point2D { return b.Geometry },
		barrierEndpointToleranceM,
	)

	candidates := make([]geo.ScreeningPoint, 0, len(crossings))
	obstructing := make([]screeningEdge, 0, len(crossings))

	for _, crossing := range crossings {
		barrier := barriers[crossing.ObstacleIndex]

		height := barrier.HeightM
		if !geo.ObstructsLineOfSight(crossing.DistFromSource, height, sourceHeightM, receiverHeightM, horizontalDist) {
			continue
		}

		candidates = append(candidates, geo.ScreeningPoint{
			DistFromSource: crossing.DistFromSource,
			TopHeightM:     height,
		})
		obstructing = append(obstructing, screeningEdge{
			point:      crossing.Point,
			topHeightM: height,
			segmentA:   barrier.Geometry[crossing.SegmentIndex],
			segmentB:   barrier.Geometry[crossing.SegmentIndex+1],
		})
	}

	// The rubber band runs over distance along the ray, while the geometry
	// below is measured perpendicular to the edge. For parallel edges the two
	// differ by one positive factor along the whole section, and scaling the
	// x axis of an upper convex hull does not change which points it rests on,
	// so the selection is the same either way.
	selected := geo.SelectDiffractionEdges(sourceHeightM, receiverHeightM, horizontalDist, candidates)
	if len(selected) == 0 {
		return nil
	}

	edges := make([]screeningEdge, 0, len(selected))
	for _, index := range selected {
		edges = append(edges, obstructing[index])
	}

	geometry, ok := barrierGeometryFromEdges(edges, source, sourceHeightM, receiver, receiverHeightM, horizontalDist)
	if !ok {
		return nil
	}

	// A derived geometry that cannot describe a real path is a defect in this
	// derivation, not a user input to refuse mid-computation. Dropping it
	// leaves A_bar at 0, which is the unscreened answer.
	if geometry.Validate() != nil {
		return nil
	}

	return &geometry
}

// barrierGeometryFromEdges measures the diffracted path over the selected
// edges in the frame Gl. 16/17 are written in.
//
// The first (source-side) edge fixes that frame: its plan-view direction u is
// the edge direction, and n perpendicular to it spans the section the path is
// drawn in. Then d_ss and d_sr are the distances from source and receiver to
// the outer edges *within that section*, e runs between the edges in it, and
// a is the source-to-receiver component along u. Gl. 16/17 recombine them as
// hypot(d_ss + d_sr + e, a), which is exactly the length of the shortest path
// from source to receiver over a straight edge — the unfolded straight line.
//
// For several edges this assumes they are effectively parallel, which is the
// case Bild 7 draws; the standard defines no path difference for edges that
// are not. Where they are not, the source-side edge governs, because it is the
// one d_ss is measured to and the one nearest the source.
func barrierGeometryFromEdges(
	edges []screeningEdge,
	source geo.Point2D, sourceHeightM float64,
	receiver geo.Point2D, receiverHeightM float64,
	horizontalDistM float64,
) (BarrierGeometry, bool) {
	first := edges[0]
	last := edges[len(edges)-1]

	edgeX, edgeY := first.segmentB.X-first.segmentA.X, first.segmentB.Y-first.segmentA.Y

	length := math.Hypot(edgeX, edgeY)
	if length <= 0 {
		return BarrierGeometry{}, false
	}

	edgeX, edgeY = edgeX/length, edgeY/length

	// Signed offset from the first edge's line, measured along its normal.
	offset := func(p geo.Point2D) float64 {
		return -(p.X-first.segmentA.X)*edgeY + (p.Y-first.segmentA.Y)*edgeX
	}

	offsetSource, offsetReceiver := offset(source), offset(receiver)

	dss := math.Hypot(math.Abs(offset(first.point)-offsetSource), first.topHeightM-sourceHeightM)
	dsr := math.Hypot(math.Abs(offsetReceiver-offset(last.point)), receiverHeightM-last.topHeightM)

	// e is the travel path length between the first and the last diffraction
	// edge, i.e. the sum over every consecutive pair, not the straight chord
	// between them: the ray runs edge to edge. The chord would under-estimate
	// e and therefore z.
	var e float64

	for i := 1; i < len(edges); i++ {
		e += math.Hypot(
			math.Abs(offset(edges[i].point)-offset(edges[i-1].point)),
			edges[i].topHeightM-edges[i-1].topHeightM,
		)
	}

	return BarrierGeometry{
		Dss: dss,
		Dsr: dsr,
		E:   e,
		A:   math.Abs((receiver.X-source.X)*edgeX + (receiver.Y-source.Y)*edgeY),
		D:   math.Hypot(horizontalDistM, receiverHeightM-sourceHeightM),
	}, true
}
