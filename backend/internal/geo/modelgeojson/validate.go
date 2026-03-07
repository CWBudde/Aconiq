package modelgeojson

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

type point2 struct {
	x float64
	y float64
}

// Validate applies schema and geometry checks to the normalized model.
func Validate(model Model) ValidationReport {
	report := ValidationReport{
		Valid:     true,
		Errors:    make([]ValidationIssue, 0),
		Warnings:  make([]ValidationIssue, 0),
		CheckedAt: time.Now().UTC(),
	}

	if len(model.Features) == 0 {
		addError(&report, "model.empty", "", "feature collection contains no features")
		report.Valid = false

		return report
	}

	ids := make(map[string]struct{}, len(model.Features))
	allPoints := make([]point2, 0, 256)

	for i, feature := range model.Features {
		fid := strings.TrimSpace(feature.ID)
		if fid == "" {
			addError(&report, "feature.id.required", "", fmt.Sprintf("feature[%d] is missing required id", i))
		} else {
			if _, exists := ids[fid]; exists {
				addError(&report, "feature.id.duplicate", fid, "feature id must be unique")
			}

			ids[fid] = struct{}{}
		}

		points := validateFeature(feature, &report)
		allPoints = append(allPoints, points...)
	}

	validateCRSPlausibility(model.ProjectCRS, allPoints, &report)

	report.Valid = len(report.Errors) == 0

	return report
}

func validateFeature(feature Feature, report *ValidationReport) []point2 {
	id := feature.ID
	kind := strings.ToLower(strings.TrimSpace(feature.Kind))
	geomType := strings.TrimSpace(feature.GeometryType)

	switch kind {
	case "source":
		sourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if sourceType == "" {
			addError(report, "source.type.required", id, "source feature requires source_type (point|line|area)")
		} else if !isOneOf(sourceType, "point", "line", "area") {
			addError(report, "source.type.invalid", id, "source_type must be one of point|line|area")
		} else if !geometryCompatibleWithSourceType(geomType, sourceType) {
			addError(report, "source.geometry.mismatch", id, fmt.Sprintf("geometry type %s does not match source_type %s", geomType, sourceType))
		}
	case "building":
		if feature.HeightM == nil {
			addError(report, "building.height.required", id, "building feature requires height_m")
		} else if *feature.HeightM <= 0 {
			addError(report, "building.height.invalid", id, "building height_m must be > 0")
		}

		if !isOneOf(geomType, "Polygon", "MultiPolygon") {
			addError(report, "building.geometry.invalid", id, "building geometry must be Polygon or MultiPolygon")
		}
	case "barrier":
		if feature.HeightM == nil {
			addError(report, "barrier.height.required", id, "barrier feature requires height_m")
		} else if *feature.HeightM <= 0 {
			addError(report, "barrier.height.invalid", id, "barrier height_m must be > 0")
		}

		if !isOneOf(geomType, "LineString", "MultiLineString") {
			addError(report, "barrier.geometry.invalid", id, "barrier geometry must be LineString or MultiLineString")
		}
	default:
		addError(report, "feature.kind.invalid", id, "kind must be one of source|building|barrier")
	}

	points, ok := validateGeometry(feature, report)
	if !ok {
		return nil
	}

	return points
}

func validateGeometry(feature Feature, report *ValidationReport) ([]point2, bool) {
	id := feature.ID
	geomType := feature.GeometryType
	coords := feature.Coordinates

	switch geomType {
	case "Point":
		p, err := parsePoint(coords)
		if err != nil {
			addError(report, "geometry.point.invalid", id, err.Error())
			return nil, false
		}

		return []point2{p}, true
	case "MultiPoint":
		rawPoints, ok := coords.([]any)
		if !ok || len(rawPoints) == 0 {
			addError(report, "geometry.multipoint.invalid", id, "MultiPoint coordinates must be a non-empty array")
			return nil, false
		}

		points := make([]point2, 0, len(rawPoints))
		for _, raw := range rawPoints {
			p, err := parsePoint(raw)
			if err != nil {
				addError(report, "geometry.multipoint.invalid", id, err.Error())
				return nil, false
			}

			points = append(points, p)
		}

		return points, true
	case "LineString":
		line, err := parseLineString(coords)
		if err != nil {
			addError(report, "geometry.linestring.invalid", id, err.Error())
			return nil, false
		}

		if hasSelfIntersection(line, false) {
			addError(report, "geometry.linestring.self_intersection", id, "LineString has self-intersections")
		}

		return line, true
	case "MultiLineString":
		rawLines, ok := coords.([]any)
		if !ok || len(rawLines) == 0 {
			addError(report, "geometry.multilinestring.invalid", id, "MultiLineString coordinates must be a non-empty array")
			return nil, false
		}

		points := make([]point2, 0, len(rawLines)*2)
		for _, rawLine := range rawLines {
			line, err := parseLineString(rawLine)
			if err != nil {
				addError(report, "geometry.multilinestring.invalid", id, err.Error())
				return nil, false
			}

			if hasSelfIntersection(line, false) {
				addError(report, "geometry.multilinestring.self_intersection", id, "MultiLineString member has self-intersections")
			}

			points = append(points, line...)
		}

		return points, true
	case "Polygon":
		rings, err := parsePolygon(coords)
		if err != nil {
			addError(report, "geometry.polygon.invalid", id, err.Error())
			return nil, false
		}

		points := make([]point2, 0)

		for idx, ring := range rings {
			if hasSelfIntersection(ring, true) {
				addError(report, "geometry.polygon.self_intersection", id, fmt.Sprintf("polygon ring %d has self-intersections", idx))
			}

			points = append(points, ring...)
		}

		return points, true
	case "MultiPolygon":
		rawPolygons, ok := coords.([]any)
		if !ok || len(rawPolygons) == 0 {
			addError(report, "geometry.multipolygon.invalid", id, "MultiPolygon coordinates must be a non-empty array")
			return nil, false
		}

		points := make([]point2, 0)

		for polyIdx, rawPoly := range rawPolygons {
			rings, err := parsePolygon(rawPoly)
			if err != nil {
				addError(report, "geometry.multipolygon.invalid", id, err.Error())
				return nil, false
			}

			for ringIdx, ring := range rings {
				if hasSelfIntersection(ring, true) {
					addError(report, "geometry.multipolygon.self_intersection", id, fmt.Sprintf("multipolygon polygon %d ring %d has self-intersections", polyIdx, ringIdx))
				}

				points = append(points, ring...)
			}
		}

		return points, true
	default:
		addError(report, "geometry.type.unsupported", id, fmt.Sprintf("unsupported geometry type %q", geomType))
		return nil, false
	}
}

func parsePoint(value any) (point2, error) {
	raw, ok := value.([]any)
	if !ok || len(raw) < 2 {
		return point2{}, errors.New("point coordinates must be [x,y]")
	}

	x, ok := asFiniteFloat(raw[0])
	if !ok {
		return point2{}, errors.New("point x must be finite number")
	}

	y, ok := asFiniteFloat(raw[1])
	if !ok {
		return point2{}, errors.New("point y must be finite number")
	}

	return point2{x: x, y: y}, nil
}

func parseLineString(value any) ([]point2, error) {
	raw, ok := value.([]any)
	if !ok || len(raw) < 2 {
		return nil, errors.New("LineString must contain at least two points")
	}

	line := make([]point2, 0, len(raw))
	for _, item := range raw {
		p, err := parsePoint(item)
		if err != nil {
			return nil, err
		}

		line = append(line, p)
	}

	return line, nil
}

func parsePolygon(value any) ([][]point2, error) {
	rawRings, ok := value.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, errors.New("Polygon must contain at least one ring")
	}

	rings := make([][]point2, 0, len(rawRings))
	for ringIdx, rawRing := range rawRings {
		ring, err := parseLineString(rawRing)
		if err != nil {
			return nil, fmt.Errorf("ring %d: %w", ringIdx, err)
		}

		if len(ring) < 4 {
			return nil, fmt.Errorf("ring %d must contain at least 4 coordinates", ringIdx)
		}

		if !pointsEqual(ring[0], ring[len(ring)-1]) {
			return nil, fmt.Errorf("ring %d is not closed", ringIdx)
		}

		rings = append(rings, ring)
	}

	return rings, nil
}

func validateCRSPlausibility(projectCRS string, points []point2, report *ValidationReport) {
	if len(points) == 0 {
		addError(report, "crs.no_coordinates", "", "no coordinates available for CRS plausibility check")
		return
	}

	crs := strings.ToUpper(strings.TrimSpace(projectCRS))
	if crs == "" {
		addWarning(report, "crs.missing", "", "project CRS is empty; plausibility checks are limited")
		return
	}

	minX, minY := points[0].x, points[0].y

	maxX, maxY := minX, minY
	for _, p := range points[1:] {
		if p.x < minX {
			minX = p.x
		}

		if p.x > maxX {
			maxX = p.x
		}

		if p.y < minY {
			minY = p.y
		}

		if p.y > maxY {
			maxY = p.y
		}
	}

	isGeographic := strings.Contains(crs, "4326") || strings.Contains(crs, "4258") || strings.Contains(crs, "WGS84")
	insideLonLatRange := minX >= -180 && maxX <= 180 && minY >= -90 && maxY <= 90

	if isGeographic {
		if !insideLonLatRange {
			addError(report, "crs.range.mismatch", "", fmt.Sprintf("CRS %s expects lon/lat range, but bounds are [%.3f, %.3f] x [%.3f, %.3f]", crs, minX, maxX, minY, maxY))
		}

		return
	}

	if insideLonLatRange {
		addWarning(report, "crs.possible_mismatch", "", fmt.Sprintf("CRS %s appears projected, but all coordinates are in lon/lat-like range", crs))
	}
}

func hasSelfIntersection(points []point2, closed bool) bool {
	if len(points) < 4 {
		return false
	}

	segmentCount := len(points) - 1
	for i := range segmentCount {
		a1 := points[i]

		a2 := points[i+1]
		for j := i + 1; j < segmentCount; j++ {
			if areAdjacentSegments(i, j, segmentCount, closed) {
				continue
			}

			b1 := points[j]

			b2 := points[j+1]
			if segmentsIntersect(a1, a2, b1, b2) {
				return true
			}
		}
	}

	return false
}

func areAdjacentSegments(i int, j int, segmentCount int, closed bool) bool {
	if i == j {
		return true
	}

	if j == i+1 {
		return true
	}

	if closed && i == 0 && j == segmentCount-1 {
		return true
	}

	return false
}

func segmentsIntersect(a, b, c, d point2) bool {
	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)

	if o1 != o2 && o3 != o4 {
		return true
	}

	if o1 == 0 && onSegment(a, c, b) {
		return true
	}

	if o2 == 0 && onSegment(a, d, b) {
		return true
	}

	if o3 == 0 && onSegment(c, a, d) {
		return true
	}

	if o4 == 0 && onSegment(c, b, d) {
		return true
	}

	return false
}

func orientation(a, b, c point2) int {
	value := (b.y-a.y)*(c.x-b.x) - (b.x-a.x)*(c.y-b.y)

	const epsilon = 1e-9
	switch {
	case math.Abs(value) <= epsilon:
		return 0
	case value > 0:
		return 1
	default:
		return 2
	}
}

func onSegment(a, b, c point2) bool {
	const epsilon = 1e-9
	return b.x <= math.Max(a.x, c.x)+epsilon && b.x+epsilon >= math.Min(a.x, c.x) && b.y <= math.Max(a.y, c.y)+epsilon && b.y+epsilon >= math.Min(a.y, c.y)
}

func pointsEqual(a, b point2) bool {
	const epsilon = 1e-9
	return math.Abs(a.x-b.x) <= epsilon && math.Abs(a.y-b.y) <= epsilon
}

func asFiniteFloat(value any) (float64, bool) {
	v, ok := value.(float64)
	if !ok {
		return 0, false
	}

	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}

	return v, true
}

func geometryCompatibleWithSourceType(geometryType string, sourceType string) bool {
	switch sourceType {
	case "point":
		return isOneOf(geometryType, "Point", "MultiPoint")
	case "line":
		return isOneOf(geometryType, "LineString", "MultiLineString")
	case "area":
		return isOneOf(geometryType, "Polygon", "MultiPolygon")
	default:
		return false
	}
}

func isOneOf(value string, options ...string) bool {
	return slices.Contains(options, value)
}

func addError(report *ValidationReport, code string, featureID string, message string) {
	report.Errors = append(report.Errors, ValidationIssue{
		Level:     "error",
		Code:      code,
		FeatureID: featureID,
		Message:   message,
	})
}

func addWarning(report *ValidationReport, code string, featureID string, message string) {
	report.Warnings = append(report.Warnings, ValidationIssue{
		Level:     "warning",
		Code:      code,
		FeatureID: featureID,
		Message:   message,
	})
}
