package httpv1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
)

func estimateTestStore(t *testing.T) projectfs.Store {
	t.Helper()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Estimate Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	return store
}

func TestRunEstimateEndpointAnswersWithTheEstimate(t *testing.T) {
	t.Parallel()

	store := estimateTestStore(t)

	var got createRunRequest

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		estimate: func(_ context.Context, req createRunRequest) (json.RawMessage, error) {
			got = req

			return json.RawMessage(`{"available":true,"standard_id":"rls19-road","wall_s":{"low":1,"estimate":2,"high":3}}`), nil
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs/estimate", strings.NewReader(`{
		"standard_id": "rls19-road",
		"params": {"grid_resolution_m": "20"}
	}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Available bool `json:"available"`
		Wall      struct {
			Estimate float64 `json:"estimate"`
		} `json:"wall_s"`
	}
	decodeResponse(t, rec.Body.Bytes(), &body)

	if !body.Available || body.Wall.Estimate != 2 {
		t.Fatalf("body %s", rec.Body.String())
	}

	if got.StandardID != "rls19-road" || got.Params["grid_resolution_m"] != "20" {
		t.Fatalf("executor got %+v", got)
	}

	// No run was started.
	proj, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(proj.Runs) != 0 {
		t.Fatalf("an estimate created %d runs", len(proj.Runs))
	}
}

func TestRunEstimateEndpointRefusesLikeARun(t *testing.T) {
	t.Parallel()

	store := estimateTestStore(t)

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		estimate: func(context.Context, createRunRequest) (json.RawMessage, error) {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", "segment_length_mode must be one of fixed, distance_scaled", nil)
		},
	})

	for _, tc := range []struct {
		name   string
		method string
		body   string
		status int
	}{
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"invalid identifier", http.MethodPost, `{"standard_id": "-rf"}`, http.StatusBadRequest},
		{"the run command's refusal", http.MethodPost, `{"standard_id": "rls19-road"}`, http.StatusBadRequest},
	} {
		req := newAPIRequest(tc.method, "/api/v1/runs/estimate", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tc.status {
			t.Fatalf("%s: status %d, want %d: %s", tc.name, rec.Code, tc.status, rec.Body.String())
		}
	}
}

func TestEstimateCommandArgsAskForJSONAndNoRun(t *testing.T) {
	t.Parallel()

	args := estimateCommandArgs("/p", createRunRequest{
		StandardID: "rls19-road",
		Params:     map[string]string{"b": "2", "a": "1"},
	})

	want := []string{
		"--project", "/p", "--json", "run", "--standard", "rls19-road",
		"--param", "a=1", "--param", "b=2", "--estimate",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args %v, want %v", args, want)
	}
}

func TestReadEstimateOutputSkipsTheLogRecords(t *testing.T) {
	t.Parallel()

	stdout := `{"time":"t","level":"INFO","msg":"command started"}
{
  "command": "run",
  "mode": "estimate",
  "estimate": {"available": false, "reason": "no estimate available"}
}
{"time":"t","level":"INFO","msg":"command finished"}
`

	estimate, message := readEstimateOutput([]byte(stdout))
	if !strings.Contains(string(estimate), "no estimate available") || message != "" {
		t.Fatalf("estimate %s, message %q", estimate, message)
	}

	_, message = readEstimateOutput([]byte(`{"msg":"command finished","error":"cli.run: model file not found"}` + "\n"))
	if message != "cli.run: model file not found" {
		t.Fatalf("message %q", message)
	}
}
