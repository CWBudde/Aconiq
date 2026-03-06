package geo

import (
	"fmt"
	"strconv"
	"strings"
)

// CRSKind classifies whether a CRS is geographic or projected.
type CRSKind string

const (
	CRSKindUnknown    CRSKind = "unknown"
	CRSKindGeographic CRSKind = "geographic"
	CRSKindProjected  CRSKind = "projected"
)

// CRS identifies a coordinate reference system (for example EPSG:4326).
type CRS struct {
	ID   string  `json:"id"`
	Kind CRSKind `json:"kind"`
}

// ParseCRS parses a CRS identifier and determines a conservative kind.
func ParseCRS(value string) (CRS, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	if trimmed == "" {
		return CRS{}, fmt.Errorf("crs id is required")
	}

	if strings.HasPrefix(trimmed, "EPSG:") {
		codeText := strings.TrimPrefix(trimmed, "EPSG:")
		code, err := strconv.Atoi(codeText)
		if err != nil {
			return CRS{}, fmt.Errorf("invalid EPSG code %q", codeText)
		}

		return CRS{ID: fmt.Sprintf("EPSG:%d", code), Kind: classifyEPSG(code)}, nil
	}

	if strings.HasPrefix(trimmed, "WKT:") {
		return CRS{ID: trimmed, Kind: CRSKindUnknown}, nil
	}

	return CRS{}, fmt.Errorf("unsupported CRS format %q", value)
}

func classifyEPSG(code int) CRSKind {
	switch code {
	case 4326, 4258:
		return CRSKindGeographic
	default:
		if code >= 2000 {
			return CRSKindProjected
		}
		return CRSKindUnknown
	}
}

// TransformStep transforms points between CRS stages.
type TransformStep interface {
	Name() string
	TransformPoint(Point2D) (Point2D, error)
}

// TransformPipeline maps coordinates from ImportCRS into ProjectCRS.
type TransformPipeline struct {
	ProjectCRS CRS
	ImportCRS  CRS
	Steps      []TransformStep
}

// BuildTransformPipeline returns a transform plan.
// Phase 5 baseline supports identity transforms and explicit errors otherwise.
func BuildTransformPipeline(projectCRS CRS, importCRS CRS) (TransformPipeline, error) {
	if projectCRS.ID == "" || importCRS.ID == "" {
		return TransformPipeline{}, fmt.Errorf("project and import CRS must both be set")
	}

	pipeline := TransformPipeline{
		ProjectCRS: projectCRS,
		ImportCRS:  importCRS,
		Steps:      make([]TransformStep, 0, 1),
	}

	if projectCRS.ID == importCRS.ID {
		pipeline.Steps = append(pipeline.Steps, IdentityTransform{})
		return pipeline, nil
	}

	return TransformPipeline{}, fmt.Errorf("transform pipeline not implemented for %s -> %s", importCRS.ID, projectCRS.ID)
}

// ApplyPoint applies all transform steps to one coordinate.
func (p TransformPipeline) ApplyPoint(point Point2D) (Point2D, error) {
	if !point.IsFinite() {
		return Point2D{}, fmt.Errorf("point must contain finite coordinates")
	}

	result := point
	for _, step := range p.Steps {
		var err error
		result, err = step.TransformPoint(result)
		if err != nil {
			return Point2D{}, fmt.Errorf("%s: %w", step.Name(), err)
		}
	}

	return result, nil
}

// IdentityTransform is a no-op transform step.
type IdentityTransform struct{}

func (IdentityTransform) Name() string { return "identity" }

func (IdentityTransform) TransformPoint(point Point2D) (Point2D, error) {
	if !point.IsFinite() {
		return Point2D{}, fmt.Errorf("point must contain finite coordinates")
	}
	return point, nil
}
