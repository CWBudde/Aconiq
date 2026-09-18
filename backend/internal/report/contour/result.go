package contour

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
)

// The refusals FromRaster distinguishes, as sentinels rather than as message
// text.
//
// A caller has to tell them apart to answer usefully — the HTTP route maps one
// to a conflict and the other to a bad request, and the map panel says a
// different sentence for each — and matching on wording would break silently
// the first time the wording improved. The messages themselves stay here, so a
// refusal reaches a browser in the words the CLI would use.
var (
	// ErrNotAGrid is a raster whose receivers were placed individually: there
	// is no cell size, so there is nothing to place a contour on.
	ErrNotAGrid = errors.New(
		"this run's raster declares no georeference, so there is nothing to place its " +
			"contours on: only a grid receiver set records a cell size, and these receivers " +
			"were placed individually")

	// ErrCRSNotTransformable is a CRS at either end with no EPSG code.
	// geo.BuildTransformPipeline needs one at both.
	ErrCRSNotTransformable = errors.New(
		"contours can only be moved between CRS that carry an EPSG code")
)

// Result is one run's contours, and the CRS they are in.
//
// CRS is always populated. A consumer handed bare lines has to guess, and it
// will guess WGS84 — which for a metric run puts the site off the coast of
// Africa, the failure `cli.errNoGeoTransform` already exists to prevent
// elsewhere. Interval is echoed for the same reason: the caller may have
// omitted it and taken the default, and a legend that names the step has to
// know which step it got.
type Result struct {
	CRS      string  `json:"crs"`
	Interval float64 `json:"interval"`
	Lines    []Line  `json:"lines"`
}

// Reproject moves contour vertices between two CRS.
//
// Split out of `cli.contoursInWGS84` so the file on disk and the two wire
// boundaries share one transform loop. The *policy* around it is deliberately
// not shared: `cli` passes a CRS it cannot transform through untouched, because
// failing would break bundles produced today for nothing, while FromRaster
// below refuses one. Each caller argues its own case; this only moves points.
//
// Note the argument order BuildTransformPipeline takes. It reads as
// (datumA, datumB) rather than (source, target), so the target goes first.
func Reproject(lines []Line, from geo.CRS, to geo.CRS) ([]Line, error) {
	if from.ID == to.ID {
		return lines, nil
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return nil, fmt.Errorf("build transform %s -> %s: %w", from.ID, to.ID, err)
	}

	out := make([]Line, len(lines))

	for i, line := range lines {
		moved := line
		moved.Points = make([][2]float64, len(line.Points))

		for j, point := range line.Points {
			transformed, applyErr := pipeline.ApplyPoint(geo.Point2D{X: point[0], Y: point[1]})
			if applyErr != nil {
				return nil, fmt.Errorf("contour %d vertex %d: %w", i, j, applyErr)
			}

			moved.Points[j] = [2]float64{transformed.X, transformed.Y}
		}

		out[i] = moved
	}

	return out, nil
}

// FromRaster is the whole of what a boundary does with a run's raster: place
// it, contour it, and say where the lines are.
//
// It is what `window.aconiq.contours` and GET /api/v1/runs/{id}/contours both
// call, so that the browser and the API cannot answer the same question
// differently. One call returns *every* band, told apart by
// Line.BandName — GenerateContours already walks them all, and a caller
// showing one band filters rather than asking again.
//
// **A declared georeference is required.** `cli` can fall back to inferring the
// transform from receiver coordinates, and still does for sidecars written
// before the georeference existed; this cannot, because a raster does not carry
// the receiver table that inference reads. That is the honest shape rather than
// a gap: a run whose receivers were not a grid has no cell size, and refusing
// is what the callers already say to a reader ("not a grid").
//
// **It refuses a CRS it cannot transform, where `cli` passes one through.** A
// GeoJSON file a reviewer opens in QGIS is a different risk from a layer this
// app draws under its own legend and calls a result.
func FromRaster(raster *results.Raster, opts Options, targetCRS string) (Result, error) {
	if raster == nil {
		return Result{}, errors.New("raster is nil")
	}

	meta := raster.Metadata()

	if meta.Geo == nil {
		return Result{}, ErrNotAGrid
	}

	if opts.Interval <= 0 {
		opts.Interval = DefaultInterval
	}

	source, err := transformableCRS(meta.CRS, "the raster")
	if err != nil {
		return Result{}, err
	}

	target, err := transformableCRS(targetCRS, "the requested target")
	if err != nil {
		return Result{}, err
	}

	// A row order this build cannot read is refused here too, but it is not
	// reachable through results.NewRaster or LoadRaster, both of which validate
	// the georeference first — see TestGeoTransformFromGeoreferenceRefusesWhat
	// ItCannotRead. This is the guard for a raster assembled some other way.
	transform, err := GeoTransformFromGeoreference(*meta.Geo, meta.Height)
	if err != nil {
		return Result{}, fmt.Errorf("place the raster: %w", err)
	}

	lines, err := GenerateContours(raster, transform, opts)
	if err != nil {
		return Result{}, fmt.Errorf("generate contours: %w", err)
	}

	moved, err := Reproject(lines, source, target)
	if err != nil {
		return Result{}, fmt.Errorf("reproject contours: %w", err)
	}

	// Never nil. A raster with no valid data in any band would otherwise
	// marshal as `"lines": null`, and every consumer of both boundaries would
	// have to defend against it separately — the kernel's TypeScript wrapper
	// first among them.
	if moved == nil {
		moved = []Line{}
	}

	return Result{CRS: target.ID, Interval: opts.Interval, Lines: moved}, nil
}

// transformableCRS parses a CRS and refuses one there is no pipeline through.
//
// `geo.BuildTransformPipeline` needs an EPSG code at both ends, so a `WKT:`
// identifier is refused here rather than at the point where it would silently
// produce metres labelled as degrees. The refusal names which end failed,
// because "unsupported CRS" on its own leaves the reader checking both.
func transformableCRS(value string, which string) (geo.CRS, error) {
	parsed, err := geo.ParseCRS(value)
	if err != nil {
		return geo.CRS{}, fmt.Errorf("%s CRS %q is not a recognised identifier: %w", which, value, err)
	}

	if parsed.EPSGCode() == 0 {
		// Wrapped, so a caller can match the sentinel while the reader still
		// gets told which end failed and what to send instead.
		return geo.CRS{}, fmt.Errorf(
			"%w: %s CRS %q carries none, so send an EPSG identifier such as \"EPSG:25832\"",
			ErrCRSNotTransformable, which, parsed.ID)
	}

	return parsed, nil
}
