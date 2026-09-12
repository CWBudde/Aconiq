package modelgeojson

import (
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
