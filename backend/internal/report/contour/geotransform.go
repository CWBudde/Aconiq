package contour

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/report/results"
)

// GeoTransform describes the affine mapping from pixel to projected coordinates.
// OriginX/OriginY is the top-left corner of the top-left pixel.
type GeoTransform struct {
	OriginX    float64 // X coordinate of top-left pixel corner
	OriginY    float64 // Y coordinate of top-left pixel corner
	PixelSizeX float64 // pixel width in CRS units (positive = east)
	PixelSizeY float64 // pixel height in CRS units (negative = south)
}

// GeoTransformFromGeoreference converts a raster's declared georeference into
// the corner-based affine transform every GIS format wants.
//
// This is the only place the two conventions meet. The sidecar records the
// centre of cell (0,0) — a grid receiver is a point in the middle of the cell
// it stands for — while GeoTransform records the top-left corner of the
// top-left pixel, so the conversion walks half a pixel west and, because
// row 0 is the southernmost, the full height plus half a pixel north.
//
// It refuses a row order it does not know rather than guessing: a north-up
// raster read as south-up is a vertically mirrored noise map that looks
// entirely plausible.
//
// Browser mode cannot call it, so it is mirrored in
// frontend/src/map/raster-extent.ts and pinned from both sides by
// testdata/raster-parity/geotransform.golden.json — see formats_parity_test.go
// before changing the arithmetic here.
func GeoTransformFromGeoreference(georef results.Georeference, gridHeight int) (GeoTransform, error) {
	err := georef.Validate()
	if err != nil {
		return GeoTransform{}, fmt.Errorf("raster georeference: %w", err)
	}

	if gridHeight <= 0 {
		return GeoTransform{}, errors.New("grid height must be positive")
	}

	return GeoTransform{
		OriginX:    georef.OriginX - georef.PixelSizeM/2,
		OriginY:    georef.OriginY + float64(gridHeight-1)*georef.PixelSizeM + georef.PixelSizeM/2,
		PixelSizeX: georef.PixelSizeM,
		PixelSizeY: -georef.PixelSizeM,
	}, nil
}

// InferGeoTransformFromReceivers derives the grid geo-transform from receiver coordinates.
// It expects receivers ordered row-major (Y ascending, X ascending within row).
//
// It is the fallback, not the source of truth: a run written since the raster
// sidecar carried a georeference uses GeoTransformFromGeoreference instead.
// This exists for the sidecars written before it did, and it cannot tell a
// grid from a scatter of receivers that happens to have the right count.
func InferGeoTransformFromReceivers(xs []float64, ys []float64, gridWidth int, gridHeight int) (GeoTransform, error) {
	if len(xs) == 0 || len(ys) == 0 {
		return GeoTransform{}, errors.New("no receiver coordinates to infer geo-transform")
	}

	if gridWidth <= 0 || gridHeight <= 0 {
		return GeoTransform{}, errors.New("grid dimensions must be positive")
	}

	if len(xs) != gridWidth*gridHeight || len(ys) != gridWidth*gridHeight {
		return GeoTransform{}, fmt.Errorf("coordinate count (%d) does not match grid dimensions %dx%d", len(xs), gridWidth, gridHeight)
	}

	// Find extent from all coordinates.
	minX, maxX := xs[0], xs[0]
	minY, maxY := ys[0], ys[0]

	for _, x := range xs {
		if x < minX {
			minX = x
		}

		if x > maxX {
			maxX = x
		}
	}

	for _, y := range ys {
		if y < minY {
			minY = y
		}

		if y > maxY {
			maxY = y
		}
	}

	// Compute pixel size from extent and grid dimensions.
	var pixelSizeX, pixelSizeY float64

	if gridWidth > 1 {
		pixelSizeX = (maxX - minX) / float64(gridWidth-1)
	} else {
		pixelSizeX = 1.0
	}

	if gridHeight > 1 {
		pixelSizeY = (maxY - minY) / float64(gridHeight-1)
	} else {
		pixelSizeY = 1.0
	}

	// Origin is top-left corner, offset by half a pixel from the first receiver center.
	originX := minX - pixelSizeX/2
	originY := maxY + pixelSizeY/2

	return GeoTransform{
		OriginX:    originX,
		OriginY:    originY,
		PixelSizeX: pixelSizeX,
		PixelSizeY: -pixelSizeY, // negative for south-pointing rows
	}, nil
}
