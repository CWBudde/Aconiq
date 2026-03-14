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

// ReadResult holds the result of reading a CityGML document.
type ReadResult struct {
	Collection modelgeojson.FeatureCollection
	EPSGCode   int // 0 if CRS could not be determined
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

	for i := range doc.Buildings {
		b := &doc.Buildings[i]

		feature, ok := buildingToFeature(b, i)
		if ok {
			features = append(features, feature)
		}
	}

	if len(features) == 0 {
		return ReadResult{}, errors.New("citygml: no supported building features found")
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     "FeatureCollection",
			Features: features,
		},
		EPSGCode: doc.CRS.Code,
	}, nil
}

func buildingToFeature(b *types.Building, index int) (modelgeojson.GeoJSONFeature, bool) {
	// Get effective height.
	height, hasHeight := helpers.BuildingHeight(b)
	if !hasHeight || !(height > 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return modelgeojson.GeoJSONFeature{}, false
	}

	// Get footprint polygon.
	if b.Footprint == nil {
		return modelgeojson.GeoJSONFeature{}, false
	}

	if len(b.Footprint.Exterior.Points) < 4 {
		return modelgeojson.GeoJSONFeature{}, false
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

	return modelgeojson.GeoJSONFeature{
		Type: "Feature",
		Properties: map[string]any{
			"id":                id,
			"kind":              "building",
			"height_m":          height,
			"import_format":     "citygml",
			"citygml_source_id": id,
		},
		Geometry: modelgeojson.Geometry{
			Type:        "Polygon",
			Coordinates: []any{coords},
		},
	}, true
}
