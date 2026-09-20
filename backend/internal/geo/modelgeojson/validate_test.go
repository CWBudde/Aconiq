package modelgeojson

import (
	"math"
	"strings"
	"testing"
)

func TestNormalizeAndValidateValidModel(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [350000.0, 5800000.0]}
    },
    {
      "type": "Feature",
      "properties": {"id": "b-1", "kind": "building", "height_m": 12.0},
      "geometry": {"type": "Polygon", "coordinates": [[[0,0],[10,0],[10,10],[0,10],[0,0]]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "bar-1", "kind": "barrier", "height_m": 2.5},
      "geometry": {"type": "LineString", "coordinates": [[1,1],[5,1],[8,2]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4.0},
      "geometry": {"type": "Point", "coordinates": [20.0, 30.0]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if !report.Valid {
		t.Fatalf("expected valid model, got errors: %#v", report.Errors)
	}

	if report.ErrorCount() != 0 {
		t.Fatalf("expected 0 errors, got %d", report.ErrorCount())
	}

	if report.WarningCount() != 0 {
		t.Fatalf("expected 0 warnings, got %d", report.WarningCount())
	}

	if model.ToDump().FeatureCount != 4 {
		t.Fatalf("expected 4 features in dump")
	}
}

func TestValidateReceiverRequiresPositiveHeightAndPointGeometry(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 0},
      "geometry": {"type": "LineString", "coordinates": [[0,0],[1,1]]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if report.Valid {
		t.Fatal("expected invalid receiver feature")
	}

	foundHeight := false
	foundGeometry := false

	for _, issue := range report.Errors {
		if issue.Code == "receiver.height.invalid" {
			foundHeight = true
		}

		if issue.Code == "receiver.geometry.invalid" {
			foundGeometry = true
		}
	}

	if !foundHeight || !foundGeometry {
		t.Fatalf("expected receiver validation errors, got %#v", report.Errors)
	}
}

func TestValidateMissingBuildingHeight(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "b-1", "kind": "building"},
      "geometry": {"type": "Polygon", "coordinates": [[[0,0],[10,0],[10,10],[0,10],[0,0]]]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:4326", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if report.Valid {
		t.Fatal("expected validation error")
	}

	if report.ErrorCount() == 0 {
		t.Fatal("expected at least one error")
	}
}

func TestValidateSelfIntersectingPolygon(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "b-2", "kind": "building", "height_m": 10},
      "geometry": {"type": "Polygon", "coordinates": [[[0,0],[10,10],[10,0],[0,10],[0,0]]]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:4326", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if report.Valid {
		t.Fatal("expected invalid model")
	}

	found := false

	for _, issue := range report.Errors {
		if issue.Code == "geometry.polygon.self_intersection" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected self-intersection error, got %#v", report.Errors)
	}
}

func TestValidateProjectedCRSWithLonLatWarning(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-2", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [13.4, 52.5]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if report.ErrorCount() != 0 {
		t.Fatalf("expected no errors, got %#v", report.Errors)
	}

	if report.WarningCount() == 0 {
		t.Fatal("expected at least one warning")
	}
}

func TestNormalizePreservesCustomPropertiesRoundTrip(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "src-3",
        "kind": "source",
        "source_type": "line",
        "road_surface_type": "concrete",
        "traffic_day_light_vph": 123.5
      },
      "geometry": {"type": "LineString", "coordinates": [[0,0],[10,0]]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if got := model.Features[0].Properties["road_surface_type"]; got != "concrete" {
		t.Fatalf("expected preserved road_surface_type, got %#v", got)
	}

	collection := model.ToFeatureCollection()

	gotProps := collection.Features[0].Properties
	if got := gotProps["road_surface_type"]; got != "concrete" {
		t.Fatalf("expected round-tripped road_surface_type, got %#v", got)
	}

	if got := gotProps["traffic_day_light_vph"]; got != 123.5 {
		t.Fatalf("expected round-tripped traffic_day_light_vph, got %#v", got)
	}
}

// hasErrorCode reports whether the report carries an error with that code, and
// the feature ids it was raised against.
func hasErrorCode(report ValidationReport, code string) (string, bool) {
	for _, issue := range report.Errors {
		if issue.Code == code {
			return issue.FeatureID, true
		}
	}

	return "", false
}

func validateCalcAreaPayload(t *testing.T, payload string) ValidationReport {
	t.Helper()

	model, err := Normalize([]byte(payload), "EPSG:25832", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	return Validate(model)
}

func TestValidateCalcAreaPolygonWithoutHeightIsValid(t *testing.T) {
	t.Parallel()

	report := validateCalcAreaPayload(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "calc-1", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[350000,5800000],[350100,5800000],[350100,5800100],[350000,5800100],[350000,5800000]]]}
    }
  ]
}`)

	if !report.Valid {
		t.Fatalf("expected a calc-area polygon without height_m to validate, got %#v", report.Errors)
	}
}

func TestValidateCalcAreaRejectsNonPolygonGeometry(t *testing.T) {
	t.Parallel()

	geometries := map[string]string{
		"LineString":   `{"type": "LineString", "coordinates": [[350000,5800000],[350100,5800100]]}`,
		"Point":        `{"type": "Point", "coordinates": [350000,5800000]}`,
		"MultiPolygon": `{"type": "MultiPolygon", "coordinates": [[[[350000,5800000],[350100,5800000],[350100,5800100],[350000,5800100],[350000,5800000]]]]}`,
	}

	for name, geometry := range geometries {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := validateCalcAreaPayload(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "calc-1", "kind": "calc-area"},
      "geometry": `+geometry+`
    }
  ]
}`)

			featureID, found := hasErrorCode(report, "calcarea.geometry.invalid")
			if !found {
				t.Fatalf("expected calcarea.geometry.invalid for %s, got %#v", name, report.Errors)
			}

			if featureID != "calc-1" {
				t.Fatalf("expected the error to name calc-1, got %q", featureID)
			}
		})
	}
}

func TestValidateRejectsASecondCalcArea(t *testing.T) {
	t.Parallel()

	report := validateCalcAreaPayload(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "calc-1", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[350000,5800000],[350100,5800000],[350100,5800100],[350000,5800100],[350000,5800000]]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "calc-2", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[360000,5800000],[360100,5800000],[360100,5800100],[360000,5800100],[360000,5800000]]]}
    }
  ]
}`)

	featureID, found := hasErrorCode(report, "model.calc_area.duplicate")
	if !found {
		t.Fatalf("expected model.calc_area.duplicate, got %#v", report.Errors)
	}

	// The second one is the one that can be removed, so it is the one named.
	if featureID != "calc-2" {
		t.Fatalf("expected the duplicate error to name calc-2, got %q", featureID)
	}
}

func TestValidateUnknownKindMessageListsEveryKind(t *testing.T) {
	t.Parallel()

	report := validateCalcAreaPayload(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "x-1", "kind": "calculation-area"},
      "geometry": {"type": "Point", "coordinates": [350000,5800000]}
    }
  ]
}`)

	var message string

	for _, issue := range report.Errors {
		if issue.Code == "feature.kind.invalid" {
			message = issue.Message
		}
	}

	if message == "" {
		t.Fatalf("expected feature.kind.invalid, got %#v", report.Errors)
	}

	for _, kind := range FeatureKinds {
		if !strings.Contains(message, kind) {
			t.Fatalf("kind rejection message %q does not mention %q", message, kind)
		}
	}
}

// A road drawn as one way that touches itself is a roundabout, not a defect,
// and nothing in the model reads a line's winding — so it is reported and the
// model still saves. 45 of 2397 ways in a Berlin OSM extract are like this.
func TestValidateSelfIntersectingLineIsAWarning(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "s-1", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0,0],[10,10],[0,10],[10,0]]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:4326", "input.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	report := Validate(model)
	if !report.Valid {
		t.Fatalf("a self-intersecting line must not refuse the model, got %#v", report.Errors)
	}

	if !hasCode(report.Warnings, "geometry.linestring.self_intersection") {
		t.Fatalf("expected a self-intersection warning, got %#v", report.Warnings)
	}
}

// The same footprint must get the same answer whichever CRS it arrives in.
//
// It did not. orientation compared a cross product — an area, so the coordinate
// unit squared — against a fixed 1e-9. That is large next to two short edges in
// degrees (~1e-10) and negligible next to the same two in metres (~1e0), so the
// predicate answered "collinear" in one CRS and "turning" in the other. A false
// "collinear" then falls through to onSegment, which is a bounding-box test and
// sound only when the triple really is collinear, and any two edges of a
// non-convex footprint whose boxes overlap read as crossing.
//
// The shape below is a rotated L: small, because a synthetic reproducer needs
// short edges to get the cross product down there. Real OSM footprints reach
// the same place at ordinary sizes by having many more vertices — it flagged
// 219 of 631 buildings in a Berlin extract, every one of them a simple polygon,
// and that is why an OSM import could not be saved at all.
func TestValidateSelfIntersectionIsScaleInvariant(t *testing.T) {
	t.Parallel()

	shape := [][2]float64{{0, 0}, {3, 0}, {3, 1}, {1, 1}, {1, 3}, {0, 3}, {0, 0}}

	const rotation = 31 * math.Pi / 180

	ring := func(unit, originX, originY float64) []point2 {
		out := make([]point2, 0, len(shape))
		for _, p := range shape {
			x := p[0]*math.Cos(rotation) - p[1]*math.Sin(rotation)
			y := p[0]*math.Sin(rotation) + p[1]*math.Cos(rotation)
			out = append(out, point2{x: originX + x*unit, y: originY + y*unit})
		}

		return out
	}

	const degreeUnit = 1e-5

	for _, tc := range []struct {
		name string
		ring []point2
	}{
		// EPSG:4326 near Berlin, and the same footprint in EPSG:25832 metres.
		{name: "degrees", ring: ring(degreeUnit, 13.38, 52.51)},
		{name: "metres", ring: ring(degreeUnit*111320, 390000, 5819000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if hasSelfIntersection(tc.ring, true) {
				t.Errorf("a simple L-shaped footprint was reported as self-intersecting in %s", tc.name)
			}
		})
	}
}

// A genuine bow-tie must still be caught at building scale, where the whole
// ring spans a ten-thousandth of a degree — the relative tolerance has to be
// tight enough to see a crossing that small.
func TestValidateSelfIntersectionStillCaughtAtBuildingScale(t *testing.T) {
	t.Parallel()

	const s = 1e-4

	ring := []point2{
		{x: 13.38, y: 52.51},
		{x: 13.38 + s, y: 52.51 + s},
		{x: 13.38 + s, y: 52.51},
		{x: 13.38, y: 52.51 + s},
		{x: 13.38, y: 52.51},
	}

	if !hasSelfIntersection(ring, true) {
		t.Error("a bow-tie footprint the size of a real building was not detected")
	}
}

func hasCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}

	return false
}
