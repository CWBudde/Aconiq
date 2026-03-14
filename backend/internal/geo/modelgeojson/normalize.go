package modelgeojson

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

// Normalize decodes raw GeoJSON and maps it into the normalized project model.
// If importCRS is non-empty and differs from projectCRS, coordinates are transformed.
func Normalize(data []byte, projectCRS string, sourcePath string) (Model, error) {
	return NormalizeWithCRS(data, projectCRS, "", sourcePath)
}

// buildCRSPipeline builds a transform pipeline if import and project CRS differ.
// Returns nil pipeline when no transform is needed.
func buildCRSPipeline(projCRS, impCRS string) (*geo.TransformPipeline, error) {
	if impCRS == "" || projCRS == "" || strings.EqualFold(impCRS, projCRS) {
		return nil, nil //nolint:nilnil // nil pipeline means no transform needed
	}

	from, err := geo.ParseCRS(impCRS)
	if err != nil {
		return nil, fmt.Errorf("parse import CRS %q: %w", impCRS, err)
	}

	to, err := geo.ParseCRS(projCRS)
	if err != nil {
		return nil, fmt.Errorf("parse project CRS %q: %w", projCRS, err)
	}

	p, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return nil, fmt.Errorf("build CRS transform %s -> %s: %w", impCRS, projCRS, err)
	}

	return &p, nil
}

// NormalizeWithCRS decodes raw GeoJSON and maps it into the normalized project model.
// When importCRS differs from projectCRS, all coordinates are reprojected into the project CRS.
func NormalizeWithCRS(data []byte, projectCRS string, importCRS string, sourcePath string) (Model, error) {
	var collection FeatureCollection

	err := json.Unmarshal(data, &collection)
	if err != nil {
		return Model{}, fmt.Errorf("decode geojson: %w", err)
	}

	if collection.Type != "FeatureCollection" {
		return Model{}, fmt.Errorf("expected GeoJSON FeatureCollection, got %q", collection.Type)
	}

	projCRS := strings.TrimSpace(projectCRS)
	impCRS := strings.TrimSpace(importCRS)

	pipeline, err := buildCRSPipeline(projCRS, impCRS)
	if err != nil {
		return Model{}, err
	}

	transformApplied := pipeline != nil

	features := make([]Feature, 0, len(collection.Features))
	for idx, raw := range collection.Features {
		if raw.Type != "Feature" {
			return Model{}, fmt.Errorf("feature[%d]: expected type Feature, got %q", idx, raw.Type)
		}

		if raw.Properties == nil {
			return Model{}, fmt.Errorf("feature[%d]: properties are required", idx)
		}

		id := featureID(raw)
		kind := normalizeKind(raw.Properties["kind"])

		geometryType := strings.TrimSpace(raw.Geometry.Type)
		if geometryType == "" {
			return Model{}, fmt.Errorf("feature[%d]: geometry type is required", idx)
		}

		sourceType := normalizeSourceType(raw.Properties["source_type"])
		if sourceType == "" && kind == "source" {
			sourceType = inferSourceTypeFromGeometry(geometryType)
		}

		heightM, hasHeight, err := readOptionalNumber(raw.Properties["height_m"])
		if err != nil {
			return Model{}, fmt.Errorf("feature[%d]: invalid height_m: %w", idx, err)
		}

		var featureHeight *float64
		if hasHeight {
			featureHeight = &heightM
		}

		coords := raw.Geometry.Coordinates
		if pipeline != nil {
			coords, err = transformCoordinates(coords, pipeline)
			if err != nil {
				return Model{}, fmt.Errorf("feature[%d] (%s): CRS transform: %w", idx, id, err)
			}
		}

		features = append(features, Feature{
			ID:           id,
			Kind:         kind,
			SourceType:   sourceType,
			HeightM:      featureHeight,
			Properties:   normalizeProperties(raw.Properties),
			GeometryType: geometryType,
			Coordinates:  coords,
		})
	}

	return Model{
		SchemaVersion:    1,
		ProjectCRS:       projCRS,
		ImportCRS:        impCRS,
		TransformApplied: transformApplied,
		ImportedAt:       time.Now().UTC(),
		SourcePath:       filepath.ToSlash(strings.TrimSpace(sourcePath)),
		Features:         features,
	}, nil
}

// transformCoordinates recursively walks GeoJSON coordinate structures and
// applies the CRS transform to each [x, y] pair. Supports all GeoJSON geometry types.
func transformCoordinates(coords any, pipeline *geo.TransformPipeline) (any, error) {
	switch v := coords.(type) {
	case []any:
		if len(v) == 0 {
			return v, nil
		}

		// Check if this is a coordinate pair [x, y] or [x, y, z].
		if isCoordinatePair(v) {
			return transformPoint(v, pipeline)
		}

		// Otherwise it's an array of sub-arrays — recurse.
		result := make([]any, len(v))
		for i, elem := range v {
			transformed, err := transformCoordinates(elem, pipeline)
			if err != nil {
				return nil, err
			}

			result[i] = transformed
		}

		return result, nil
	default:
		return coords, nil
	}
}

// isCoordinatePair checks if a []any looks like a GeoJSON coordinate [number, number, ...].
func isCoordinatePair(v []any) bool {
	if len(v) < 2 {
		return false
	}

	_, xOK := v[0].(float64)
	_, yOK := v[1].(float64)

	return xOK && yOK
}

// transformPoint transforms a single [x, y] or [x, y, z] coordinate.
func transformPoint(v []any, pipeline *geo.TransformPipeline) ([]any, error) {
	x, _ := v[0].(float64)
	y, _ := v[1].(float64)

	out, err := pipeline.ApplyPoint(geo.Point2D{X: x, Y: y})
	if err != nil {
		return nil, fmt.Errorf("point (%.6f, %.6f): %w", x, y, err)
	}

	result := make([]any, len(v))
	result[0] = out.X
	result[1] = out.Y

	// Preserve Z and any additional ordinates unchanged.
	for i := 2; i < len(v); i++ {
		result[i] = v[i]
	}

	return result, nil
}

// ToFeatureCollection serializes the normalized model back into canonical GeoJSON.
func (m Model) ToFeatureCollection() FeatureCollection {
	features := make([]GeoJSONFeature, 0, len(m.Features))
	for _, f := range m.Features {
		props := cloneProperties(f.Properties)
		props["id"] = f.ID

		props["kind"] = f.Kind
		if f.SourceType != "" {
			props["source_type"] = f.SourceType
		}

		if f.HeightM != nil {
			props["height_m"] = *f.HeightM
		}

		features = append(features, GeoJSONFeature{
			Type:       "Feature",
			Properties: props,
			Geometry: Geometry{
				Type:        f.GeometryType,
				Coordinates: f.Coordinates,
			},
		})
	}

	collection := FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
	if m.ProjectCRS != "" {
		collection.CRS = map[string]any{
			"type": "name",
			"properties": map[string]any{
				"name": m.ProjectCRS,
			},
		}
	}

	return collection
}

// ToDump produces a compact JSON-friendly model debug view.
func (m Model) ToDump() ModelDump {
	counts := make(map[string]int)

	dumpFeatures := make([]FeatureDump, 0, len(m.Features))
	for _, f := range m.Features {
		counts[f.Kind]++
		dumpFeatures = append(dumpFeatures, FeatureDump{
			ID:           f.ID,
			Kind:         f.Kind,
			SourceType:   f.SourceType,
			HeightM:      f.HeightM,
			Properties:   cloneProperties(f.Properties),
			GeometryType: f.GeometryType,
		})
	}

	return ModelDump{
		SchemaVersion:    m.SchemaVersion,
		ProjectCRS:       m.ProjectCRS,
		ImportCRS:        m.ImportCRS,
		TransformApplied: m.TransformApplied,
		ImportedAt:       m.ImportedAt,
		SourcePath:       m.SourcePath,
		FeatureCount:     len(m.Features),
		CountsByKind:     counts,
		Features:         dumpFeatures,
	}
}

func featureID(raw GeoJSONFeature) string {
	if value, ok := raw.Properties["id"]; ok {
		if text := stringifyID(value); text != "" {
			return text
		}
	}

	return stringifyID(raw.ID)
}

func stringifyID(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}

		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func normalizeKind(value any) string {
	return strings.ToLower(strings.TrimSpace(stringifyID(value)))
}

func normalizeSourceType(value any) string {
	return strings.ToLower(strings.TrimSpace(stringifyID(value)))
}

func inferSourceTypeFromGeometry(geometryType string) string {
	switch geometryType {
	case "Point", "MultiPoint":
		return "point"
	case "LineString", "MultiLineString":
		return "line"
	case "Polygon", "MultiPolygon":
		return "area"
	default:
		return ""
	}
}

func readOptionalNumber(value any) (float64, bool, error) {
	if value == nil {
		return 0, false, nil
	}

	switch v := value.(type) {
	case float64:
		return v, true, nil
	case int:
		return float64(v), true, nil
	case int64:
		return float64(v), true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false, nil
		}

		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false, err
		}

		return parsed, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported numeric type %T", value)
	}
}

func normalizeProperties(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	props := make(map[string]any, len(raw))
	for key, value := range raw {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}

		props[trimmedKey] = value
	}

	if len(props) == 0 {
		return nil
	}

	return props
}

func cloneProperties(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	props := make(map[string]any, len(raw))
	maps.Copy(props, raw)

	return props
}
