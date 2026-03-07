package httpv1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	if _, err := store.Init("Phase23 API", "EPSG:25832"); err != nil {
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
	if _, err := store.Init("Runs Test", "EPSG:25832"); err != nil {
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

func TestRunLogEndpointReturnsNotFoundForUnknownRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := store.Init("Runs Log Test", "EPSG:25832"); err != nil {
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
	}
	if !found {
		t.Fatal("expected rls19-road in standards list")
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
	if _, err := store.Init("Phase23 Stream", "EPSG:25832"); err != nil {
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
	if err := json.Unmarshal([]byte(eventData["project_status"]), &statusPayload); err != nil {
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
	if err := json.Unmarshal([]byte(eventData["project_status"]), &statusPayload); err != nil {
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

func decodeResponse(t *testing.T, payload []byte, out any) {
	t.Helper()

	if err := json.Unmarshal(payload, out); err != nil {
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
			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
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
		if err := scanner.Err(); err != nil {
			errCh <- err
			return
		}
		errCh <- fmt.Errorf("sse stream ended before expected events")
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
		return nil, fmt.Errorf("timed out waiting for sse events")
	}
}
