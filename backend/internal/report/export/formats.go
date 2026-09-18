package export

import (
	"errors"
	"fmt"
	"slices"
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

	// FormatCOG exports raster results as Cloud Optimized GeoTIFF with tiles and overviews.
	FormatCOG Format = "cog"
)

// AllFormats lists every supported export format.
var AllFormats = []Format{
	FormatGeoTIFF,
	FormatCOG,
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
			Format:      FormatCOG,
			Label:       "Cloud Optimized GeoTIFF",
			Description: "Raster results as Cloud Optimized GeoTIFF with tiles and overview pyramids",
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
	return slices.Contains(AllFormats, f)
}

func validFormatList() string {
	names := make([]string, 0, len(AllFormats))
	for _, f := range AllFormats {
		names = append(names, string(f))
	}

	return strings.Join(names, ", ")
}
