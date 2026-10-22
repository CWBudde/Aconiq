package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runRequestArgs is a complete start_run / estimate_run request for the
// standard given, with every field the schema requires.
func runRequestArgs(standardID string) map[string]any {
	return map[string]any{
		"scenario_id":      "default",
		"standard_id":      standardID,
		"standard_version": "v1",
		"standard_profile": "default",
		"receiver_mode":    "auto-grid",
	}
}

// seedArtifact writes a file under the project and records it as an artifact
// of the run, as `aconiq run` does when it persists its results.
func (f fixture) seedArtifact(t *testing.T, runID string, kind string, relPath string, data []byte) string {
	t.Helper()

	path := filepath.Join(f.store.Root(), filepath.FromSlash(relPath))

	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		t.Fatalf("create artifact directory: %v", err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	proj, err := f.store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	id := fmt.Sprintf("artifact-%s-%d", runID, len(proj.Artifacts)+1)
	proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
		ID: id, RunID: runID, Kind: kind, Path: relPath, CreatedAt: fixedNow,
	})

	err = f.store.Save(proj)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	return id
}

// receiverTable is a receiver table of n receivers in the JSON shape
// `aconiq run` writes.
func receiverTable(t *testing.T, n int) []byte {
	t.Helper()

	records := make([]map[string]any, 0, n)
	for i := range n {
		records = append(records, map[string]any{
			"id": fmt.Sprintf("R%05d", i+1), "x": 500000 + float64(i), "y": 5700000.0, "height_m": 4.0,
			"values": map[string]float64{"LrDay": 55.25, "LrNight": 45.75},
		})
	}

	data, err := json.Marshal(map[string]any{
		"indicator_order": []string{"LrDay", "LrNight"},
		"units":           map[string]string{"LrDay": "dB(A)", "LrNight": "dB(A)"},
		"records":         records,
	})
	if err != nil {
		t.Fatalf("encode receiver table: %v", err)
	}

	return data
}

// The run tools a read-only server offers are the read class; start_run and
// cancel_run change the project and appear only under --allow-writes.
func TestRunToolsFollowTheWriteGate(t *testing.T) {
	t.Parallel()

	reads := []string{"estimate_run", "get_contours", "get_run", "get_run_log", "get_run_results", "list_runs"}

	readOnly := listTools(t, newFixture(t, false).session)
	for _, name := range reads {
		tool, ok := readOnly[name]
		if !ok || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("read-only server: %s missing or not read-only (listed %v)", name, ok)
		}
	}

	for _, name := range []string{"start_run", "cancel_run"} {
		if _, ok := readOnly[name]; ok {
			t.Fatalf("read-only server lists %s", name)
		}
	}

	writable := listTools(t, newFixture(t, true).session)
	for _, name := range []string{"start_run", "cancel_run"} {
		tool, ok := writable[name]
		if !ok || tool.Annotations == nil || tool.Annotations.ReadOnlyHint {
			t.Fatalf("writable server: %s missing or read-only (listed %v)", name, ok)
		}

		if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Fatalf("%s is state-changing, not destructive: %+v", name, tool.Annotations)
		}
	}
}

// The method-selecting fields are required, so the choice of standard,
// version, profile and receiver mode is always the agent's own and never a
// default the CLI applied (ADR-002, security policy 6). No path is offered:
// the model is always the saved one.
func TestRunRequestSchemasRequireTheMethodFields(t *testing.T) {
	t.Parallel()

	tools := listTools(t, newFixture(t, true).session)

	for _, name := range []string{"start_run", "estimate_run"} {
		schema, err := json.Marshal(tools[name].InputSchema)
		if err != nil {
			t.Fatalf("encode %s schema: %v", name, err)
		}

		var decoded struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		}

		err = json.Unmarshal(schema, &decoded)
		if err != nil {
			t.Fatalf("decode %s schema: %v", name, err)
		}

		required := []string{"receiver_mode", "scenario_id", "standard_id", "standard_profile", "standard_version"}
		if slices.Sort(decoded.Required); !slices.Equal(decoded.Required, required) {
			t.Fatalf("%s requires %v, want %v", name, decoded.Required, required)
		}

		properties := []string{
			"allow_unconfirmed", "experimental", "facade_buildings", "facade_include_courtyard", "facade_spacing_m",
			"params", "receiver_mode", "scenario_id", "standard_id", "standard_profile", "standard_version",
		}
		if got := slices.Sorted(maps.Keys(decoded.Properties)); !slices.Equal(got, properties) {
			t.Fatalf("%s offers %v, want %v", name, got, properties)
		}
	}
}

func TestRunReadToolsAreTheAPIResponsesVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	for _, tc := range []struct {
		tool string
		args map[string]any
		path string
	}{
		{"list_runs", map[string]any{}, "/api/v1/runs"},
		{"get_run", map[string]any{"run_id": run.ID}, "/api/v1/runs/" + run.ID},
		{"get_run_log", map[string]any{"run_id": run.ID}, "/api/v1/runs/" + run.ID + "/log"},
		// A run with no raster is refused by the API, in its own words.
		{"get_contours", map[string]any{"run_id": run.ID, "interval": 2.5, "crs": "EPSG:4326"}, "/api/v1/runs/" + run.ID + "/contours?crs=EPSG%3A4326&interval=2.5"},
	} {
		result := f.call(t, tc.tool, tc.args)

		code, want := f.apiAnswer(t, http.MethodGet, tc.path, nil)
		if got := resultText(t, result); got != string(want) || result.IsError != (code >= http.StatusBadRequest) {
			t.Fatalf("%s differs from GET %s (%d, is_error %v):\n got %s\nwant %s", tc.tool, tc.path, code, result.IsError, got, want)
		}
	}
}

// An explicit interval reaches the API as given, zero included, so a bad one
// is the API's refusal and not a default this layer substituted.
func TestGetContoursPassesAnIntervalOfZeroThrough(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	result := f.call(t, "get_contours", map[string]any{"run_id": run.ID, "interval": 0})

	_, want := f.apiAnswer(t, http.MethodGet, "/api/v1/runs/"+run.ID+"/contours?interval=0", nil)
	if got := resultText(t, result); got != string(want) {
		t.Fatalf("get_contours interval 0:\n got %s\nwant %s", got, want)
	}
}

func TestUnknownRunsAreTheAPIsNotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)

	for _, tool := range []string{"get_run", "get_run_log", "get_contours", "get_run_results"} {
		apiErr := apiErrorOf(t, f.call(t, tool, map[string]any{"run_id": "run-nope"}))
		if apiErr["code"] != "not_found" {
			t.Fatalf("%s: envelope = %v", tool, apiErr)
		}
	}
}

// A run id is one path segment of its route, however it is spelled: no
// run_id makes a tool read the model the project does hold.
func TestRunIDArgumentsCannotLeaveTheirRoute(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	result := f.call(t, "save_model", saveArgs(validModel, "none"))
	if result.IsError {
		t.Fatalf("seed save: %s", resultText(t, result))
	}

	for _, id := range []string{"..", ".", "..%2F..%2Fmodel", "../../model", "a/b", ""} {
		for _, tool := range []string{"get_run", "get_run_log", "get_contours", "get_run_results", "cancel_run"} {
			result := f.call(t, tool, map[string]any{"run_id": id})

			text := resultText(t, result)
			if strings.Contains(text, "FeatureCollection") {
				t.Fatalf("%s run_id %q read the model: %s", tool, id, text)
			}

			// Not a redirect or any other route's answer: the id names no
			// run. cancel_run's schema refuses an empty id before that.
			want := "not_found"
			if tool == "cancel_run" && id == "" {
				want = "invalid_arguments"
			}

			if apiErr := apiErrorOf(t, result); apiErr["code"] != want {
				t.Fatalf("%s run_id %q: envelope = %v, want %s", tool, id, apiErr, want)
			}
		}
	}
}

func TestGetRunResultsReturnsASmallTableInline(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)
	table := receiverTable(t, 3)
	f.seedArtifact(t, run.ID, project.ArtifactKindRunResultReceiverTableJSON, ".noise/runs/"+run.ID+"/results/receivers.json", table)

	result := f.call(t, "get_run_results", map[string]any{"run_id": run.ID})
	if result.IsError || resultText(t, result) != string(table) || len(result.Content) != 1 {
		t.Fatalf("get_run_results: is_error %v, %d contents:\n%s", result.IsError, len(result.Content), resultText(t, result))
	}
}

// Past the inline cap the agent gets what it needs to read the table — its
// indicators, units and size — and a link to fetch it, not the table itself.
func TestGetRunResultsLinksALargeTable(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)
	table := receiverTable(t, 3000)

	if len(table) <= 256<<10 {
		t.Fatalf("fixture table is %d bytes, not past the inline cap", len(table))
	}

	artifactID := f.seedArtifact(t, run.ID, project.ArtifactKindRunResultReceiverTableJSON, ".noise/runs/"+run.ID+"/results/receivers.json", table)

	result := f.call(t, "get_run_results", map[string]any{"run_id": run.ID})
	if result.IsError || len(result.Content) != 2 {
		t.Fatalf("get_run_results: is_error %v, %d contents", result.IsError, len(result.Content))
	}

	var summary struct {
		RunID          string            `json:"run_id"`
		ArtifactID     string            `json:"artifact_id"`
		IndicatorOrder []string          `json:"indicator_order"`
		Units          map[string]string `json:"units"`
		RecordCount    int               `json:"record_count"`
		SizeBytes      int               `json:"size_bytes"`
		Resource       string            `json:"resource"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &summary)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	uri := "aconiq://artifacts/" + artifactID
	if summary.RunID != run.ID || summary.ArtifactID != artifactID || summary.RecordCount != 3000 ||
		summary.SizeBytes != len(table) || summary.Resource != uri ||
		!slices.Equal(summary.IndicatorOrder, []string{"LrDay", "LrNight"}) || summary.Units["LrNight"] != "dB(A)" {
		t.Fatalf("summary = %+v", summary)
	}

	link, ok := result.Content[1].(*mcp.ResourceLink)
	if !ok || link.URI != uri || link.Size == nil || *link.Size != int64(len(table)) {
		t.Fatalf("second content = %#v, want a resource link to %s", result.Content[1], uri)
	}

	if got := f.resourceText(t, uri); got != string(table) {
		t.Fatal("the linked resource is not the table")
	}
}

func TestGetRunResultsRefusesARunWithoutATable(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	apiErr := apiErrorOf(t, f.call(t, "get_run_results", map[string]any{"run_id": run.ID}))
	if apiErr["code"] != "results_not_found" || apiErr["hint"] == nil || !strings.Contains(fmt.Sprint(apiErr["message"]), run.Status) {
		t.Fatalf("envelope = %v", apiErr)
	}
}

func TestArtifactResourcesAreTheFilesVerbatim(t *testing.T) {
	t.Parallel()

	f := newFixture(t, false)
	run := f.seedRun(t)

	jsonData := []byte(`{"units":"dB(A)"}` + "\n")
	jsonID := f.seedArtifact(t, run.ID, project.ArtifactKindRunResultRasterMetadata, ".noise/runs/"+run.ID+"/results/raster.json", jsonData)

	binData := []byte{0, 0, 0, 0, 0, 0, 0xf0, 0x3f, 0xff, 0x00}
	binID := f.seedArtifact(t, run.ID, project.ArtifactKindRunResultRasterBinary, ".noise/runs/"+run.ID+"/results/raster.bin", binData)

	result, err := f.readResource(t, "aconiq://artifacts/"+jsonID)
	if err != nil || len(result.Contents) != 1 || result.Contents[0].Text != string(jsonData) ||
		!strings.HasPrefix(result.Contents[0].MIMEType, "application/json") {
		t.Fatalf("json artifact = %+v (%v)", result, err)
	}

	result, err = f.readResource(t, "aconiq://artifacts/"+binID)
	if err != nil || len(result.Contents) != 1 {
		t.Fatalf("read binary artifact: %v", err)
	}

	content := result.Contents[0]
	if !bytes.Equal(content.Blob, binData) || content.Text != "" || content.MIMEType != "application/octet-stream" {
		t.Fatalf("binary artifact = %+v", content)
	}

	for _, uri := range []string{"aconiq://artifacts/nope", "aconiq://artifacts/..", "aconiq://artifacts/..%2F..%2Fmodel"} {
		_, err := f.readResource(t, uri)
		assertResourceNotFound(t, err)
	}
}

// A refused start is audited with the code it was refused with, and what the
// schema refuses never reaches the API.
func TestStartRunRefusalsAreAudited(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	incomplete := runRequestArgs("dummy-freefield")
	delete(incomplete, "standard_profile")

	empty := runRequestArgs("dummy-freefield")
	empty["standard_profile"] = ""

	for _, args := range []map[string]any{incomplete, empty} {
		apiErr := apiErrorOf(t, f.call(t, "start_run", args))
		if apiErr["code"] != "invalid_arguments" {
			t.Fatalf("envelope = %v", apiErr)
		}
	}

	// A scaffold without the opt-in is the API's own gate.
	apiErr := apiErrorOf(t, f.call(t, "start_run", runRequestArgs("cnossos-road")))

	body, err := json.Marshal(runRequestArgs("cnossos-road"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	_, want := f.apiAnswer(t, http.MethodPost, "/api/v1/runs", body)

	var wantEnvelope struct {
		Error map[string]any `json:"error"`
	}

	err = json.Unmarshal(want, &wantEnvelope)
	if err != nil || apiErr["code"] == nil || apiErr["code"] != wantEnvelope.Error["code"] {
		t.Fatalf("gate envelope = %v, API = %s", apiErr, want)
	}

	proj, err := f.store.Load()
	if err != nil || len(proj.Runs) != 0 {
		t.Fatalf("a refused start left %d runs (%v)", len(proj.Runs), err)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 3 || lines[0]["tool"] != "start_run" || lines[0]["outcome"] != "invalid_arguments" ||
		lines[1]["outcome"] != "invalid_arguments" || lines[2]["outcome"] != wantEnvelope.Error["code"] {
		t.Fatalf("audit = %v", lines)
	}
}

// startRunAnswering wraps the API so that POST /api/v1/runs is answered by
// answer; every other route is the API's.
func startRunAnswering(answer http.HandlerFunc) func(http.Handler) http.Handler {
	return func(api http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs" {
				answer(w, r)

				return
			}

			api.ServeHTTP(w, r)
		})
	}
}

// The API writes nothing to a start request whose caller has gone away before
// the run's row exists, and the run computes regardless. So the request must
// not be one the call's cancellation can abandon: this stand-in abandons any
// request that could be, as the API would if the agent went away.
func TestStartRunCannotBeAbandonedByItsCaller(t *testing.T) {
	t.Parallel()

	f := newFixtureWrapped(t, true, startRunAnswering(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Done() != nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"run-1","status":"pending"}`))
	}))

	result := f.call(t, "start_run", runRequestArgs("dummy-freefield"))
	if result.IsError {
		t.Fatalf("start_run = %s", resultText(t, result))
	}

	lines := auditLines(t, f.store)
	if len(lines) != 1 || lines[0]["outcome"] != "ok" || lines[0]["run_id"] != "run-1" {
		t.Fatalf("audit = %v", lines)
	}
}

// The facade receiver set and the unconfirmed opt-in reach the API under the
// API's own field names, so the run they start is the one `aconiq run` would.
func TestStartRunForwardsTheFacadeSetAndTheUnconfirmedOptIn(t *testing.T) {
	t.Parallel()

	var forwarded map[string]any

	f := newFixtureWrapped(t, true, startRunAnswering(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&forwarded)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"run-1","status":"pending"}`))
	}))

	args := runRequestArgs("rls19-road")
	args["receiver_mode"] = "facade"
	args["facade_buildings"] = []any{"osm-way-1"}
	args["facade_spacing_m"] = 2.5
	args["facade_include_courtyard"] = true
	args["allow_unconfirmed"] = true

	result := f.call(t, "start_run", args)
	if result.IsError {
		t.Fatalf("start_run = %s", resultText(t, result))
	}

	for key, want := range map[string]any{
		"receiver_mode":            "facade",
		"facade_spacing_m":         2.5,
		"facade_include_courtyard": true,
		"allow_unconfirmed":        true,
	} {
		if forwarded[key] != want {
			t.Fatalf("forwarded %s = %v, want %v (body %v)", key, forwarded[key], want, forwarded)
		}
	}

	if buildings, ok := forwarded["facade_buildings"].([]any); !ok || len(buildings) != 1 || buildings[0] != "osm-way-1" {
		t.Fatalf("forwarded facade_buildings = %v", forwarded["facade_buildings"])
	}
}

// A success that names no run is not recorded as one.
func TestStartRunWithoutARunIDIsNotASuccess(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{"empty": "", "no id": `{"status":"pending"}`} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixtureWrapped(t, true, startRunAnswering(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(body))
			}))

			apiErr := apiErrorOf(t, f.call(t, "start_run", runRequestArgs("dummy-freefield")))
			if apiErr["code"] != "internal_error" {
				t.Fatalf("envelope = %v", apiErr)
			}

			lines := auditLines(t, f.store)
			if len(lines) != 1 || lines[0]["outcome"] != "internal_error" || lines[0]["run_id"] != nil {
				t.Fatalf("audit = %v", lines)
			}
		})
	}
}

// A run this server did not start is someone else's to stop; the refusal is
// the API's, and the audit line names the run.
func TestCancelRunOfAForeignRunIsRefusedAndAudited(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	// A run still open in the manifest, as one `aconiq run` in a terminal
	// is computing.
	run, _, err := f.store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
		Status:   project.RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	apiErr := apiErrorOf(t, f.call(t, "cancel_run", map[string]any{"run_id": run.ID}))
	if apiErr["code"] != "run_not_owned" {
		t.Fatalf("envelope = %v", apiErr)
	}

	lines := auditLines(t, f.store)
	if len(lines) != 1 || lines[0]["tool"] != "cancel_run" || lines[0]["outcome"] != "run_not_owned" || lines[0]["run_id"] != run.ID {
		t.Fatalf("audit = %v", lines)
	}
}

// No run is started or cancelled unaudited.
func TestRunWritesNeedTheAuditLog(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	run := f.seedRun(t)

	// A directory where the log file should be makes it unopenable.
	err := os.MkdirAll(f.store.MCPAuditLogPath(), 0o750)
	if err != nil {
		t.Fatalf("block audit log: %v", err)
	}

	for tool, args := range map[string]map[string]any{
		"start_run":  runRequestArgs("dummy-freefield"),
		"cancel_run": {"run_id": run.ID},
	} {
		apiErr := apiErrorOf(t, f.call(t, tool, args))
		if apiErr["code"] != "audit_unavailable" {
			t.Fatalf("%s: envelope = %v", tool, apiErr)
		}
	}

	proj, err := f.store.Load()
	if err != nil || len(proj.Runs) != 1 || proj.Runs[0].Status != run.Status {
		t.Fatalf("runs after refused writes: %+v (%v)", proj.Runs, err)
	}
}
