package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/report/contour"
)

// The contour geometry lives in internal/report/contour, and these are the
// names this package used to own.
//
// They are kept, rather than the callers updated, because that package was
// split out so the WebAssembly kernel could reach marching squares without
// linking modernc.org/sqlite through the GeoPackage writer below. That is a
// dependency decision, and it should not also be an API break: `cli` holds a
// dozen call sites, and testdata/raster-parity/geotransform.golden.json pins
// GeoTransformFromGeoreference against a TypeScript mirror that reads it by
// path. Aliases mean the split moved no caller and no fixture.
//
// Type aliases, not definitions — export.GeoTransform *is* contour.GeoTransform,
// so a value crosses between the two packages without a conversion.
type (
	// ContourLine is a single contour at a given dB level.
	ContourLine = contour.Line
	// ContourOptions configures contour generation.
	ContourOptions = contour.Options
	// GeoTransform is the affine mapping from pixel to projected coordinates.
	GeoTransform = contour.GeoTransform
)

// DefaultContourInterval is 5 dB per EU Environmental Noise Directive convention.
const DefaultContourInterval = contour.DefaultInterval

// GenerateContours, GeoTransformFromGeoreference and InferGeoTransformFromReceivers
// are the functions that moved, forwarded by value rather than by a wrapper.
//
// A wrapper would have to either return the inner error unwrapped — which
// `wrapcheck` refuses, correctly, for a normal call — or wrap it, which would
// change the text `cli` already matches on and the words a refusal reaches a
// user in. Neither is right for something whose whole claim is that it is the
// same function, so these are the same function.
//
// GeoTransformFromGeoreference in particular is mirrored in
// frontend/src/map/raster-extent.ts and pinned from both sides by
// testdata/raster-parity/geotransform.golden.json — see formats_parity_test.go
// before changing the arithmetic, which now lives in internal/report/contour.
var (
	GenerateContours               = contour.GenerateContours
	GeoTransformFromGeoreference   = contour.GeoTransformFromGeoreference
	InferGeoTransformFromReceivers = contour.InferGeoTransformFromReceivers
)

// ExportContourGeoJSON writes contour lines as a GeoJSON FeatureCollection.
func ExportContourGeoJSON(path string, contours []ContourLine) error {
	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("create contour geojson directory: %w", err)
	}

	fc := buildContourFeatureCollection(contours)

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal contour geojson: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write contour geojson %s: %w", path, err)
	}

	return nil
}

func buildContourFeatureCollection(contours []ContourLine) map[string]any {
	features := make([]map[string]any, 0, len(contours))

	for _, c := range contours {
		if len(c.Points) < 2 {
			continue
		}

		coords := make([][]float64, len(c.Points))
		for i, pt := range c.Points {
			coords[i] = []float64{pt[0], pt[1]}
		}

		features = append(features, map[string]any{
			"type": "Feature",
			"properties": map[string]any{
				"level_db":  c.Level,
				"band_name": c.BandName,
			},
			"geometry": map[string]any{
				"type":        "LineString",
				"coordinates": coords,
			},
		})
	}

	return map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	}
}
