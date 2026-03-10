package httpv1

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
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
