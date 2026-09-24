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

	var calcAreas calcAreaTracker

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

		calcAreas.observe(feature, fid, &report)

		points := validateFeature(feature, &report)
		allPoints = append(allPoints, points...)
	}

	validateCRSPlausibility(model.ProjectCRS, allPoints, &report)

	report.Valid = len(report.Errors) == 0

	return report
}

// calcAreaTracker enforces the at-most-one calculation area rule. It lives
// outside validateFeature because that function sees one feature at a time, and
// this is the only v1 rule a single feature cannot answer.
//
// Two areas is a model the UI cannot produce, and neither resolution is honest:
// the union bbox covers ground between them that the user drew around, and
// taking the first one makes a run's extent depend on the order features happen
// to sit in the file.
type calcAreaTracker struct {
	seen    bool
	firstID string
}

func (t *calcAreaTracker) observe(feature Feature, featureID string, report *ValidationReport) {
	if featureKind(feature) != FeatureKindCalcArea {
		return
	}

	if t.seen {
		addError(report, "model.calc_area.duplicate", featureID, fmt.Sprintf(
			"model carries more than one %s feature (the first is %q); a run has exactly one receiver grid extent",
			FeatureKindCalcArea, t.firstID,
		))

		return
	}

	t.seen = true
	t.firstID = featureID
}

// featureKind is the feature's kind as the validator compares it. Normalize
// already lowercases and trims, but Validate also runs over models built in
// code, so the comparison does not assume it.
func featureKind(feature Feature) string {
	return strings.ToLower(strings.TrimSpace(feature.Kind))
}

func validateFeature(feature Feature, report *ValidationReport) []point2 {
	id := feature.ID
	kind := featureKind(feature)
	geomType := strings.TrimSpace(feature.GeometryType)

	switch kind {
	case FeatureKindSource:
		validateSourceKind(feature, geomType, report)
	case FeatureKindBuilding:
		validateHeightM(feature.HeightM, id, FeatureKindBuilding, report)

		if !isOneOf(geomType, GeometryTypePolygon, GeometryTypeMultiPolygon) {
			addError(report, "building.geometry.invalid", id, "building geometry must be Polygon or MultiPolygon")
		}
	case FeatureKindBarrier:
		validateHeightM(feature.HeightM, id, FeatureKindBarrier, report)

		if !isOneOf(geomType, GeometryTypeLineString, GeometryTypeMultiLineString) {
			addError(report, "barrier.geometry.invalid", id, "barrier geometry must be LineString or MultiLineString")
		}
	case FeatureKindReceiver:
		validateHeightM(feature.HeightM, id, FeatureKindReceiver, report)

		if geomType != GeometryTypePoint {
			addError(report, "receiver.geometry.invalid", id, "receiver geometry must be Point")
		}
	case FeatureKindGroundZone:
		validateGroundFactor(feature, report)

		// Polygon only, for calc-area's reason: a multi-part zone is a shape
		// the UI cannot draw, and accepting one now fixes how its parts
		// combine before anything needs them to.
		if geomType != GeometryTypePolygon {
			addError(report, "groundzone.geometry.invalid", id, "ground-zone geometry must be Polygon")
		}
	case FeatureKindCalcArea:
		// No height_m check: a calculation area is a footprint on the ground
		// bounding the receiver grid, not an object sound travels around.
		//
		// MultiPolygon is refused rather than reduced to its envelope. A
		// disjoint multi-part area would silently become one bbox spanning the
		// gap between its parts, computing receivers over ground the user drew
		// around. Widening this later is cheap; the reverse is not.
		if geomType != GeometryTypePolygon {
			addError(report, "calcarea.geometry.invalid", id, "calc-area geometry must be Polygon")
		}
	default:
		addError(report, "feature.kind.invalid", id, "kind must be one of "+strings.Join(FeatureKinds, "|"))
	}

	points, ok := validateGeometry(feature, report)
	if !ok {
		return nil
	}

	return points
}

// validateSourceKind applies the source-specific checks in their original order.
func validateSourceKind(feature Feature, geomType string, report *ValidationReport) {
	id := feature.ID

	sourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
	switch {
	case sourceType == "":
		addError(report, "source.type.required", id, "source feature requires source_type (point|line|area)")
	case !isOneOf(sourceType, SourceTypePoint, SourceTypeLine, SourceTypeArea):
		addError(report, "source.type.invalid", id, "source_type must be one of point|line|area")
	case !geometryCompatibleWithSourceType(geomType, sourceType):
		addError(report, "source.geometry.mismatch", id, fmt.Sprintf("geometry type %s does not match source_type %s", geomType, sourceType))
	}
}

// validateGroundFactor checks a ground zone's ground_factor property.
//
// It is required rather than defaulted: a zone whose factor is missing is
// indistinguishable from ground the model says nothing about, and the run
// already has a global fallback for that. Defaulting it here would make a
// typo in the property name read as hard ground.
func validateGroundFactor(feature Feature, report *ValidationReport) {
	id := feature.ID

	raw, present := feature.Properties[PropertyGroundFactor]
	if !present {
		addError(report, "groundzone.factor.required", id,
			"ground-zone feature requires "+PropertyGroundFactor)

		return
	}

	factor, ok := asFiniteFloat(raw)
	if !ok || factor < 0 || factor > 1 {
		addError(report, "groundzone.factor.invalid", id,
			"ground-zone "+PropertyGroundFactor+" must be a finite number within [0,1]")
	}
}

// validateHeightM checks the shared height_m rule for building, barrier and receiver features.
func validateHeightM(heightM *float64, id string, kind string, report *ValidationReport) {
	if heightM == nil {
		addError(report, kind+".height.required", id, kind+" feature requires height_m")
		return
	}

	if *heightM <= 0 {
		addError(report, kind+".height.invalid", id, kind+" height_m must be > 0")
	}
}

func validateGeometry(feature Feature, report *ValidationReport) ([]point2, bool) {
	id := feature.ID
	geomType := feature.GeometryType
	coords := feature.Coordinates

	switch geomType {
	case GeometryTypePoint:
		return validatePointGeometry(coords, id, report)
	case GeometryTypeMultiPoint:
		return validateMultiPointGeometry(coords, id, report)
	case GeometryTypeLineString:
		return validateLineStringGeometry(coords, id, report)
	case GeometryTypeMultiLineString:
		return validateMultiLineStringGeometry(coords, id, report)
	case GeometryTypePolygon:
		return validatePolygonGeometry(coords, id, report)
	case GeometryTypeMultiPolygon:
		return validateMultiPolygonGeometry(coords, id, report)
	default:
		addError(report, "geometry.type.unsupported", id, fmt.Sprintf("unsupported geometry type %q", geomType))
		return nil, false
	}
}

func validatePointGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
	p, err := parsePoint(coords)
	if err != nil {
		addError(report, "geometry.point.invalid", id, err.Error())
		return nil, false
	}

	return []point2{p}, true
}

func validateMultiPointGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
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
}

func validateLineStringGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
	line, err := parseLineString(coords)
	if err != nil {
		addError(report, "geometry.linestring.invalid", id, err.Error())
		return nil, false
	}

	checkSelfIntersection(line, false, "geometry.linestring.self_intersection", id, "LineString has self-intersections", report)

	return line, true
}

func validateMultiLineStringGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
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

		checkSelfIntersection(line, false, "geometry.multilinestring.self_intersection", id, "MultiLineString member has self-intersections", report)

		points = append(points, line...)
	}

	return points, true
}

func validatePolygonGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
	rings, err := parsePolygon(coords)
	if err != nil {
		addError(report, "geometry.polygon.invalid", id, err.Error())
		return nil, false
	}

	points := make([]point2, 0)

	for idx, ring := range rings {
		checkSelfIntersection(ring, true, "geometry.polygon.self_intersection", id, fmt.Sprintf("polygon ring %d has self-intersections", idx), report)

		points = append(points, ring...)
	}

	return points, true
}

func validateMultiPolygonGeometry(coords any, id string, report *ValidationReport) ([]point2, bool) {
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
			checkSelfIntersection(ring, true, "geometry.multipolygon.self_intersection", id,
				fmt.Sprintf("multipolygon polygon %d ring %d has self-intersections", polyIdx, ringIdx), report)

			points = append(points, ring...)
		}
	}

	return points, true
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
		return nil, errors.New("polygon must contain at least one ring")
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

	minX, minY, maxX, maxY := coordinateBounds(points)

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

// coordinateBounds returns minX, minY, maxX, maxY of a non-empty point set.
func coordinateBounds(points []point2) (float64, float64, float64, float64) {
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

	return minX, minY, maxX, maxY
}

// maxSelfIntersectionPoints bounds the exhaustive self-intersection check.
//
// The check compares every pair of segments, so its cost grows with the square
// of the vertex count. 10,000 points is about 5*10^7 segment-pair tests, which
// runs in well under a second; 200,000 points -- an unremarkable size for a
// machine-generated GeoJSON upload -- would be 2*10^10 and would hang the
// import. Geometries above the limit are reported as unchecked rather than
// rejected, so a legitimate very dense line still imports.
const maxSelfIntersectionPoints = 10000

// checkSelfIntersection runs the self-intersection test within its cost bound
// and records the outcome on the report.
//
// `closed` decides the severity as well as the geometry, because the two follow
// from each other. A ring's winding is consumed — point-in-polygon and the
// screening crossing counts both read it — so a footprint that crosses itself
// has no reliable inside, and refusing it is the honest answer. A line's winding
// is consumed by nothing: a road drawn as one way that touches itself is a
// roundabout, not a defect, and refusing it made the OSM import unsaveable over
// geometry it holds correctly. 45 of 2397 ways in a Berlin extract are like
// that, which is an ordinary proportion and not one a reader can fix.
func checkSelfIntersection(points []point2, closed bool, code, id, message string, report *ValidationReport) {
	if len(points) > maxSelfIntersectionPoints {
		addWarning(report, code+".skipped", id,
			fmt.Sprintf("self-intersection check skipped: %d points exceed the limit of %d", len(points), maxSelfIntersectionPoints))

		return
	}

	if !hasSelfIntersection(points, closed) {
		return
	}

	if closed {
		addError(report, code, id, message)

		return
	}

	addWarning(report, code, id, message)
}

// RingSelfIntersects reports whether a closed ring of [x, y] pairs crosses
// itself, by exactly the test Validate refuses a polygon for. An importer that
// would rather drop one broken footprint than have Validate refuse the whole
// model asks this first. Rings above the cost bound Validate applies are
// reported as not intersecting, as Validate itself does.
func RingSelfIntersects(ring [][2]float64) bool {
	if len(ring) > maxSelfIntersectionPoints {
		return false
	}

	points := make([]point2, len(ring))
	for i, p := range ring {
		points[i] = point2{x: p[0], y: p[1]}
	}

	return hasSelfIntersection(points, true)
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

// relativeEpsilon is the tolerance orientation applies to a cross product,
// as a fraction of the magnitudes that produced it. It sits a few orders above
// float64's own rounding, which leaves a genuinely collinear triple (~1e-16 of
// scale) and a genuinely turning one (~1e-1) far apart.
const relativeEpsilon = 1e-12

// orientation classifies the turn at b along a → b → c: 0 collinear, 1 one way,
// 2 the other.
//
// The tolerance is relative, and that is the whole point. The value is a cross
// product, so its unit is the coordinate unit *squared*: one edge of a building
// measures ~1e-4 in EPSG:4326 and ~10 in EPSG:25832, which puts the cross
// product of two of them at ~1e-8 against ~1e2. The fixed 1e-9 this used to
// compare against therefore answered "collinear" for nearly every pair of short
// edges in degrees and for nearly none of the same building's in metres — one
// footprint, two answers, decided by the CRS it happened to arrive in.
//
// A wrong "collinear" is not a near miss either, because it hands the decision
// to onSegment, which is a bounding-box overlap test and sound only when the
// triple really is collinear. So any two edges of a non-convex footprint whose
// boxes overlapped were reported as crossing. On a 3198-feature Berlin extract
// that was 219 of 631 buildings, every one of which is a simple polygon, and it
// is why an OSM import could not be saved at all.
func orientation(a, b, c point2) int {
	abx, aby := b.x-a.x, b.y-a.y
	bcx, bcy := c.x-b.x, c.y-b.y

	value := aby*bcx - abx*bcy
	tolerance := relativeEpsilon * (math.Abs(abx) + math.Abs(aby)) * (math.Abs(bcx) + math.Abs(bcy))

	switch {
	case math.Abs(value) <= tolerance:
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
	case SourceTypePoint:
		return isOneOf(geometryType, GeometryTypePoint, GeometryTypeMultiPoint)
	case SourceTypeLine:
		return isOneOf(geometryType, GeometryTypeLineString, GeometryTypeMultiLineString)
	case SourceTypeArea:
		return isOneOf(geometryType, GeometryTypePolygon, GeometryTypeMultiPolygon)
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
