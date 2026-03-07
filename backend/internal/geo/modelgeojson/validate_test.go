package modelgeojson

import "testing"

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

	if model.ToDump().FeatureCount != 3 {
		t.Fatalf("expected 3 features in dump")
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
