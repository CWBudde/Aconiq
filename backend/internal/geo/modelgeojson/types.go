package modelgeojson

import "time"

// Model contains normalized GeoJSON features for the project model layer.
type Model struct {
	SchemaVersion int       `json:"schema_version"`
	ProjectCRS    string    `json:"project_crs"`
	ImportedAt    time.Time `json:"imported_at"`
	SourcePath    string    `json:"source_path,omitempty"`
	Features      []Feature `json:"features"`
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
	SchemaVersion int            `json:"schema_version"`
	ProjectCRS    string         `json:"project_crs"`
	ImportedAt    time.Time      `json:"imported_at"`
	SourcePath    string         `json:"source_path,omitempty"`
	FeatureCount  int            `json:"feature_count"`
	CountsByKind  map[string]int `json:"counts_by_kind"`
	Features      []FeatureDump  `json:"features"`
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
