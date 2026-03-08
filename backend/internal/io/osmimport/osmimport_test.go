package osmimport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTP implements overpass.HTTPClient for testing without a real network.
type mockHTTP struct{ body string }

func (m *mockHTTP) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

const highwayJSON = `{
  "version": 0.6,
  "elements": [
    {
      "type": "way",
      "id": 100,
      "geometry": [{"lat": 51.500, "lon": 8.200}, {"lat": 51.501, "lon": 8.201}],
      "tags": {"highway": "primary", "maxspeed": "50", "lanes": "2", "surface": "asphalt"}
    }
  ]
}`

const buildingJSON = `{
  "version": 0.6,
  "elements": [
    {
      "type": "way",
      "id": 200,
      "geometry": [
        {"lat": 51.510, "lon": 8.210},
        {"lat": 51.511, "lon": 8.211},
        {"lat": 51.512, "lon": 8.212},
        {"lat": 51.510, "lon": 8.210}
      ],
      "tags": {"building": "yes", "building:levels": "3"}
    }
  ]
}`

const barrierJSON = `{
  "version": 0.6,
  "elements": [
    {
      "type": "way",
      "id": 300,
      "geometry": [{"lat": 51.520, "lon": 8.220}, {"lat": 51.521, "lon": 8.221}],
      "tags": {"barrier": "wall", "height": "2.5"}
    }
  ]
}`

func TestFetch_Highway(t *testing.T) {
	t.Parallel()

	fc, err := Fetch(context.Background(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.6, East: 8.3},
		OverpassEndpoint: "http://mock",
		HTTPClient:       &mockHTTP{body: highwayJSON},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	feat := fc.Features[0]

	if feat.Properties["kind"] != "source" {
		t.Errorf("expected kind=source, got %v", feat.Properties["kind"])
	}

	if feat.Properties["source_type"] != "line" {
		t.Errorf("expected source_type=line, got %v", feat.Properties["source_type"])
	}

	if feat.Properties["highway"] != "primary" {
		t.Errorf("expected highway=primary, got %v", feat.Properties["highway"])
	}

	if feat.Properties["maxspeed_kmh"] != 50.0 {
		t.Errorf("expected maxspeed_kmh=50.0, got %v", feat.Properties["maxspeed_kmh"])
	}

	if feat.Geometry.Type != "LineString" {
		t.Errorf("expected LineString geometry, got %s", feat.Geometry.Type)
	}

	if feat.ID != "osm-way-100" {
		t.Errorf("expected ID osm-way-100, got %v", feat.ID)
	}
}

func TestFetch_Building(t *testing.T) {
	t.Parallel()

	fc, err := Fetch(context.Background(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.6, East: 8.3},
		OverpassEndpoint: "http://mock",
		HTTPClient:       &mockHTTP{body: buildingJSON},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	feat := fc.Features[0]

	if feat.Properties["kind"] != "building" {
		t.Errorf("expected kind=building, got %v", feat.Properties["kind"])
	}

	if feat.Geometry.Type != "Polygon" {
		t.Errorf("expected Polygon geometry, got %s", feat.Geometry.Type)
	}

	heightM, ok := feat.Properties["height_m"].(float64)
	if !ok {
		t.Fatalf("expected height_m as float64, got %T", feat.Properties["height_m"])
	}

	const wantHeight = 9.0 // 3 levels × 3 m
	if heightM != wantHeight {
		t.Errorf("expected height_m=%g, got %g", wantHeight, heightM)
	}

	if feat.ID != "osm-way-200" {
		t.Errorf("expected ID osm-way-200, got %v", feat.ID)
	}
}

func TestFetch_Barrier(t *testing.T) {
	t.Parallel()

	fc, err := Fetch(context.Background(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.6, East: 8.3},
		OverpassEndpoint: "http://mock",
		HTTPClient:       &mockHTTP{body: barrierJSON},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	feat := fc.Features[0]

	if feat.Properties["kind"] != "barrier" {
		t.Errorf("expected kind=barrier, got %v", feat.Properties["kind"])
	}

	heightM, ok := feat.Properties["height_m"].(float64)
	if !ok {
		t.Fatalf("expected height_m as float64, got %T", feat.Properties["height_m"])
	}

	const wantHeight = 2.5
	if heightM != wantHeight {
		t.Errorf("expected height_m=%g (from tag), got %g", wantHeight, heightM)
	}

	if feat.Geometry.Type != "LineString" {
		t.Errorf("expected LineString geometry, got %s", feat.Geometry.Type)
	}

	if feat.ID != "osm-way-300" {
		t.Errorf("expected ID osm-way-300, got %v", feat.ID)
	}
}

func TestBuildingHeight(t *testing.T) {
	t.Parallel()

	t.Run("from height tag", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"height": "12m"})
		if h == nil || *h != 12.0 {
			t.Errorf("expected 12.0, got %v", h)
		}
	})

	t.Run("from building:levels tag", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"building:levels": "4"})
		if h == nil || *h != 12.0 {
			t.Errorf("expected 12.0 (4×3m), got %v", h)
		}
	})

	t.Run("nil when unknown", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"building": "yes"})
		if h != nil {
			t.Errorf("expected nil, got %v", *h)
		}
	})

	t.Run("height tag preferred over levels", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"height": "7.5 m", "building:levels": "3"})
		if h == nil || *h != 7.5 {
			t.Errorf("expected 7.5 from height tag, got %v", h)
		}
	})
}

func TestBarrierHeight(t *testing.T) {
	t.Parallel()

	t.Run("explicit height from tag", func(t *testing.T) {
		t.Parallel()

		h := barrierHeight(map[string]string{"height": "3.5"})
		if h == nil || *h != 3.5 {
			t.Errorf("expected 3.5, got %v", h)
		}
	})

	t.Run("default 2m when no tag", func(t *testing.T) {
		t.Parallel()

		h := barrierHeight(map[string]string{"barrier": "wall"})
		if h == nil || *h != defaultBarrierHeightM {
			t.Errorf("expected default %g, got %v", defaultBarrierHeightM, h)
		}
	})
}

func TestPolygonCoords_Closed(t *testing.T) {
	t.Parallel()

	// Verify ring closure via Fetch using a mock with an open-ring building
	// (4 distinct points, first != last → polygon ring must be auto-closed to 5 points).
	openRingJSON := `{
  "version": 0.6,
  "elements": [
    {
      "type": "way",
      "id": 400,
      "geometry": [
        {"lat": 51.0, "lon": 8.0},
        {"lat": 51.1, "lon": 8.0},
        {"lat": 51.1, "lon": 8.1},
        {"lat": 51.0, "lon": 8.1}
      ],
      "tags": {"building": "yes"}
    }
  ]
}`

	fc, err := Fetch(context.Background(), Config{
		BBox:             BBox{South: 51.0, West: 8.0, North: 51.2, East: 8.2},
		OverpassEndpoint: "http://mock",
		HTTPClient:       &mockHTTP{body: openRingJSON},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature for open-ring building, got %d", len(fc.Features))
	}

	feat := fc.Features[0]
	if feat.Geometry.Type != "Polygon" {
		t.Fatalf("expected Polygon, got %s", feat.Geometry.Type)
	}

	ring := feat.Geometry.Coordinates.([]any)[0].([]any)
	// 4 input points → ring should be closed → 5 points
	if len(ring) != 5 {
		t.Errorf("expected 5 ring points (4 input + closure), got %d", len(ring))
	}

	first := ring[0].([]any)
	last := ring[len(ring)-1].([]any)

	if first[0] != last[0] || first[1] != last[1] {
		t.Errorf("ring not closed: first=%v, last=%v", first, last)
	}
}

func TestBuildQuery(t *testing.T) {
	t.Parallel()

	bbox := BBox{South: 51.5, West: 8.2, North: 51.6, East: 8.3}
	q := buildQuery(bbox, 30)

	for _, keyword := range []string{"highway", "railway", "building", "barrier", "out geom"} {
		if !strings.Contains(q, keyword) {
			t.Errorf("expected query to contain %q, but it does not\nQuery:\n%s", keyword, q)
		}
	}

	if !strings.Contains(q, "[timeout:30]") {
		t.Errorf("expected [timeout:30] in query, but got:\n%s", q)
	}
}
