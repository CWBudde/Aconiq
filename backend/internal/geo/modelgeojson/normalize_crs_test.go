package modelgeojson

import (
	"math"
	"testing"
)

func TestNormalizeWithCRS_TransformApplied(t *testing.T) {
	t.Parallel()

	// A point source in WGS84 (lon=9.732, lat=52.376) near Hannover.
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [9.732, 52.376]}
    }
  ]
}`)

	model, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:4326", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if !model.TransformApplied {
		t.Fatal("expected TransformApplied=true")
	}

	if model.ImportCRS != "EPSG:4326" {
		t.Fatalf("expected ImportCRS=EPSG:4326, got %q", model.ImportCRS)
	}

	if model.ProjectCRS != "EPSG:25832" {
		t.Fatalf("expected ProjectCRS=EPSG:25832, got %q", model.ProjectCRS)
	}

	// Check that coordinates were transformed from lon/lat to UTM32 easting/northing.
	coords, ok := model.Features[0].Coordinates.([]any)
	if !ok || len(coords) < 2 {
		t.Fatalf("unexpected coordinate structure: %T", model.Features[0].Coordinates)
	}

	x, xOK := coords[0].(float64)
	y, yOK := coords[1].(float64)

	if !xOK || !yOK {
		t.Fatalf("coordinates are not float64: %T, %T", coords[0], coords[1])
	}

	// UTM32 easting should be ~549xxx, northing ~5803xxx.
	if x < 540000 || x > 560000 {
		t.Fatalf("transformed easting %.2f not in expected UTM32 range", x)
	}

	if y < 5790000 || y > 5810000 {
		t.Fatalf("transformed northing %.2f not in expected UTM32 range", y)
	}
}

func TestNormalizeWithCRS_NoTransformWhenSameCRS(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [500000.0, 5800000.0]}
    }
  ]
}`)

	model, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:25832", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if model.TransformApplied {
		t.Fatal("expected TransformApplied=false when CRS match")
	}

	// Coordinates should be unchanged.
	coords, ok := model.Features[0].Coordinates.([]any)
	if !ok {
		t.Fatal("unexpected coordinate type")
	}

	x, _ := coords[0].(float64)
	y, _ := coords[1].(float64)

	if x != 500000.0 || y != 5800000.0 {
		t.Fatalf("coordinates should be unchanged: got (%.2f, %.2f)", x, y)
	}
}

func TestNormalizeWithCRS_EmptyImportCRS(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [9.0, 52.0]}
    }
  ]
}`)

	// Empty importCRS should not attempt transform.
	model, err := NormalizeWithCRS(payload, "EPSG:4326", "", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if model.TransformApplied {
		t.Fatal("expected no transform when importCRS is empty")
	}
}

func TestNormalizeWithCRS_LineStringTransform(t *testing.T) {
	t.Parallel()

	// Two-point line in WGS84.
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "road-1", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[9.73, 52.37], [9.74, 52.38]]}
    }
  ]
}`)

	model, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:4326", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if !model.TransformApplied {
		t.Fatal("expected transform applied")
	}

	coords, ok := model.Features[0].Coordinates.([]any)
	if !ok || len(coords) != 2 {
		t.Fatalf("expected 2 coordinate pairs, got %v", model.Features[0].Coordinates)
	}

	// Both points should be in UTM32 range.
	for i, c := range coords {
		pt, ptOK := c.([]any)
		if !ptOK || len(pt) < 2 {
			t.Fatalf("point[%d]: unexpected structure", i)
		}

		x, _ := pt[0].(float64)
		y, _ := pt[1].(float64)

		if x < 540000 || x > 560000 || y < 5790000 || y > 5810000 {
			t.Fatalf("point[%d]: transformed coords (%.2f, %.2f) out of UTM32 range", i, x, y)
		}
	}
}

func TestNormalizeWithCRS_PolygonTransform(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "bld-1", "kind": "building", "height_m": 10},
      "geometry": {"type": "Polygon", "coordinates": [[[9.73, 52.37], [9.74, 52.37], [9.74, 52.38], [9.73, 52.38], [9.73, 52.37]]]}
    }
  ]
}`)

	model, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:4326", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if !model.TransformApplied {
		t.Fatal("expected transform applied")
	}

	// Polygon coordinates are [rings][points][x,y].
	rings, ok := model.Features[0].Coordinates.([]any)
	if !ok || len(rings) != 1 {
		t.Fatalf("expected 1 ring, got %v", model.Features[0].Coordinates)
	}

	ring, ringOK := rings[0].([]any)
	if !ringOK || len(ring) != 5 {
		t.Fatalf("expected 5 points in ring, got %d", len(ring))
	}

	// Check first point is in UTM32 range.
	pt, ptOK := ring[0].([]any)
	if !ptOK || len(pt) < 2 {
		t.Fatal("unexpected point structure")
	}

	x, _ := pt[0].(float64)
	y, _ := pt[1].(float64)

	if x < 540000 || x > 560000 || y < 5790000 || y > 5810000 {
		t.Fatalf("transformed coords (%.2f, %.2f) out of UTM32 range", x, y)
	}
}

func TestNormalizeWithCRS_PreservesZ(t *testing.T) {
	t.Parallel()

	// Point with Z coordinate.
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [9.732, 52.376, 55.5]}
    }
  ]
}`)

	model, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:4326", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	coords, ok := model.Features[0].Coordinates.([]any)
	if !ok || len(coords) != 3 {
		t.Fatalf("expected 3 coordinates (x,y,z), got %v", model.Features[0].Coordinates)
	}

	z, zOK := coords[2].(float64)
	if !zOK || math.Abs(z-55.5) > 0.001 {
		t.Fatalf("Z coordinate should be preserved: got %v", coords[2])
	}
}

func TestNormalizeWithCRS_UnsupportedCRS(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source"},
      "geometry": {"type": "Point", "coordinates": [0, 0]}
    }
  ]
}`)

	_, err := NormalizeWithCRS(payload, "EPSG:25832", "EPSG:99999", "test.geojson")
	if err == nil {
		t.Fatal("expected error for unsupported CRS")
	}
}

func TestNormalizeWithCRS_BackwardCompatible(t *testing.T) {
	t.Parallel()

	// Original Normalize() signature should still work without CRS transform.
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [9.0, 52.0]}
    }
  ]
}`)

	model, err := Normalize(payload, "EPSG:4326", "test.geojson")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if model.TransformApplied {
		t.Fatal("legacy Normalize should not apply transform")
	}

	if model.ImportCRS != "" {
		t.Fatalf("legacy Normalize should have empty ImportCRS, got %q", model.ImportCRS)
	}
}
