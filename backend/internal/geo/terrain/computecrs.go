package terrain

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// inComputeCRS adapts a terrain model stored in one CRS so it can be queried
// with coordinates in the CRS a run computes in.
//
// It wraps rather than resamples. Reprojecting the grid itself would
// interpolate every cell into a new raster — lossy, and expensive for a DTM —
// while the query path needs only two numbers transformed per lookup.
//
// Without it a run that projects the model leaves the terrain behind: every
// ElevationAt call arrives in metres against a grid in degrees, falls outside
// the bounds, and the caller's miss handling turns that into an elevation of 0
// without saying so. Silence is what makes it dangerous — the receiver heights
// and the propagation path would simply be wrong.
type inComputeCRS struct {
	inner    Model
	toNative geo.TransformPipeline
}

// InComputeCRS wraps a terrain model when the caller computes in a CRS the
// terrain is not in. It returns the model unchanged when no transform is
// needed — no model, or a compute CRS that resolves to the terrain's own — so
// the ordinary case keeps querying the grid directly rather than transforming
// a point between a CRS and itself on every lookup.
//
// The CRS the terrain is in is a fact about the stored raster, and it has to be
// declared: the GeoTIFF loader in this package reads the tie point and the
// pixel scale, not the GeoKeyDirectory, so a Model does not know its own CRS
// and cannot be asked.
func InComputeCRS(model Model, nativeCRS, computeCRS string) (Model, error) {
	if model == nil {
		return model, nil
	}

	compute, err := geo.ParseCRS(computeCRS)
	if err != nil {
		return nil, fmt.Errorf("terrain: parse compute CRS %q: %w", computeCRS, err)
	}

	native, err := geo.ParseCRS(nativeCRS)
	if err != nil {
		return nil, fmt.Errorf("terrain: parse terrain CRS %q: %w", nativeCRS, err)
	}

	if native.ID == compute.ID {
		return model, nil
	}

	toNative, err := geo.BuildTransformPipeline(native, compute)
	if err != nil {
		return nil, fmt.Errorf("terrain: build transform %s -> %s: %w", compute.ID, native.ID, err)
	}

	return inComputeCRS{inner: model, toNative: toNative}, nil
}

// ElevationAt transforms the query into the terrain's own CRS. A point the
// transform refuses is outside the terrain rather than at elevation zero, so
// it is reported as a miss — which is the same answer the wrapped model gives
// for a point outside its bounds.
func (t inComputeCRS) ElevationAt(x, y float64) (float64, bool) {
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
func (t inComputeCRS) Bounds() [4]float64 { return t.inner.Bounds() }

func (t inComputeCRS) Info() Info { return t.inner.Info() }
