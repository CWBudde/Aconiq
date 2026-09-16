package cli

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
)

// terrainInComputeCRS adapts a terrain model stored in the project CRS so it
// can be queried with compute-CRS coordinates.
//
// It wraps rather than resamples. Reprojecting the grid itself would
// interpolate every cell into a new raster — lossy, and expensive for a DTM —
// while the query path needs only two numbers transformed per lookup.
//
// Without it a run that projects the model leaves the terrain behind: every
// ElevationAt call arrives in metres against a grid in degrees, falls outside
// the bounds, and terrainElevationAt turns that into an elevation of 0 without
// saying so. Silence is what makes it dangerous — the receiver heights and the
// propagation path would simply be wrong.
type terrainInComputeCRS struct {
	inner    terrain.Model
	toNative geo.TransformPipeline
}

// newTerrainInComputeCRS wraps a terrain model when the run computes in a CRS
// the terrain is not in. It returns the model unchanged when no transform is
// needed, so the ordinary projected project keeps querying the grid directly.
func newTerrainInComputeCRS(model terrain.Model, projection computeProjection) (terrain.Model, error) {
	if model == nil || !projection.Applied {
		return model, nil
	}

	computeCRS, err := geo.ParseCRS(projection.ComputeCRS)
	if err != nil {
		return nil, fmt.Errorf("parse compute CRS %q: %w", projection.ComputeCRS, err)
	}

	nativeCRS, err := geo.ParseCRS(projection.ProjectCRS)
	if err != nil {
		return nil, fmt.Errorf("parse project CRS %q: %w", projection.ProjectCRS, err)
	}

	toNative, err := geo.BuildTransformPipeline(nativeCRS, computeCRS)
	if err != nil {
		return nil, fmt.Errorf("build terrain transform %s -> %s: %w", computeCRS.ID, nativeCRS.ID, err)
	}

	return terrainInComputeCRS{inner: model, toNative: toNative}, nil
}

// ElevationAt transforms the query into the terrain's own CRS. A point the
// transform refuses is outside the terrain rather than at elevation zero, so
// it is reported as a miss — which is the same answer the wrapped model gives
// for a point outside its bounds.
func (t terrainInComputeCRS) ElevationAt(x, y float64) (float64, bool) {
	native, err := t.toNative.ApplyPoint(geo.Point2D{X: x, Y: y})
	if err != nil {
		return 0, false
	}

	return t.inner.ElevationAt(native.X, native.Y)
}

// Bounds and Info describe the terrain in its native CRS, unchanged.
//
// Converting the bounds would mean projecting a rectangle, and the projection
// of a rectangle is not a rectangle: the four transformed corners bound a
// curved quadrilateral, so any [4]float64 answer is either too large or too
// small somewhere. Nothing on the run path reads these — they serve
// `aconiq status` and the import report, which describe the stored artifact —
// so reporting the artifact's own extent is both correct and what callers
// expect.
func (t terrainInComputeCRS) Bounds() [4]float64 { return t.inner.Bounds() }

func (t terrainInComputeCRS) Info() terrain.Info { return t.inner.Info() }
