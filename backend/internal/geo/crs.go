package geo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/wroge/wgs84"
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
		return CRS{}, errors.New("crs id is required")
	}

	if after, ok := strings.CutPrefix(trimmed, "EPSG:"); ok {
		codeText := after

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
	TransformPoint(p Point2D) (Point2D, error)
}

// TransformPipeline maps coordinates from ImportCRS into ProjectCRS.
type TransformPipeline struct {
	ProjectCRS CRS
	ImportCRS  CRS
	Steps      []TransformStep
}

// EPSGCode extracts the numeric EPSG code from a CRS, or returns 0 if not an EPSG CRS.
func (c CRS) EPSGCode() int {
	after, ok := strings.CutPrefix(c.ID, "EPSG:")
	if !ok {
		return 0
	}

	code, err := strconv.Atoi(after)
	if err != nil {
		return 0
	}

	return code
}

// supportedEPSGCodes lists all EPSG codes for which we can build transforms.
var supportedEPSGCodes = map[int]bool{
	4326:  true, // WGS84 geographic
	4258:  true, // ETRS89 geographic
	25832: true, // ETRS89 / UTM zone 32N
	25833: true, // ETRS89 / UTM zone 33N
	25831: true, // ETRS89 / UTM zone 31N
	25834: true, // ETRS89 / UTM zone 34N
	31466: true, // DHDN / 3-degree Gauss-Kruger zone 2
	31467: true, // DHDN / 3-degree Gauss-Kruger zone 3
	31468: true, // DHDN / 3-degree Gauss-Kruger zone 4
	31469: true, // DHDN / 3-degree Gauss-Kruger zone 5
	32632: true, // WGS84 / UTM zone 32N
	32633: true, // WGS84 / UTM zone 33N
	3857:  true, // WGS84 / Pseudo-Mercator (Web Mercator)
}

// utmScale is the scale factor on the central meridian shared by every UTM zone.
const utmScale = 0.9996

// epsgToCRS maps an EPSG code to the wroge/wgs84 CoordinateReferenceSystem.
//
// Every transverse-Mercator CRS keeps the library's datum, Helmert shift and
// zone Area predicate but swaps in this package's TransverseMercator, because
// the library's inverse is wrong by metres — see TransverseMercator's doc
// comment. EPSG:3857 and the two geographic systems use a different projection
// and are untouched.
func epsgToCRS(code int) (wgs84.CoordinateReferenceSystem, error) {
	switch code {
	case 4326:
		return wgs84.LonLat(), nil
	case 4258:
		return wgs84.ETRS89().LonLat(), nil
	case 25831, 25832, 25833, 25834:
		zone := float64(code - 25800)
		return utmWithExactProjection(wgs84.ETRS89UTM(zone), zone), nil
	case 31466, 31467, 31468, 31469:
		// DHDN 3-degree Gauss-Krüger: zone 2..5, central meridian 3°·zone,
		// unit scale, and the zone number prefixed onto the false easting.
		return gaussKrugerWithExactProjection(float64(code - 31464)), nil
	case 32632, 32633:
		zone := float64(code - 32600)
		return utmWithExactProjection(wgs84.UTM(zone, true), zone), nil
	case 3857:
		return wgs84.WebMercator(), nil
	default:
		return nil, fmt.Errorf("unsupported EPSG code %d", code)
	}
}

// utmWithExactProjection restates the UTM parameters wgs84.UTM and
// wgs84.ETRS89UTM pass to Datum.TransverseMercator (reference.go:47 and :62),
// for northern-hemisphere zones only — which is all this package supports.
func utmWithExactProjection(crs wgs84.ProjectedReferenceSystem, zone float64) wgs84.ProjectedReferenceSystem {
	return withExactTransverseMercator(crs, zone*6-183, 0, utmScale, 500000, 0)
}

// gaussKrugerWithExactProjection restates the parameters wgs84.DHDN2001GK
// passes to Datum.TransverseMercator (reference.go:132).
func gaussKrugerWithExactProjection(zone float64) wgs84.ProjectedReferenceSystem {
	return withExactTransverseMercator(
		wgs84.DHDN2001GK(zone), zone*3, 0, 1, zone*1e6+500000, 0,
	)
}

// withExactTransverseMercator replaces a projected CRS's projection with this
// package's Krüger series, leaving Datum and Area alone.
func withExactTransverseMercator(
	crs wgs84.ProjectedReferenceSystem,
	lonOrigin, latOrigin, scale, falseEasting, falseNorthing float64,
) wgs84.ProjectedReferenceSystem {
	crs.Projection = TransverseMercator{
		LonOrigin:     lonOrigin,
		LatOrigin:     latOrigin,
		Scale:         scale,
		FalseEasting:  falseEasting,
		FalseNorthing: falseNorthing,
	}

	return crs
}

// IsSupportedEPSG reports whether the given EPSG code can be used in transforms.
func IsSupportedEPSG(code int) bool {
	return supportedEPSGCodes[code]
}

// SupportedEPSGCodes returns all EPSG codes for which transforms are available.
func SupportedEPSGCodes() []int {
	codes := make([]int, 0, len(supportedEPSGCodes))
	for code := range supportedEPSGCodes {
		codes = append(codes, code)
	}

	return codes
}

// BuildTransformPipeline returns a transform plan for converting coordinates
// from importCRS to projectCRS. Supports identity transforms and EPSG-to-EPSG
// transforms for all codes listed in supportedEPSGCodes.
func BuildTransformPipeline(projectCRS CRS, importCRS CRS) (TransformPipeline, error) {
	if projectCRS.ID == "" || importCRS.ID == "" {
		return TransformPipeline{}, errors.New("project and import CRS must both be set")
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

	fromCode := importCRS.EPSGCode()
	toCode := projectCRS.EPSGCode()

	if fromCode == 0 || toCode == 0 {
		return TransformPipeline{}, fmt.Errorf("transform pipeline requires EPSG codes, got %s -> %s", importCRS.ID, projectCRS.ID)
	}

	fromCRS, err := epsgToCRS(fromCode)
	if err != nil {
		return TransformPipeline{}, fmt.Errorf("source CRS: %w", err)
	}

	toCRS, err := epsgToCRS(toCode)
	if err != nil {
		return TransformPipeline{}, fmt.Errorf("target CRS: %w", err)
	}

	pipeline.Steps = append(pipeline.Steps, &EPSGTransform{
		fromCode: fromCode,
		toCode:   toCode,
		fn:       wgs84.Transform(fromCRS, toCRS),
	})

	return pipeline, nil
}

// ApplyPoint applies all transform steps to one coordinate.
func (p TransformPipeline) ApplyPoint(point Point2D) (Point2D, error) {
	if !point.IsFinite() {
		return Point2D{}, errors.New("point must contain finite coordinates")
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
		return Point2D{}, errors.New("point must contain finite coordinates")
	}

	return point, nil
}

// EPSGTransform converts coordinates between two EPSG coordinate reference systems
// using the wroge/wgs84 library (pure Go, includes Helmert datum shifts).
//
// Uses wgs84.Transform (not SafeTransform) to allow cross-zone transforms which
// are geometrically valid. Input/output finiteness is checked explicitly.
type EPSGTransform struct {
	fromCode int
	toCode   int
	fn       wgs84.Func
}

func (t *EPSGTransform) Name() string {
	return fmt.Sprintf("EPSG:%d -> EPSG:%d", t.fromCode, t.toCode)
}

func (t *EPSGTransform) TransformPoint(point Point2D) (Point2D, error) {
	if !point.IsFinite() {
		return Point2D{}, errors.New("point must contain finite coordinates")
	}

	// wgs84 convention: first arg = easting/lon, second = northing/lat, third = height.
	outX, outY, _ := t.fn(point.X, point.Y, 0)

	result := Point2D{X: outX, Y: outY}
	if !result.IsFinite() {
		return Point2D{}, fmt.Errorf("transform EPSG:%d -> EPSG:%d produced non-finite result for (%.6f, %.6f)",
			t.fromCode, t.toCode, point.X, point.Y)
	}

	return result, nil
}
