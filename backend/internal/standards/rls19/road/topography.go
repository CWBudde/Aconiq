package road

import (
	"errors"
	"fmt"
	"sort"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
)

// TerrainEdge represents a slope edge (Böschungskante or Böschungsfuß) as a 3D
// polyline with absolute elevations. In the RLS-19 model:
//   - Böschungskante (SlopeCrest): upper edge of cut or embankment — acts as a
//     diffraction edge for acoustic shielding (like a noise barrier).
//   - Böschungsfuß (SlopeFoot): lower edge — refines the terrain height profile
//     used for the mean-height (h_m) ground-effect correction.
type TerrainEdge struct {
	ID       string        `json:"id"`
	Geometry []geo.Point3D `json:"geometry"` // 3D polyline, at least 2 points
}

// Validate checks the terrain edge definition.
func (e TerrainEdge) Validate() error {
	if e.ID == "" {
		return errors.New("terrain edge id is required")
	}

	if len(e.Geometry) < 2 {
		return fmt.Errorf("terrain edge %q geometry must contain at least 2 points", e.ID)
	}

	for i, pt := range e.Geometry {
		if !pt.IsFinite() {
			return fmt.Errorf("terrain edge %q geometry[%d] is not finite", e.ID, i)
		}
	}

	return nil
}

// geometry2D returns the plan-view (XY) projection of the terrain edge.
func (e TerrainEdge) geometry2D() []geo.Point2D {
	pts := make([]geo.Point2D, len(e.Geometry))
	for i, p := range e.Geometry {
		pts[i] = p.XY()
	}

	return pts
}

// TerrainSlope describes one slope face beside a road. It covers both
// Tieflage (road in cut) and Hochlage (road on embankment):
//
//   - Tieflage: SlopeCrest is at terrain level; SlopeFoot is at road level.
//   - Hochlage: SlopeCrest is at road level; SlopeFoot is at terrain level.
//
// SlopeCrest is required and acts as a diffraction edge. SlopeFoot is optional
// and, when provided, improves the terrain-height profile for h_m.
type TerrainSlope struct {
	SlopeCrest TerrainEdge  `json:"slope_crest"`          // Böschungskante (required)
	SlopeFoot  *TerrainEdge `json:"slope_foot,omitempty"` // Böschungsfuß (optional)
}

// TerrainProfile collects terrain slopes around a road source.
// Multiple slopes may exist (e.g. one on each side of the road).
type TerrainProfile struct {
	Slopes []TerrainSlope `json:"slopes"`
}

// terrainSampleStepM is the nominal spacing at which a DTM is sampled along a
// propagation path for the mean-height correction. The ground enters h_m only
// through its average, and h_m enters D_gr through a single term, so metre-fine
// sampling would buy no accuracy the correction can express; 25 m resolves an
// embankment or a cutting while keeping the query count per path in single
// figures at the distances RLS-19 assesses.
const terrainSampleStepM = 25.0

// groundPath is one source→receiver path together with everything that is
// known about the ground beneath it.
type groundPath struct {
	source, receiver geo.Point2D

	// sourceZ and receiverZ are the absolute elevations of the path's ends —
	// the substitute point source and the receiver.
	sourceZ, receiverZ float64

	// sourceGroundZ and receiverGroundZ are the absolute elevations of the
	// ground each end stands on. They are what makes h_m a height above the
	// ground rather than above the datum, so they are required, not optional.
	sourceGroundZ, receiverGroundZ float64

	// profiles are declared slope edges (Böschungskante, Böschungsfuß); dtm is
	// the imported terrain model, in the CRS the run computes in.
	profiles []TerrainProfile
	dtm      terrain.Model
}

// computeMeanHeight returns h_m, the mean height of the source–receiver path
// above the ground, using the RLS-19 modified formula:
//
//	h_m = (sourceZ + receiverZ) / 2 − avgTerrainZ
//
// where avgTerrainZ is the distance-weighted average ground elevation along the
// horizontal projection of the source–receiver path.
func computeMeanHeight(p groundPath) float64 {
	meanPathZ := (p.sourceZ + p.receiverZ) / 2.0

	return meanPathZ - p.avgTerrainZ()
}

// avgTerrainZ resolves the average ground elevation under the path, in order of
// preference:
//
//  1. the declared terrain profiles, integrated piecewise between their
//     breakpoints;
//  2. the imported DTM, sampled along the path and read relative to the ground
//     already known at each end;
//  3. the straight line between the ground elevations at the two ends.
//
// It is never zero. Zero is an elevation like any other, and answering with it
// where nothing is known makes h_m the height of the path above sea level: at a
// site a few hundred metres up that drives D_gr (Eq. 14) far below zero, where
// it clamps to 0 and the ground attenuation disappears without a word. Where no
// terrain is known the ground under each end is the ground at that end, which
// is what case 3 says; on flat ground it returns h_m = (sourceHeight +
// receiverHeight)/2 at any elevation, including zero.
func (p groundPath) avgTerrainZ() float64 {
	avg, ok := computeTerrainAvgZ(p.source, p.receiver, p.profiles)
	if ok {
		return avg
	}

	chordZ := (p.sourceGroundZ + p.receiverGroundZ) / 2.0

	rise, ok := terrain.MeanRiseAboveChord(
		p.dtm,
		p.source.X, p.source.Y,
		p.receiver.X, p.receiver.Y,
		terrainSampleStepM,
	)
	if ok {
		return chordZ + rise
	}

	return chordZ
}

// computeTerrainAvgZ computes the weighted-average terrain elevation along the
// horizontal source→receiver path, integrating all slope edges piecewise.
//
// The second return value is false when the declared profiles say nothing about
// this path — there are none, or none of their edges crosses it. That is not an
// average of zero; it is the caller's cue to fall back to what else it knows
// about the ground.
func computeTerrainAvgZ(source, receiver geo.Point2D, profiles []TerrainProfile) (float64, bool) {
	if len(profiles) == 0 {
		return 0, false
	}

	dTotal := dist2D(source, receiver)
	if dTotal < 1e-9 {
		return 0, false
	}

	// Collect terrain breakpoints: (distance from source, terrain Z).
	type bp struct{ d, z float64 }

	var bps []bp

	for _, profile := range profiles {
		for _, slope := range profile.Slopes {
			if d, z, ok := terrainEdgeCrossing(slope.SlopeCrest, source, receiver); ok {
				bps = append(bps, bp{d, z})
			}

			if slope.SlopeFoot != nil {
				if d, z, ok := terrainEdgeCrossing(*slope.SlopeFoot, source, receiver); ok {
					bps = append(bps, bp{d, z})
				}
			}
		}
	}

	if len(bps) == 0 {
		return 0, false
	}

	// Sort by distance from source.
	sort.Slice(bps, func(i, j int) bool { return bps[i].d < bps[j].d })

	// Build piecewise-linear terrain segments along the path.
	type seg struct{ d0, d1, z0, z1 float64 }

	var segs []seg

	first := bps[0]
	last := bps[len(bps)-1]

	// Before first breakpoint: terrain is at the first breakpoint elevation.
	if first.d > 1e-9 {
		segs = append(segs, seg{0, first.d, first.z, first.z})
	}

	// Between consecutive breakpoints: linear interpolation.
	for i := range len(bps) - 1 {
		segs = append(segs, seg{bps[i].d, bps[i+1].d, bps[i].z, bps[i+1].z})
	}

	// After last breakpoint: terrain is at the last breakpoint elevation.
	if last.d < dTotal-1e-9 {
		segs = append(segs, seg{last.d, dTotal, last.z, last.z})
	}

	// Trapezoidal integration → weighted average.
	totalArea := 0.0

	for _, s := range segs {
		w := s.d1 - s.d0
		if w < 0 {
			w = 0
		}

		totalArea += w * (s.z0 + s.z1) / 2.0
	}

	return totalArea / dTotal, true
}

// terrainEdgeCrossing finds where a terrain edge crosses the source→receiver
// path in plan view and returns the distance from source and the interpolated
// Z of the edge at the crossing.  Returns (0, 0, false) if no crossing.
func terrainEdgeCrossing(edge TerrainEdge, source, receiver geo.Point2D) (float64, float64, bool) {
	edge2D := edge.geometry2D()
	crossPt, edgeIdx, ok := geo.LineStringIntersectsSegment(edge2D, source, receiver)

	if !ok {
		return 0, 0, false
	}

	dFromSource := dist2D(source, crossPt)

	// Interpolate the edge Z at the crossing point.
	a := edge.Geometry[edgeIdx]
	b := edge.Geometry[edgeIdx+1]
	edgeSegLen := dist2D(a.XY(), b.XY())

	var edgeZ float64

	if edgeSegLen < 1e-9 {
		edgeZ = a.Z
	} else {
		t := dist2D(a.XY(), crossPt) / edgeSegLen
		if t < 0 {
			t = 0
		}

		if t > 1 {
			t = 1
		}

		edgeZ = a.Z + t*(b.Z-a.Z)
	}

	return dFromSource, edgeZ, true
}

// computeTerrainEdgeShielding returns the maximum insertion loss from terrain
// edges (Böschungskante) acting as diffraction edges.  All heights are absolute
// Z coordinates, making the pathDifference function translation-invariant.
func computeTerrainEdgeShielding(
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	profiles []TerrainProfile,
) ShieldingResult {
	best := ShieldingResult{}

	for _, profile := range profiles {
		for _, slope := range profile.Slopes {
			result := terrainEdgeShieldingResult(source, sourceZ, receiver, receiverZ, slope.SlopeCrest)
			if result.InsertionLoss > best.InsertionLoss {
				best = result
			}
		}
	}

	return best
}

// terrainEdgeShieldingResult computes insertion loss from a single terrain edge.
func terrainEdgeShieldingResult(
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	edge TerrainEdge,
) ShieldingResult {
	edge2D := edge.geometry2D()
	crossPt, edgeIdx, ok := geo.LineStringIntersectsSegment(edge2D, source, receiver)

	if !ok {
		return ShieldingResult{}
	}

	dSB := dist2D(source, crossPt)
	dBR := dist2D(crossPt, receiver)

	if dSB < 1e-6 || dBR < 1e-6 {
		return ShieldingResult{}
	}

	// Interpolate edge Z at crossing.
	a := edge.Geometry[edgeIdx]
	b := edge.Geometry[edgeIdx+1]
	edgeSegLen := dist2D(a.XY(), b.XY())

	var edgeZ float64

	if edgeSegLen < 1e-9 {
		edgeZ = a.Z
	} else {
		t := dist2D(a.XY(), crossPt) / edgeSegLen
		if t < 0 {
			t = 0
		}

		if t > 1 {
			t = 1
		}

		edgeZ = a.Z + t*(b.Z-a.Z)
	}

	// computeDiffraction is translation-invariant: absolute Z values are correct.
	diff := computeDiffraction(dSB, sourceZ, dBR, receiverZ, edgeZ)
	if diff.Z <= 0 {
		return ShieldingResult{}
	}

	loss := rls19BarrierLoss(diff)

	return ShieldingResult{
		Shielded:       true,
		InsertionLoss:  loss,
		PathDifference: diff.Z,
		BarrierID:      edge.ID,
	}
}
