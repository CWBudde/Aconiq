// Package citygmlimport reads CityGML files and converts building data
// into the project model format using the go-citygml library.
package citygmlimport

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/cwbudde/go-citygml/citygml"
	"github.com/cwbudde/go-citygml/helpers"
	"github.com/cwbudde/go-citygml/types"
)

// SkipReason describes why a building was excluded from the import.
type SkipReason string

const (
	SkipNoHeight       SkipReason = "no height available"
	SkipInvalidHeight  SkipReason = "height is zero, NaN, or Inf"
	SkipNoFootprint    SkipReason = "no footprint geometry"
	SkipDegeneratePoly SkipReason = "footprint has fewer than 4 points"
)

// SkippedBuilding records a building that was excluded during import.
type SkippedBuilding struct {
	ID     string     `json:"id"`
	Reason SkipReason `json:"reason"`
}

// ImportReport summarises the outcome of a CityGML import.
type ImportReport struct {
	Total    int               `json:"total"`
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"`
	Details  []SkippedBuilding `json:"skipped_buildings,omitempty"`
}

// ReadResult holds the result of reading a CityGML document.
type ReadResult struct {
	Collection modelgeojson.FeatureCollection
	EPSGCode   int // 0 if CRS could not be determined
	Report     ImportReport
}

// Read reads a CityGML document and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func Read(data []byte) (modelgeojson.FeatureCollection, error) {
	result, err := ReadWithCRS(data)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	return result.Collection, nil
}

// ReadWithCRS reads a CityGML document and also extracts the CRS from the document's srsName.
func ReadWithCRS(data []byte) (ReadResult, error) {
	doc, err := citygml.Read(bytes.NewReader(data), citygml.Options{})
	if err != nil {
		return ReadResult{}, fmt.Errorf("citygml: %w", err)
	}

	features := make([]modelgeojson.GeoJSONFeature, 0, len(doc.Buildings))
	report := ImportReport{Total: len(doc.Buildings)}

	for i := range doc.Buildings {
		b := &doc.Buildings[i]

		feature, reason := buildingToFeature(b, i)
		if reason == "" {
			features = append(features, feature)
		} else {
			id := strings.TrimSpace(b.ID)
			if id == "" {
				id = fmt.Sprintf("citygml-building-%03d", i)
			}

			report.Details = append(report.Details, SkippedBuilding{
				ID:     id,
				Reason: reason,
			})
		}
	}

	report.Imported = len(features)
	report.Skipped = report.Total - report.Imported

	if len(features) == 0 {
		return ReadResult{Report: report}, errors.New("citygml: no supported building features found")
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     "FeatureCollection",
			Features: features,
		},
		EPSGCode: doc.CRS.Code,
		Report:   report,
	}, nil
}

// buildingToFeature converts a CityGML building to a GeoJSON feature.
// Returns the feature and an empty SkipReason on success, or a zero feature
// and the reason on failure.
func buildingToFeature(b *types.Building, index int) (modelgeojson.GeoJSONFeature, SkipReason) {
	// Get effective height.
	height, hasHeight := helpers.BuildingHeight(b)
	if !hasHeight {
		return modelgeojson.GeoJSONFeature{}, SkipNoHeight
	}

	if !(height > 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return modelgeojson.GeoJSONFeature{}, SkipInvalidHeight
	}

	// Get footprint polygon.
	if b.Footprint == nil {
		return modelgeojson.GeoJSONFeature{}, SkipNoFootprint
	}

	if len(b.Footprint.Exterior.Points) < 4 {
		return modelgeojson.GeoJSONFeature{}, SkipDegeneratePoly
	}

	// Build ID.
	id := strings.TrimSpace(b.ID)
	if id == "" {
		id = fmt.Sprintf("citygml-building-%03d", index)
	}

	// Convert footprint to GeoJSON coordinates.
	coords := make([]any, 0, len(b.Footprint.Exterior.Points))
	for _, pt := range b.Footprint.Exterior.Points {
		coords = append(coords, []any{pt.X, pt.Y})
	}

	props := map[string]any{
		"id":                id,
		"kind":              "building",
		"height_m":          height,
		"import_format":     "citygml",
		"citygml_source_id": id,
	}

	if b.Class != "" {
		props["citygml_class"] = b.Class
	}

	if b.Function != "" {
		props["citygml_function"] = b.Function
	}

	if b.Usage != "" {
		props["citygml_usage"] = b.Usage
	}

	if b.LoD != "" {
		props["citygml_lod"] = string(b.LoD)
	}

	return modelgeojson.GeoJSONFeature{
		Type:       "Feature",
		Properties: props,
		Geometry: modelgeojson.Geometry{
			Type:        "Polygon",
			Coordinates: []any{coords},
		},
	}, ""
}
