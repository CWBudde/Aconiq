package modelgeojson

import "time"

// TypeFeatureCollection is the GeoJSON `type` member of a FeatureCollection
// object.
const TypeFeatureCollection = "FeatureCollection"

// GeoJSON geometry type tags, as they appear in a geometry object's `type`
// member and in Feature.GeometryType. These are the canonical spellings for
// the whole backend — importers, extractors and validators must use them
// rather than repeating the literals.
const (
	GeometryTypePoint           = "Point"
	GeometryTypeMultiPoint      = "MultiPoint"
	GeometryTypeLineString      = "LineString"
	GeometryTypeMultiLineString = "MultiLineString"
	GeometryTypePolygon         = "Polygon"
	GeometryTypeMultiPolygon    = "MultiPolygon"
)

// Feature kinds accepted by model schema v1 (Feature.Kind).
const (
	FeatureKindSource   = "source"
	FeatureKindBuilding = "building"
	FeatureKindBarrier  = "barrier"
	FeatureKindReceiver = "receiver"
)

// Source geometry classes accepted by model schema v1 (Feature.SourceType).
const (
	SourceTypePoint = "point"
	SourceTypeLine  = "line"
	SourceTypeArea  = "area"
)

// Model contains normalized GeoJSON features for the project model layer.
type Model struct {
	SchemaVersion    int       `json:"schema_version"`
	ProjectCRS       string    `json:"project_crs"`
	ImportCRS        string    `json:"import_crs,omitempty"`
	TransformApplied bool      `json:"transform_applied,omitempty"`
	ImportedAt       time.Time `json:"imported_at"`
	SourcePath       string    `json:"source_path,omitempty"`
	Features         []Feature `json:"features"`
}

// Feature is a normalized model feature derived from raw GeoJSON.
type Feature struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	SourceType   string         `json:"source_type,omitempty"`
	HeightM      *float64       `json:"height_m,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
	GeometryType string         `json:"geometry_type"`
	Coordinates  any            `json:"coordinates"`
}

// ValidationIssue describes one validation finding.
type ValidationIssue struct {
	Level     string `json:"level"`
	Code      string `json:"code"`
	FeatureID string `json:"feature_id,omitempty"`
	Message   string `json:"message"`
}

// ValidationReport captures all validation findings.
type ValidationReport struct {
	Valid     bool              `json:"valid"`
	Errors    []ValidationIssue `json:"errors"`
	Warnings  []ValidationIssue `json:"warnings"`
	CheckedAt time.Time         `json:"checked_at"`
}

func (r ValidationReport) ErrorCount() int {
	return len(r.Errors)
}

func (r ValidationReport) WarningCount() int {
	return len(r.Warnings)
}

// ModelDump is a compact, debug-friendly projection of the normalized model.
type ModelDump struct {
	SchemaVersion    int            `json:"schema_version"`
	ProjectCRS       string         `json:"project_crs"`
	ImportCRS        string         `json:"import_crs,omitempty"`
	TransformApplied bool           `json:"transform_applied,omitempty"`
	ImportedAt       time.Time      `json:"imported_at"`
	SourcePath       string         `json:"source_path,omitempty"`
	FeatureCount     int            `json:"feature_count"`
	CountsByKind     map[string]int `json:"counts_by_kind"`
	Features         []FeatureDump  `json:"features"`
}

// FeatureDump summarizes one normalized feature.
type FeatureDump struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	SourceType   string         `json:"source_type,omitempty"`
	HeightM      *float64       `json:"height_m,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
	GeometryType string         `json:"geometry_type"`
}

// FeatureCollection is a GeoJSON FeatureCollection payload.
type FeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
	CRS      map[string]any   `json:"crs,omitempty"`
}

// GeoJSONFeature is a GeoJSON feature object.
type GeoJSONFeature struct {
	Type       string         `json:"type"`
	ID         any            `json:"id,omitempty"`
	Properties map[string]any `json:"properties"`
	Geometry   Geometry       `json:"geometry"`
}

// Geometry is a GeoJSON geometry object.
type Geometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}
