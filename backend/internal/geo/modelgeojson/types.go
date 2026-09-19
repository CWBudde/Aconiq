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
//
// FeatureKindCalcArea is the calculation area a user draws on the map: the
// extent the automatic receiver grid is built over, instead of the extent of
// whatever happens to emit. It is a model feature rather than a run setting so
// that it travels through the same file as everything else — reprojected by
// NormalizeWithCRS, hashed into the run's recorded inputs, and honoured
// identically by `aconiq run` and by the local API.
//
// Multi-word kinds are hyphenated, matching the enum values elsewhere in this
// codebase (`auto-grid`, `test-fixture`); snake_case is reserved for property
// names (`source_type`, `height_m`).
//
// FeatureKindGroundZone is a ground-category area: a polygon carrying a
// `ground_factor` in [0,1], from which ISO 9613-2 resolves G for the source,
// middle and receiver regions of each path instead of reading one global
// number three times. Like calc-area it is a footprint on the ground rather
// than an object sound travels around, so it carries no height_m.
const (
	FeatureKindSource     = "source"
	FeatureKindBuilding   = "building"
	FeatureKindBarrier    = "barrier"
	FeatureKindReceiver   = "receiver"
	FeatureKindCalcArea   = "calc-area"
	FeatureKindGroundZone = "ground-zone"
)

// FeatureKinds is every kind schema v1 accepts. The validator builds its
// rejection message from this slice rather than repeating the spellings, so the
// enum has one source.
var FeatureKinds = []string{
	FeatureKindSource,
	FeatureKindBuilding,
	FeatureKindBarrier,
	FeatureKindReceiver,
	FeatureKindCalcArea,
	FeatureKindGroundZone,
}

// Property names schema v1 reads off a feature, for the kinds whose payload is
// not a typed field on Feature.
const (
	// PropertyGroundFactor is a ground zone's normalized ground factor G,
	// 0 for acoustically hard and 1 for porous ground.
	PropertyGroundFactor = "ground_factor"
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
