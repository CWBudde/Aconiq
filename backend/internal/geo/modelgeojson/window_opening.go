package modelgeojson

import (
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// openingPlaneToleranceM is how far an opening's outline may lie from the
// line of the facade it is in. A LoD3 window sits in its reveal, recessed
// from the wall's outer face by the frame's depth, and 0.5 m covers the
// thickest reveal of a masonry wall without reaching a facade a window is
// not in: a window on one wall lies metres from the line of the others.
const openingPlaneToleranceM = 0.5

// OpeningSpec is a window given by its outline in 3D, as a CityGML LoD3
// bldg:Window states it (P18 C1b): the building it belongs to, its vertices
// in the project CRS with z above the datum, and the base of the building in
// the same datum, from which its sill is measured.
type OpeningSpec struct {
	ID          string
	BuildingRef string
	Vertices    [][3]float64
	BaseZ       float64
	Evidence    []any
}

// OpeningsInProjectCRS moves the outlines of specs from importCRS into
// projectCRS, by the transform NormalizeWithCRS applies to the features read
// with them, so a window stays on the facade its building's footprint was
// moved to. z is a height above the datum and is left as it is. Equal or
// empty CRS return specs unchanged.
func OpeningsInProjectCRS(specs []OpeningSpec, projectCRS, importCRS string) ([]OpeningSpec, error) {
	pipeline, needed, err := buildCRSPipeline(projectCRS, importCRS)
	if err != nil || !needed {
		return specs, err
	}

	moved := make([]OpeningSpec, len(specs))

	for i, spec := range specs {
		moved[i] = spec
		moved[i].Vertices = make([][3]float64, len(spec.Vertices))

		for k, v := range spec.Vertices {
			p, err := pipeline.ApplyPoint(geo.Point2D{X: v[0], Y: v[1]})
			if err != nil {
				return nil, fmt.Errorf("window %q: vertex %d: %w", spec.ID, k, err)
			}

			moved[i].Vertices[k] = [3]float64{p.X, p.Y, v[2]}
		}
	}

	return moved, nil
}

// openingFit is how an opening's outline lies along one canonical edge: the
// span of its vertices along the edge, in the metric frame.
type openingFit struct {
	edge       geo.CanonicalFacadeEdge
	uMin, uMax float64
}

// OpeningFeature places a window from its outline: on the one facade of its
// building in whose plane every vertex lies, within openingPlaneToleranceM
// of the edge's line and inside its span, with u_m and width_m the outline's
// extent along the edge, and sill_z_m and height_m its extent in z above
// BaseZ. The window is proposed and not sensitive: LoD3 says where a window
// is, never what the room behind it is used for. It refuses an outline in no
// facade plane, or in more than one, naming the opening.
func (p WindowPlacer) OpeningFeature(spec OpeningSpec) (Feature, error) {
	id := strings.TrimSpace(spec.ID)

	building, ok := p.index.byID[spec.BuildingRef]
	if !ok {
		return Feature{}, fmt.Errorf("window %q: building %q is not in the model, or has no stable id to key a facade by", id, spec.BuildingRef)
	}

	if len(spec.Vertices) < 3 {
		return Feature{}, fmt.Errorf("window %q: its outline has %d vertices, fewer than a polygon needs", id, len(spec.Vertices))
	}

	points := make([]geo.Point2D, len(spec.Vertices))
	minZ, maxZ := math.Inf(1), math.Inf(-1)

	for i, v := range spec.Vertices {
		if !isFinite(v[0]) || !isFinite(v[1]) || !isFinite(v[2]) || !isFinite(spec.BaseZ) {
			return Feature{}, fmt.Errorf("window %q: vertex %d %v and the base z %g must be finite", id, i, v, spec.BaseZ)
		}

		metric, err := p.index.toMetric(geo.Point2D{X: v[0], Y: v[1]})
		if err != nil {
			return Feature{}, fmt.Errorf("window %q: vertex %d: %w", id, i, err)
		}

		points[i] = metric
		minZ, maxZ = math.Min(minZ, v[2]), math.Max(maxZ, v[2])
	}

	fit, err := p.fitOpening(building, id, points)
	if err != nil {
		return Feature{}, err
	}

	// Both ends are clamped to the edge, which fitOnEdge let them overhang
	// by the fit tolerance, so the window it accepted also fits.
	start := math.Max(fit.uMin, 0)
	frame := geo.FacadeWindow{
		UM:      start,
		SillZM:  minZ - spec.BaseZ,
		WidthM:  math.Min(fit.uMax, geo.Distance(fit.edge.From, fit.edge.To)) - start,
		HeightM: maxZ - minZ,
	}

	// A sill within the tolerance of the ground is on it: the datum and the
	// outline come from one survey, and a rounding below zero is no window
	// below the ground.
	if frame.SillZM < 0 && frame.SillZM >= -geo.FacadeWindowFitToleranceM {
		frame.SillZM = 0
	}

	if frame.SillZM < 0 {
		return Feature{}, fmt.Errorf("window %q: its sill at z %g is %.3f m below its building's base at z %g, so sill_z_m would be negative",
			id, minZ, -frame.SillZM, spec.BaseZ)
	}

	return p.Feature(WindowSpec{
		ID:           id,
		BuildingRef:  spec.BuildingRef,
		Part:         fit.edge.Part,
		Ring:         fit.edge.Ring,
		Edge:         fit.edge.Edge,
		Frame:        frame,
		ReviewStatus: geo.ReviewStatusProposed,
		Evidence:     spec.Evidence,
	})
}

// fitOpening finds the one edge of the building whose plane holds the
// outline.
func (p WindowPlacer) fitOpening(building int, id string, points []geo.Point2D) (openingFit, error) {
	var fits []openingFit

	for _, edge := range p.index.buildings[building].edges {
		if fit, ok := fitOnEdge(edge, points); ok {
			fits = append(fits, fit)
		}
	}

	switch len(fits) {
	case 1:
		return fits[0], nil
	case 0:
		return openingFit{}, fmt.Errorf(
			"window %q lies in no facade plane of building %q: no edge has every vertex within %g m of its line and inside its span%s",
			id, p.index.buildings[building].id, openingPlaneToleranceM, p.index.measuredIn(),
		)
	default:
		names := make([]string, len(fits))
		for i, f := range fits {
			names[i] = fmt.Sprintf("part %d, ring %d, edge %d", f.edge.Part, f.edge.Ring, f.edge.Edge)
		}

		return openingFit{}, fmt.Errorf("window %q lies in the plane of %d facades of building %q (%s); its facade cannot be told",
			id, len(fits), p.index.buildings[building].id, strings.Join(names, "; "))
	}
}

// fitOnEdge measures the outline against one edge: every vertex within
// openingPlaneToleranceM of its line, the outline's span along it inside the
// edge, and wider than the fit tolerance, so a window on a perpendicular wall
// near the corner, which projects onto this edge as a point, is not taken for
// one in it.
func fitOnEdge(edge geo.CanonicalFacadeEdge, points []geo.Point2D) (openingFit, bool) {
	length := geo.Distance(edge.From, edge.To)
	if !(length > 0) {
		return openingFit{}, false
	}

	tx, ty := (edge.To.X-edge.From.X)/length, (edge.To.Y-edge.From.Y)/length
	fit := openingFit{edge: edge, uMin: math.Inf(1), uMax: math.Inf(-1)}

	for _, pt := range points {
		dx, dy := pt.X-edge.From.X, pt.Y-edge.From.Y
		if math.Abs(dx*ty-dy*tx) > openingPlaneToleranceM {
			return openingFit{}, false
		}

		u := dx*tx + dy*ty
		fit.uMin, fit.uMax = math.Min(fit.uMin, u), math.Max(fit.uMax, u)
	}

	tolerance := geo.FacadeWindowFitToleranceM

	return fit, fit.uMin >= -tolerance && fit.uMax <= length+tolerance && fit.uMax-fit.uMin > tolerance
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
