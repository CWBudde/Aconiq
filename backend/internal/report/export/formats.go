package export

import (
	"errors"
	"fmt"
	"strings"
)

// Format identifies an export output format.
type Format string

const (
	// FormatGeoTIFF exports raster results as GeoTIFF with embedded CRS.
	FormatGeoTIFF Format = "geotiff"

	// FormatGeoPackage exports receiver tables and model features as GeoPackage.
	FormatGeoPackage Format = "gpkg"

	// FormatContourGeoJSON exports ISO-band contour lines as GeoJSON.
	FormatContourGeoJSON Format = "contour-geojson"

	// FormatContourGeoPackage exports ISO-band contour lines as GeoPackage.
	FormatContourGeoPackage Format = "contour-gpkg"
)

// AllFormats lists every supported export format.
var AllFormats = []Format{
	FormatGeoTIFF,
	FormatGeoPackage,
	FormatContourGeoJSON,
	FormatContourGeoPackage,
}

// FormatInfo describes one export format for documentation and the format matrix.
type FormatInfo struct {
	Format      Format
	Label       string
	Description string
	Category    string // "raster", "vector", "contour"
	Extension   string
}

// FormatMatrix returns the complete export format matrix.
func FormatMatrix() []FormatInfo {
	return []FormatInfo{
		{
			Format:      FormatGeoTIFF,
			Label:       "GeoTIFF",
			Description: "Raster results as GeoTIFF with embedded CRS metadata, one band per indicator",
			Category:    "raster",
			Extension:   ".tif",
		},
		{
			Format:      FormatGeoPackage,
			Label:       "GeoPackage",
			Description: "Receiver tables as attributed point features in OGC GeoPackage",
			Category:    "vector",
			Extension:   ".gpkg",
		},
		{
			Format:      FormatContourGeoJSON,
			Label:       "Contour GeoJSON",
			Description: "ISO-band contour lines from raster results as GeoJSON FeatureCollection",
			Category:    "contour",
			Extension:   ".geojson",
		},
		{
			Format:      FormatContourGeoPackage,
			Label:       "Contour GeoPackage",
			Description: "ISO-band contour lines from raster results as OGC GeoPackage",
			Category:    "contour",
			Extension:   ".gpkg",
		},
	}
}

// ParseFormats parses a comma-separated format list and validates each entry.
func ParseFormats(input string) ([]Format, error) {
	if input == "" {
		return nil, nil
	}

	parts := strings.Split(input, ",")
	formats := make([]Format, 0, len(parts))
	seen := make(map[Format]struct{}, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		f := Format(strings.ToLower(trimmed))

		if !isValidFormat(f) {
			return nil, fmt.Errorf("unknown export format %q; valid formats: %s", trimmed, validFormatList())
		}

		if _, exists := seen[f]; exists {
			continue
		}

		seen[f] = struct{}{}
		formats = append(formats, f)
	}

	if len(formats) == 0 {
		return nil, errors.New("no valid export formats specified")
	}

	return formats, nil
}

func isValidFormat(f Format) bool {
	for _, valid := range AllFormats {
		if f == valid {
			return true
		}
	}

	return false
}

func validFormatList() string {
	names := make([]string, 0, len(AllFormats))
	for _, f := range AllFormats {
		names = append(names, string(f))
	}

	return strings.Join(names, ", ")
}

// GeoTransform describes the affine mapping from pixel to projected coordinates.
// OriginX/OriginY is the top-left corner of the top-left pixel.
type GeoTransform struct {
	OriginX    float64 // X coordinate of top-left pixel corner
	OriginY    float64 // Y coordinate of top-left pixel corner
	PixelSizeX float64 // pixel width in CRS units (positive = east)
	PixelSizeY float64 // pixel height in CRS units (negative = south)
}

// InferGeoTransformFromReceivers derives the grid geo-transform from receiver coordinates.
// It expects receivers ordered row-major (Y ascending, X ascending within row).
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
