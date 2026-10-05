package httpv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

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

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{
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

func TestCreateRunEndpointRejectsArgumentInjection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{"absolute input path", `{"input_paths": ["/etc/passwd"]}`},
		{"escaping input path", `{"input_paths": ["../../../etc/passwd"]}`},
		{"flag-shaped input path", `{"input_paths": ["--project"]}`},
		{"absolute model path", `{"model_path": "/etc/passwd"}`},
		{"flag-shaped standard id", `{"standard_id": "--project"}`},
		{"flag-shaped param key", `{"params": {"--project": "x"}}`},
		{"newline in param value", "{\"params\": {\"road_speed_kph\": \"50\\n--project\"}}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := projectfs.New(t.TempDir())
			if err != nil {
				t.Fatalf("new store: %v", err)
			}

			_, err = store.Init("Run Validation Test", "EPSG:25832")
			if err != nil {
				t.Fatalf("init project: %v", err)
			}

			executed := false
			handler := newHandlerWithOptions(store, handlerOptions{
				clock: time.Now,
				runExecutor: func(_ context.Context, _ createRunRequest) error {
					executed = true

					return nil
				},
			})

			req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			if executed {
				t.Fatal("run executor was reached with an unvalidated request")
			}
		})
	}
}

func TestCreateRunEndpointAcceptsRelativeInputPaths(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Run Validation Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	var got createRunRequest

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, req createRunRequest) error {
			got = req

			return nil
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{
		"standard_id": "rls19-road",
		"receiver_mode": "auto-grid",
		"model_path": ".noise/model/normalized.geojson",
		"input_paths": ["inputs/terrain.tif"],
		"params": {"road_speed_kph": "50"}
	}`))
	req.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.ModelPath != ".noise/model/normalized.geojson" {
		t.Fatalf("valid request was rejected before reaching the executor: %+v", got)
	}
}

// The API refuses a scaffold-tier run that the caller has not acknowledged, and
// it refuses it before the executor is reached: no process is spawned, so no
// run can be persisted. The refusal names the standard and the tier rather than
// handing back a subprocess's prose.
func TestCreateRunEndpointRejectsScaffoldStandardWithoutExperimental(t *testing.T) {
	t.Parallel()

	store, registry := runGateFixture(t)

	executed := false
	handler := newHandlerWithOptions(store, handlerOptions{
		clock:    time.Now,
		registry: &registry,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			executed = true

			return nil
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "cnossos-road"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if executed {
		t.Fatal("run executor was reached for an unacknowledged scaffold-tier standard")
	}

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "experimental_opt_in_required" {
		t.Fatalf("error code = %q, want %q", response.Error.Code, "experimental_opt_in_required")
	}

	if response.Error.Details["standard_id"] != "cnossos-road" {
		t.Fatalf("error details standard_id = %v, want cnossos-road", response.Error.Details["standard_id"])
	}

	if response.Error.Details["evidence_tier"] != "scaffold" {
		t.Fatalf("error details evidence_tier = %v, want scaffold", response.Error.Details["evidence_tier"])
	}

	if !strings.Contains(response.Error.Hint, "experimental") {
		t.Fatalf("error hint does not name the field that proceeds: %q", response.Error.Hint)
	}
}

// With the acknowledgement given the request runs, and the executor sees the
// flag it has to forward to the run command.
func TestCreateRunEndpointRunsScaffoldStandardWithExperimental(t *testing.T) {
	t.Parallel()

	store, registry := runGateFixture(t)

	var got createRunRequest

	handler := newHandlerWithOptions(store, handlerOptions{
		clock:    time.Now,
		registry: &registry,
		runExecutor: func(_ context.Context, req createRunRequest) error {
			got = req

			_, _, err := store.CreateRun(projectfs.CreateRunSpec{
				ScenarioID: "default",
				Standard: project.StandardRef{
					ID:      "cnossos-road",
					Version: "0.1.0-preview",
					Profile: "default",
				},
				Status: project.RunStatusCompleted,
			})

			return err
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "cnossos-road", "experimental": true}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if !got.Experimental {
		t.Fatal("executor did not receive the experimental opt-in")
	}
}

// Only the scaffold tier is gated: a normative standard reaches the executor
// with no acknowledgement at all.
func TestCreateRunEndpointDoesNotGateNormativeStandards(t *testing.T) {
	t.Parallel()

	store, registry := runGateFixture(t)

	executed := false
	handler := newHandlerWithOptions(store, handlerOptions{
		clock:    time.Now,
		registry: &registry,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			executed = true

			return nil
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "rls19-road"}`))
	req.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !executed {
		t.Fatal("a normative standard was gated")
	}
}
