package osmimport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/buildinfo"
	overpass "github.com/cwbudde/go-overpass"
)

// mockHTTP implements overpass.HTTPClient for testing without a real network.
//
// It records the last request so a test can assert on the headers Fetch sends,
// and answers StatusOK unless status says otherwise.
type mockHTTP struct {
	body   string
	status int
	last   *http.Request
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	m.last = req

	status := m.status
	if status == 0 {
		status = http.StatusOK
	}

	return &http.Response{
		StatusCode: status,
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

// heightSourceJSON carries one way per height_source case: a building whose
// height is tagged, one whose height is derived from its storey count, one
// with neither, and a barrier with and without a height tag.
const heightSourceJSON = `{
  "version": 0.6,
  "elements": [
    {
      "type": "way",
      "id": 401,
      "geometry": [
        {"lat": 51.510, "lon": 8.210},
        {"lat": 51.511, "lon": 8.211},
        {"lat": 51.512, "lon": 8.212},
        {"lat": 51.510, "lon": 8.210}
      ],
      "tags": {"building": "yes", "height": "12 m", "building:levels": "5"}
    },
    {
      "type": "way",
      "id": 402,
      "geometry": [
        {"lat": 51.520, "lon": 8.220},
        {"lat": 51.521, "lon": 8.221},
        {"lat": 51.522, "lon": 8.222},
        {"lat": 51.520, "lon": 8.220}
      ],
      "tags": {"building": "yes", "building:levels": "4"}
    },
    {
      "type": "way",
      "id": 403,
      "geometry": [
        {"lat": 51.530, "lon": 8.230},
        {"lat": 51.531, "lon": 8.231},
        {"lat": 51.532, "lon": 8.232},
        {"lat": 51.530, "lon": 8.230}
      ],
      "tags": {"building": "yes"}
    },
    {
      "type": "way",
      "id": 404,
      "geometry": [{"lat": 51.540, "lon": 8.240}, {"lat": 51.541, "lon": 8.241}],
      "tags": {"barrier": "wall", "height": "2.5"}
    },
    {
      "type": "way",
      "id": 405,
      "geometry": [{"lat": 51.550, "lon": 8.250}, {"lat": 51.551, "lon": 8.251}],
      "tags": {"barrier": "fence"}
    }
  ]
}`

// TestFetch_HeightSource holds the provenance each case leaves on the feature.
//
// The levels case is the one that matters downstream: a facade receiver's
// slab height is only as good as the building height it divides, and a reader
// has to tell a tagged height from one estimated off the storey count. While
// both left height_source absent it could not.
func TestFetch_HeightSource(t *testing.T) {
	t.Parallel()

	fc, err := Fetch(context.Background(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.6, East: 8.3},
		OverpassEndpoint: "http://mock",
		HTTPClient:       &mockHTTP{body: heightSourceJSON},
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	byID := make(map[string]map[string]any, len(fc.Features))
	for _, feat := range fc.Features {
		id, _ := feat.ID.(string)
		byID[id] = feat.Properties
	}

	tests := []struct {
		id         string
		wantHeight float64
		wantSource string // "" means the property must be absent
	}{
		{id: "osm-way-401", wantHeight: 12.0},
		{id: "osm-way-402", wantHeight: 12.0, wantSource: heightSourceLevels},
		{id: "osm-way-403", wantHeight: defaultBuildingHeightM, wantSource: heightSourceAssumed},
		{id: "osm-way-404", wantHeight: 2.5},
		{id: "osm-way-405", wantHeight: defaultBarrierHeightM, wantSource: heightSourceAssumed},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()

			props, ok := byID[tc.id]
			if !ok {
				t.Fatalf("feature %s missing from the import", tc.id)
			}

			if got := props["height_m"]; got != tc.wantHeight {
				t.Errorf("height_m = %v, want %g", got, tc.wantHeight)
			}

			got, present := props[propertyHeightSource]

			switch {
			case tc.wantSource == "" && present:
				t.Errorf("height_source = %v, want it absent", got)
			case tc.wantSource != "" && got != tc.wantSource:
				t.Errorf("height_source = %v, want %q", got, tc.wantSource)
			}
		})
	}
}

func TestBuildingHeight(t *testing.T) {
	t.Parallel()

	t.Run("from height tag", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"height": "12m"})
		if h.meters != 12.0 || h.source != "" {
			t.Errorf("expected 12.0 read from the tag, got %+v", h)
		}
	})

	t.Run("from building:levels tag", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"building:levels": "4"})
		if h.meters != 12.0 {
			t.Errorf("expected 12.0 (4x3m), got %+v", h)
		}

		// Derived, not read: the storey count is a tag, the 3 m a storey is
		// not, so the feature has to say which of the two it got.
		if h.source != heightSourceLevels {
			t.Errorf("expected source %q, got %+v", heightSourceLevels, h)
		}
	})

	t.Run("assumed when neither tag is present", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"building": "yes"})
		if h.meters != defaultBuildingHeightM {
			t.Errorf("expected the default %g, got %+v", defaultBuildingHeightM, h)
		}

		// The number alone is not the point: an untagged building used to be
		// dropped, which made the whole import unsaveable. It is now assumed,
		// and the feature has to say so.
		if h.source != heightSourceAssumed {
			t.Error("the default height must be reported as assumed")
		}
	})

	t.Run("height tag preferred over levels", func(t *testing.T) {
		t.Parallel()

		h := buildingHeight(map[string]string{"height": "7.5 m", "building:levels": "3"})
		if h.meters != 7.5 || h.source != "" {
			t.Errorf("expected 7.5 from the height tag, got %+v", h)
		}
	})
}

// A tag that parses but is not a positive, finite number says nothing about
// the building. Taking it gave a 0 m (or negative) building that model
// validation refuses, which made the whole import unsaveable. It falls
// through to the next rule instead, as the browser import does
// (osmBuildingHeight in frontend/src/api/browser-backend.ts).
func TestBuildingHeightIgnoresANonPositiveTag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		tags   map[string]string
		meters float64
		source string
	}{
		{"zero height falls through to levels", map[string]string{"height": "0", "building:levels": "4"}, 12, heightSourceLevels},
		{"zero metre height falls through to levels", map[string]string{"height": "0 m", "building:levels": "2"}, 6, heightSourceLevels},
		{"negative height falls through to the default", map[string]string{"height": "-3"}, defaultBuildingHeightM, heightSourceAssumed},
		{"infinite height falls through to the default", map[string]string{"height": "Inf"}, defaultBuildingHeightM, heightSourceAssumed},
		{"zero levels fall through to the default", map[string]string{"building:levels": "0"}, defaultBuildingHeightM, heightSourceAssumed},
		{"negative levels fall through to the default", map[string]string{"building:levels": "-1"}, defaultBuildingHeightM, heightSourceAssumed},
		{"NaN levels fall through to the default", map[string]string{"building:levels": "NaN"}, defaultBuildingHeightM, heightSourceAssumed},
		{"zero height and zero levels give the default", map[string]string{"height": "0", "building:levels": "0"}, defaultBuildingHeightM, heightSourceAssumed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := buildingHeight(tc.tags)
			if h.meters != tc.meters || h.source != tc.source {
				t.Errorf("buildingHeight(%v) = %+v, want %g m with source %q", tc.tags, h, tc.meters, tc.source)
			}
		})
	}
}

func TestBarrierHeight(t *testing.T) {
	t.Parallel()

	t.Run("explicit height from tag", func(t *testing.T) {
		t.Parallel()

		h := barrierHeight(map[string]string{"height": "3.5"})
		if h.meters != 3.5 || h.source != "" {
			t.Errorf("expected 3.5 read from the tag, got %+v", h)
		}
	})

	t.Run("assumed 2m when no tag", func(t *testing.T) {
		t.Parallel()

		h := barrierHeight(map[string]string{"barrier": "wall"})
		if h.meters != defaultBarrierHeightM || h.source != heightSourceAssumed {
			t.Errorf("expected an assumed %g, got %+v", defaultBarrierHeightM, h)
		}
	})

	// A barrier needs height_m > 0 as a building does, so a zero tag is
	// no height at all.
	t.Run("assumed 2m when the tag is zero", func(t *testing.T) {
		t.Parallel()

		h := barrierHeight(map[string]string{"barrier": "wall", "height": "0"})
		if h.meters != defaultBarrierHeightM || h.source != heightSourceAssumed {
			t.Errorf("expected an assumed %g, got %+v", defaultBarrierHeightM, h)
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

func TestFetch_SendsAUserAgent(t *testing.T) {
	t.Parallel()

	// overpass-api.de answers 406 to Go's default `Go-http-client/1.1`, and
	// go-overpass sets no User-Agent of its own, so this header is the whole
	// reason an import works from some networks at all.
	mock := &mockHTTP{body: highwayJSON}

	if _, err := Fetch(t.Context(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.51, East: 8.21},
		OverpassEndpoint: "http://mock",
		HTTPClient:       mock,
	}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if mock.last == nil {
		t.Fatal("no request reached the client")
	}

	got := mock.last.Header.Get("User-Agent")
	if got == "" {
		t.Fatal("no User-Agent was sent")
	}

	if !strings.HasPrefix(got, buildinfo.Name+"/") {
		t.Errorf("User-Agent %q does not name the product", got)
	}

	// The Overpass usage policy asks for a way to make contact, so the URL is
	// as load-bearing as the name.
	if !strings.Contains(got, contactURL) {
		t.Errorf("User-Agent %q carries no contact URL", got)
	}
}

func TestFetch_UpstreamStatusSurvives(t *testing.T) {
	t.Parallel()

	// The handler turns this status into an error envelope a reader can act on,
	// which it can only do if the status reaches it. 406 is deliberate: it is
	// what overpass-api.de answers an unnamed agent, and go-overpass does not
	// retry it, so the test neither sleeps through a backoff nor hides a retry.
	mock := &mockHTTP{body: "not acceptable", status: http.StatusNotAcceptable}

	_, err := Fetch(t.Context(), Config{
		BBox:             BBox{South: 51.5, West: 8.2, North: 51.51, East: 8.21},
		OverpassEndpoint: "http://mock",
		HTTPClient:       mock,
	})
	if err == nil {
		t.Fatal("expected an error for a 406 response")
	}

	var serverErr *overpass.ServerError
	if !errors.As(err, &serverErr) {
		t.Fatalf("the upstream status did not survive: %v", err)
	}

	if serverErr.StatusCode != http.StatusNotAcceptable {
		t.Errorf("expected status 406, got %d", serverErr.StatusCode)
	}
}
