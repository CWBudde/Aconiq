// Package crstransform projects a batch of coordinates between two CRS, and
// owns the one decision about which CRS a geographic model goes into.
//
// It exists as its own package because that decision has three consumers that
// must not be able to disagree: `aconiq run`, which projects before anything
// reads a coordinate; the WebAssembly kernel, which gives browser mode the same
// projector; and POST /api/v1/transform, which gives API mode a projector at
// all. A second implementation in any of the three would let the browser draw a
// model where the CLI would not compute it.
//
// It is deliberately below all three rather than inside one of them.
// internal/wasmkernel is defined as the logic behind the kernel's JavaScript
// surface; the local HTTP API depending on that package would say something
// untrue about both.
package crstransform

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// AutoTarget asks Transform to make the same decision `aconiq run` makes:
// project a geographic model into the ETRS89 / UTM zone its centre falls in,
// and leave a model that is already metric exactly where it is.
//
// The decision lives in Go rather than in the browser on purpose. It is
// geo.ComputeCRSForGeographic either way, so the browser cannot resolve a
// different zone — or a different refusal — than the CLI would for the same
// site.
const AutoTarget = "auto"

// Request is the batch to project.
//
// Coordinates are flat and interleaved — x0, y0, x1, y1, … — rather than nested
// pairs. A per-point crossing of the JavaScript boundary is not viable for a
// model of any size, so the call is batched; flat halves the JSON of the batch
// and leaves the door open to handing a Float64Array across instead.
//
// The batch is the *model*, never the receiver grid: the model is projected
// first and the grid is then built in the compute CRS, so no grid coordinate
// ever crosses the boundary.
type Request struct {
	SourceCRS   string    `json:"source_crs"`
	TargetCRS   string    `json:"target_crs"`
	Coordinates []float64 `json:"coordinates"`
}

// Response is the projected batch.
//
// TargetCRS is the CRS the coordinates are actually in, which for an AutoTarget
// request is the resolved zone rather than the string that was asked for.
// Applied is false only when nothing moved, and then Coordinates are the input
// values verbatim — not a round trip that happens to land close.
type Response struct {
	SourceCRS   string    `json:"source_crs"`
	TargetCRS   string    `json:"target_crs"`
	Applied     bool      `json:"applied"`
	Coordinates []float64 `json:"coordinates"`
}

// Transform projects a batch of coordinates between two CRS.
//
// Every standards module measures distance with geo.Distance, which is
// math.Hypot over the coordinates it is handed: correct for a metric projected
// CRS and silently wrong for a geographic one, where a whole city fits inside
// 0.05 units, every propagation distance falls under the modules'
// minimum-distance clamp, and each receiver reports the source's emission level
// verbatim.
func Transform(req Request) (Response, error) {
	if len(req.Coordinates)%2 != 0 {
		return Response{}, refuse(ReasonOddCoordinateCount, NoIndex, fmt.Errorf(
			"coordinates must hold an even number of values (x, y interleaved), got %d",
			len(req.Coordinates),
		))
	}

	source, err := geo.ParseCRS(req.SourceCRS)
	if err != nil {
		return Response{}, refuse(ReasonSourceCRS, NoIndex,
			fmt.Errorf("parse source CRS %q: %w", req.SourceCRS, err))
	}

	target, auto, err := resolveTarget(req, source)
	if err != nil {
		return Response{}, err
	}

	// Nothing to do: either the model is already metric, or it is geographic and
	// carries no coordinate to take a centre from. Both mirror
	// cli.resolveComputeModel's early returns, and both copy the input verbatim
	// rather than round-tripping it through an identity pipeline.
	if auto && target.ID == source.ID {
		return Response{
			SourceCRS:   source.ID,
			TargetCRS:   source.ID,
			Applied:     false,
			Coordinates: append([]float64(nil), req.Coordinates...),
		}, nil
	}

	// Argument order is target-first, as geo.BuildTransformPipeline declares it.
	pipeline, err := geo.BuildTransformPipeline(target, source)
	if err != nil {
		return Response{}, refuse(ReasonPipelineUnavailable, NoIndex,
			fmt.Errorf("build CRS transform %s -> %s: %w", source.ID, target.ID, err))
	}

	moved := make([]float64, len(req.Coordinates))

	for i := 0; i < len(req.Coordinates); i += 2 {
		point, err := pipeline.ApplyPoint(geo.Point2D{X: req.Coordinates[i], Y: req.Coordinates[i+1]})
		if err != nil {
			return Response{}, refuse(ReasonCoordinate, i/2,
				fmt.Errorf("coordinate %d: %w", i/2, err))
		}

		moved[i] = point.X
		moved[i+1] = point.Y
	}

	return Response{
		SourceCRS:   source.ID,
		TargetCRS:   target.ID,
		Applied:     true,
		Coordinates: moved,
	}, nil
}

// GeographicRefusal is the single place the sentence refusing a geographic
// model outside the supported ETRS89 / UTM zones is written.
//
// It is exported because three surfaces have to refuse the same site in the
// same words, and only two of them come through Transform: POST
// /api/v1/transform and the WebAssembly kernel do, while `aconiq run` resolves
// the zone itself and needs the sentence for the message inside its own error
// envelope. A second literal in any of the three would be free to drift, which
// is exactly what this function removes.
//
// sourceCRS is the canonicalised geo.CRS.ID rather than whatever identifier the
// caller was handed, so the three also name the CRS in identical words.
// cli.TestGeographicRefusalReadsTheSameOnAllThreeSurfaces compares them byte
// for byte.
func GeographicRefusal(sourceCRS string, cause error) *Error {
	return refuse(ReasonGeographicRefused, NoIndex, errors.New(
		"project CRS "+sourceCRS+
			" is geographic, so the model has to be projected before levels can be computed: "+
			cause.Error(),
	))
}

// resolveTarget answers which CRS the batch is going into, and whether that was
// this package's decision to make. An explicit target always transforms, which
// is what makes the inverse direction free; only AutoTarget consults the
// coordinates.
func resolveTarget(req Request, source geo.CRS) (geo.CRS, bool, error) {
	if req.TargetCRS != "" && req.TargetCRS != AutoTarget {
		target, err := geo.ParseCRS(req.TargetCRS)
		if err != nil {
			return geo.CRS{}, false, refuse(ReasonTargetCRS, NoIndex,
				fmt.Errorf("parse target CRS %q: %w", req.TargetCRS, err))
		}

		return target, false, nil
	}

	if source.Kind != geo.CRSKindGeographic {
		return source, true, nil
	}

	centreLon, centreLat, ok := centre(req.Coordinates)
	if !ok {
		return source, true, nil
	}

	target, err := geo.ComputeCRSForGeographic(centreLon, centreLat)
	if err != nil {
		// Verbatim, so that browser mode, API mode and `aconiq run` refuse the
		// same site in the same words.
		return geo.CRS{}, false, GeographicRefusal(source.ID, err)
	}

	return target, true, nil
}

// centre returns the middle of the batch's bounding box, matching what
// cli.resolveComputeModel takes off modelgeojson.Model.Bounds.
func centre(coordinates []float64) (lon, lat float64, ok bool) {
	if len(coordinates) < 2 {
		return 0, 0, false
	}

	minX, maxX := coordinates[0], coordinates[0]
	minY, maxY := coordinates[1], coordinates[1]

	for i := 2; i < len(coordinates); i += 2 {
		minX = min(minX, coordinates[i])
		maxX = max(maxX, coordinates[i])
		minY = min(minY, coordinates[i+1])
		maxY = max(maxY, coordinates[i+1])
	}

	bbox := geo.BBox{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	if !bbox.IsFinite() {
		return 0, 0, false
	}

	return (minX + maxX) / 2, (minY + maxY) / 2, true
}
