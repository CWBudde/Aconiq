package httpv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
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

// A second estimate cancels the first: the first answers 409, the second its
// estimate, and only one probe ever runs at a time.
func TestRunEstimateEndpointTakesOneEstimateAtATime(t *testing.T) {
	t.Parallel()

	store := estimateTestStore(t)
	firstStarted := make(chan struct{})

	var calls atomic.Int32

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		estimate: func(ctx context.Context, _ createRunRequest) (json.RawMessage, error) {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-ctx.Done()

				return nil, fmt.Errorf("estimate run: %w", ctx.Err())
			}

			return json.RawMessage(`{"available":true}`), nil
		},
	})

	post := func() *httptest.ResponseRecorder {
		req := newAPIRequest(http.MethodPost, "/api/v1/runs/estimate", strings.NewReader(`{"standard_id": "rls19-road"}`))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		return rec
	}

	firstDone := make(chan *httptest.ResponseRecorder)

	go func() { firstDone <- post() }()

	<-firstStarted

	second := post()
	first := <-firstDone

	if second.Code != http.StatusOK {
		t.Fatalf("second: status %d: %s", second.Code, second.Body.String())
	}

	if first.Code != http.StatusConflict || !strings.Contains(first.Body.String(), errorCodeEstimateSuperseded) {
		t.Fatalf("first: status %d: %s", first.Code, first.Body.String())
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

// The facade receiver fields and the unconfirmed opt-in reach argv for a run
// and an estimate alike. A building id goes in the `--flag=value` form, so one
// that starts with "-" is the flag's value and never a flag of its own.
func TestRunCommandArgsCarryFacadeModeAndAllowUnconfirmed(t *testing.T) {
	t.Parallel()

	spacing := 2.5
	req := createRunRequest{
		StandardID:             "rls19-road",
		ReceiverMode:           "facade",
		FacadeBuildings:        []string{"osm-way-2", "-b1"},
		FacadeSpacingM:         &spacing,
		FacadeIncludeCourtyard: true,
		AllowUnconfirmed:       true,
	}

	want := []string{
		"--standard", "rls19-road", "--receiver-mode", "facade",
		"--facade-building=osm-way-2", "--facade-building=-b1",
		"--facade-spacing-m", "2.5", "--facade-courtyard", "--allow-unconfirmed",
	}

	run := runCommandArgs("/p", "run-1", req)
	if got := run[5:]; !slices.Equal(got, want) {
		t.Fatalf("run args %v, want %v", got, want)
	}

	estimate := estimateCommandArgs("/p", req)
	if got := estimate[4 : len(estimate)-1]; !slices.Equal(got, want) {
		t.Fatalf("estimate args %v, want %v", got, want)
	}
}

// stdout under --json is the command's output and nothing else, so it is
// decoded as one document.
func TestReadEstimateOutputDecodesOneDocument(t *testing.T) {
	t.Parallel()

	stdout := `{
  "command": "run",
  "mode": "estimate",
  "estimate": {"available": false, "reason": "no estimate available"}
}
`

	estimate, err := readEstimateOutput([]byte(stdout))
	if err != nil || !strings.Contains(string(estimate), "no estimate available") {
		t.Fatalf("estimate %s, err %v", estimate, err)
	}
}

// A log record on stdout — what this decoder used to skip — or any second
// value is refused rather than skipped: it means stdout is no longer only the
// result.
func TestReadEstimateOutputRefusesAnythingButTheEstimate(t *testing.T) {
	t.Parallel()

	for name, stdout := range map[string]string{
		"log record first": `{"level":"INFO","msg":"command started"}` + "\n" + `{"mode":"estimate","estimate":{}}`,
		"log record after": `{"mode":"estimate","estimate":{}}` + "\n" + `{"level":"INFO","msg":"command finished"}`,
		"text after":       `{"mode":"estimate","estimate":{}}` + "\nEvidence tier: normative\n",
		"not an estimate":  `{"command":"run","mode":"run"}`,
		"empty":            ``,
		"cut off":          `{"mode":"estimate","estimate":{"avail`,
	} {
		_, err := readEstimateOutput([]byte(stdout))
		if err == nil {
			t.Errorf("%s: accepted %q", name, stdout)
		}
	}
}
