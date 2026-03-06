package modelgeojson

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Normalize decodes raw GeoJSON and maps it into the normalized project model.
func Normalize(data []byte, projectCRS string, sourcePath string) (Model, error) {
	var collection FeatureCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return Model{}, fmt.Errorf("decode geojson: %w", err)
	}

	if collection.Type != "FeatureCollection" {
		return Model{}, fmt.Errorf("expected GeoJSON FeatureCollection, got %q", collection.Type)
	}

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

		features = append(features, Feature{
			ID:           id,
			Kind:         kind,
			SourceType:   sourceType,
			HeightM:      featureHeight,
			GeometryType: geometryType,
			Coordinates:  raw.Geometry.Coordinates,
		})
	}

	return Model{
		SchemaVersion: 1,
		ProjectCRS:    strings.TrimSpace(projectCRS),
		ImportedAt:    time.Now().UTC(),
		SourcePath:    filepath.ToSlash(strings.TrimSpace(sourcePath)),
		Features:      features,
	}, nil
}

// ToFeatureCollection serializes the normalized model back into canonical GeoJSON.
func (m Model) ToFeatureCollection() FeatureCollection {
	features := make([]GeoJSONFeature, 0, len(m.Features))
	for _, f := range m.Features {
		props := map[string]any{
			"id":   f.ID,
			"kind": f.Kind,
		}
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
			GeometryType: f.GeometryType,
		})
	}

	return ModelDump{
		SchemaVersion: m.SchemaVersion,
		ProjectCRS:    m.ProjectCRS,
		ImportedAt:    m.ImportedAt,
		SourcePath:    m.SourcePath,
		FeatureCount:  len(m.Features),
		CountsByKind:  counts,
		Features:      dumpFeatures,
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
