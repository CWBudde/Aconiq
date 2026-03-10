package httpv1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	fixedTime := time.Date(2026, 3, 6, 9, 0, 0, 0, time.UTC)
	handler := NewHandler(store, func() time.Time { return fixedTime })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response healthResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Status != "ok" {
		t.Fatalf("unexpected status: %q", response.Status)
	}

	if response.Version != apiVersion {
		t.Fatalf("unexpected version: %q", response.Version)
	}

	if !response.Time.Equal(fixedTime) {
		t.Fatalf("unexpected time: %s", response.Time)
	}
}

func TestProjectStatusEndpoint(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Phase23 API", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response projectStatusResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.ProjectID == "" {
		t.Fatal("project_id must be set")
	}

	if response.Name != "Phase23 API" {
		t.Fatalf("unexpected name: %q", response.Name)
	}

	if response.ProjectPath != projectDir {
		t.Fatalf("unexpected project path: %q", response.ProjectPath)
	}
}

func TestProjectStatusReturnsNotFoundWhenProjectNotInitialized(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestMethodNotAllowedReturnsStandardizedError(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "method_not_allowed" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestUnknownRouteReturnsStandardizedNotFound(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestOpenAPIEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	decodeResponse(t, rec.Body.Bytes(), &payload)

	if payload["openapi"] != OpenAPIVersion {
		t.Fatalf("unexpected openapi version: %#v", payload["openapi"])
	}

	paths, ok := payload["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi paths object")
	}

	if _, exists := paths["/api/v1/events"]; !exists {
		t.Fatalf("expected /api/v1/events path in openapi document")
	}
}

func TestRunsListEndpointReturnsEmptyListWhenNoRuns(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Runs Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []runSummaryResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if len(response) != 0 {
		t.Fatalf("expected empty list, got %d runs", len(response))
	}
}

func TestCreateRunEndpointCreatesRunSummary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Runs Create Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		runExecutor: func(ctx context.Context, req createRunRequest) error {
			_, _, err := store.CreateRun(projectfs.CreateRunSpec{
				ScenarioID:    "default",
				ReceiverMode:  req.ReceiverMode,
				ReceiverSetID: "explicit-manual",
				Standard: project.StandardRef{
					ID:      "rls19-road",
					Version: "2019",
					Profile: "default",
				},
				Status: project.RunStatusCompleted,
			})

			return err
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{
		"standard_id": "rls19-road",
		"receiver_mode": "custom"
	}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var response runSummaryResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.StandardID != "rls19-road" {
		t.Fatalf("unexpected standard id: %q", response.StandardID)
	}

	if response.ReceiverMode != "custom" {
		t.Fatalf("unexpected receiver mode: %q", response.ReceiverMode)
	}
}

func TestRunLogEndpointReturnsNotFoundForUnknownRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Runs Log Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/run-nope/log", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestStandardsEndpointReturnsRegisteredStandards(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	handler := NewHandlerWithRegistry(store, nil, registry)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/standards", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []standardResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if len(response) == 0 {
		t.Fatal("expected at least one standard")
	}

	found := false
	foundSchall03 := false
	foundISO9613 := false

	for _, s := range response {
		if s.ID == "rls19-road" {
			found = true

			if len(s.Versions) == 0 {
				t.Fatal("rls19-road: expected at least one version")
			}

			if len(s.Versions[0].Profiles) == 0 {
				t.Fatal("rls19-road: expected at least one profile")
			}

			if len(s.Versions[0].Profiles[0].Parameters) == 0 {
				t.Fatal("rls19-road: expected parameters")
			}
		}

		if s.ID == "schall03" {
			foundSchall03 = true

			if len(s.Versions) == 0 {
				t.Fatal("schall03: expected at least one version")
			}

			if s.Context != "planning" {
				t.Fatalf("schall03: unexpected context %q", s.Context)
			}
		}

		if s.ID == "iso9613" {
			foundISO9613 = true

			if len(s.Versions) == 0 {
				t.Fatal("iso9613: expected at least one version")
			}

			if s.Context != "planning" {
				t.Fatalf("iso9613: unexpected context %q", s.Context)
			}
		}
	}

	if !found {
		t.Fatal("expected rls19-road in standards list")
	}

	if !foundSchall03 {
		t.Fatal("expected schall03 in standards list")
	}

	if !foundISO9613 {
		t.Fatal("expected iso9613 in standards list")
	}
}

func TestStandardsEndpointReturnsUnavailableWithoutRegistry(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/standards", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestEventsEndpointStreamsProjectStatusAndHeartbeat(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Phase23 Stream", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{
		clock:       time.Now,
		sseInterval: 10 * time.Millisecond,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/api/v1/events")
	if err != nil {
		t.Fatalf("request events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("unexpected content-type: %q", got)
	}

	eventData, err := waitForSSEEventData(resp.Body, 1500*time.Millisecond, func(seen map[string]string) bool {
		return seen["project_status"] != "" && seen["heartbeat"] != ""
	})
	if err != nil {
		t.Fatalf("read stream events: %v", err)
	}

	var statusPayload map[string]any

	err = json.Unmarshal([]byte(eventData["project_status"]), &statusPayload)
	if err != nil {
		t.Fatalf("decode project_status payload: %v", err)
	}

	projectAvailable, ok := statusPayload["project_available"].(bool)
	if !ok || !projectAvailable {
		t.Fatalf("expected project_available=true, got %#v", statusPayload["project_available"])
	}
}

func TestEventsEndpointReportsMissingProject(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{
		clock:       time.Now,
		sseInterval: 10 * time.Millisecond,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/api/v1/events")
	if err != nil {
		t.Fatalf("request events: %v", err)
	}
	defer resp.Body.Close()

	eventData, err := waitForSSEEventData(resp.Body, 1500*time.Millisecond, func(seen map[string]string) bool {
		return seen["project_status"] != ""
	})
	if err != nil {
		t.Fatalf("read stream events: %v", err)
	}

	var statusPayload map[string]any

	err = json.Unmarshal([]byte(eventData["project_status"]), &statusPayload)
	if err != nil {
		t.Fatalf("decode project_status payload: %v", err)
	}

	projectAvailable, ok := statusPayload["project_available"].(bool)
	if !ok || projectAvailable {
		t.Fatalf("expected project_available=false, got %#v", statusPayload["project_available"])
	}

	errorPayload, ok := statusPayload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error payload in stream event, got %#v", statusPayload["error"])
	}

	if errorPayload["code"] != "not_found" {
		t.Fatalf("expected stream error code not_found, got %#v", errorPayload["code"])
	}
}

func TestImportOSMEndpointRejectsBadMethod(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/import/osm", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "method_not_allowed" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsBadBBoxSouthGeNorth(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":52.5,"west":13.3,"north":52.0,"east":13.5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsBadBBoxWestGeEast(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":52.0,"west":13.5,"north":52.5,"east":13.3}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsOutOfRangeCoordinates(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`{"south":-91.0,"west":13.3,"north":52.5,"east":13.5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportOSMEndpointRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/osm", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestImportTerrainEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	fixedTime := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	handler := newHandlerWithOptions(store, handlerOptions{
		clock:        func() time.Time { return fixedTime },
		corsDisabled: true,
	})

	// Build a minimal synthetic GeoTIFF (2x2, float32, 10m pixels, origin 100,200).
	geotiffData := buildTestGeoTIFF(t)

	body, contentType := createMultipartFile(t, "file", "terrain.tif", geotiffData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var info terrainInfoResponse
	decodeResponse(t, rec.Body.Bytes(), &info)

	if info.GridSize[0] != 2 || info.GridSize[1] != 2 {
		t.Errorf("expected grid 2x2, got %v", info.GridSize)
	}

	if info.PixelSize[0] != 10 || info.PixelSize[1] != 10 {
		t.Errorf("expected pixel size [10, 10], got %v", info.PixelSize)
	}

	// Verify artifact was registered.
	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	found := false

	for _, a := range proj.Artifacts {
		if a.ID == "artifact-terrain" {
			found = true

			if a.Kind != "model.terrain_geotiff" {
				t.Errorf("unexpected artifact kind: %q", a.Kind)
			}
		}
	}

	if !found {
		t.Error("artifact-terrain not found in project manifest")
	}
}

func TestImportTerrainRejectsBadExtension(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{corsDisabled: true})

	body, contentType := createMultipartFile(t, "file", "terrain.png", []byte("not a tiff"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImportTerrainRejectsInvalidGeoTIFF(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{corsDisabled: true})

	body, contentType := createMultipartFile(t, "file", "terrain.tif", []byte("not valid tiff data"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

type terrainInfoResponse struct {
	Bounds    [4]float64 `json:"bounds"`
	PixelSize [2]float64 `json:"pixel_size"`
	GridSize  [2]int     `json:"grid_size"`
}

func buildTestGeoTIFF(t *testing.T) []byte {
	t.Helper()

	order := binary.LittleEndian
	width, height := 2, 2
	pixels := []float32{10, 20, 30, 40}
	bytesPerPixel := 4
	pixelDataSize := width * height * bytesPerPixel

	pixelOffset := 8
	numTags := 9
	ifdOffset := pixelOffset + pixelDataSize
	ifdSize := 2 + numTags*12 + 4
	scaleDataOffset := ifdOffset + ifdSize
	tpDataOffset := scaleDataOffset + 24
	totalSize := tpDataOffset + 48

	buf := make([]byte, totalSize)

	// Header.
	buf[0] = 'I'
	buf[1] = 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	// Pixel data.
	for i, v := range pixels {
		order.PutUint32(buf[pixelOffset+i*4:], math.Float32bits(v))
	}

	// IFD.
	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(numTags))
	pos += 2

	writeTag := func(tag, dtype uint16, count uint32, value uint32) {
		order.PutUint16(buf[pos:], tag)
		order.PutUint16(buf[pos+2:], dtype)
		order.PutUint32(buf[pos+4:], count)
		order.PutUint32(buf[pos+8:], value)
		pos += 12
	}

	writeTag(256, 3, 1, uint32(width))              // ImageWidth
	writeTag(257, 3, 1, uint32(height))             // ImageLength
	writeTag(258, 3, 1, 32)                         // BitsPerSample
	writeTag(259, 3, 1, 1)                          // Compression = None
	writeTag(273, 4, 1, uint32(pixelOffset))        // StripOffsets
	writeTag(279, 4, 1, uint32(pixelDataSize))      // StripByteCounts
	writeTag(339, 3, 1, 3)                          // SampleFormat = Float
	writeTag(33550, 12, 3, uint32(scaleDataOffset)) // ModelPixelScale
	writeTag(33922, 12, 6, uint32(tpDataOffset))    // ModelTiepoint

	// Next IFD = 0.
	order.PutUint32(buf[pos:], 0)

	// Scale data: 10m x 10m.
	order.PutUint64(buf[scaleDataOffset:], math.Float64bits(10))
	order.PutUint64(buf[scaleDataOffset+8:], math.Float64bits(10))
	order.PutUint64(buf[scaleDataOffset+16:], 0)

	// Tiepoint: origin at (100, 200).
	order.PutUint64(buf[tpDataOffset:], 0)
	order.PutUint64(buf[tpDataOffset+8:], 0)
	order.PutUint64(buf[tpDataOffset+16:], 0)
	order.PutUint64(buf[tpDataOffset+24:], math.Float64bits(100))
	order.PutUint64(buf[tpDataOffset+32:], math.Float64bits(200))
	order.PutUint64(buf[tpDataOffset+40:], 0)

	return buf
}

func createMultipartFile(t *testing.T, fieldName, fileName string, data []byte) (io.Reader, string) {
	t.Helper()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	part, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	_, err = part.Write(data)
	if err != nil {
		t.Fatalf("write form file: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &b, w.FormDataContentType()
}

func decodeResponse(t *testing.T, payload []byte, out any) {
	t.Helper()

	err := json.Unmarshal(payload, out)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func waitForSSEEventData(body io.ReadCloser, timeout time.Duration, done func(seen map[string]string) bool) (map[string]string, error) {
	resultCh := make(chan map[string]string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		scanner := bufio.NewScanner(body)
		currentEvent := ""
		seen := make(map[string]string)

		for scanner.Scan() {
			line := scanner.Text()
			if after, ok := strings.CutPrefix(line, "event: "); ok {
				currentEvent = strings.TrimSpace(after)
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				if currentEvent == "" {
					continue
				}

				seen[currentEvent] = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				if done(seen) {
					resultCh <- seen
					return
				}
			}
		}

		err := scanner.Err()
		if err != nil {
			errCh <- err
			return
		}

		errCh <- errors.New("sse stream ended before expected events")
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-timer.C:
		_ = body.Close()
		return nil, errors.New("timed out waiting for sse events")
	}
}
