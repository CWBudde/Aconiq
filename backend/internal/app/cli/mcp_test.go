package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// mcpSession speaks newline-delimited JSON-RPC to an `aconiq mcp` over its
// stdin and stdout, one exchange at a time, so every response is read before
// stdin closes and the session ends.
type mcpSession struct {
	stdin  io.WriteCloser
	stdout *bufio.Reader
	nextID int
}

func (s *mcpSession) send(t *testing.T, message map[string]any) {
	t.Helper()

	message["jsonrpc"] = "2.0"

	line, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("encode %v: %v", message, err)
	}

	_, err = s.stdin.Write(append(line, '\n'))
	if err != nil {
		t.Fatalf("write to mcp stdin: %v", err)
	}
}

// request sends one call and returns its result, failing on a JSON-RPC error.
func (s *mcpSession) request(t *testing.T, method string, params any) map[string]any {
	t.Helper()

	s.nextID++
	s.send(t, map[string]any{"id": s.nextID, "method": method, "params": params})

	line, err := s.stdout.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read the %s response: %v", method, err)
	}

	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  map[string]any  `json:"result"`
		Error   json.RawMessage `json:"error"`
	}

	err = json.Unmarshal(line, &response)
	if err != nil {
		t.Fatalf("stdout line is not JSON-RPC: %q (%v)", line, err)
	}

	if response.JSONRPC != "2.0" || response.ID != s.nextID || response.Error != nil {
		t.Fatalf("%s answered %s", method, line)
	}

	return response.Result
}

func (s *mcpSession) initialize(t *testing.T) {
	t.Helper()

	s.request(t, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "test"},
	})
	s.send(t, map[string]any{"method": "notifications/initialized"})
}

func (s *mcpSession) toolNames(t *testing.T) []string {
	t.Helper()

	listed, _ := s.request(t, "tools/list", map[string]any{})["tools"].([]any)

	names := make([]string, 0, len(listed))

	for _, tool := range listed {
		entry, _ := tool.(map[string]any)
		name, _ := entry["name"].(string)
		names = append(names, name)
	}

	return names
}

func (s *mcpSession) callTool(t *testing.T, name string, args map[string]any) (string, bool) {
	t.Helper()

	result := s.request(t, "tools/call", map[string]any{"name": name, "arguments": args})

	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("%s returned no content: %v", name, result)
	}

	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	isError, _ := result["isError"].(bool)

	return text, isError
}

// `aconiq mcp` as a process: stdout carries JSON-RPC and nothing else — not
// the startup log, not the shutdown log — and stdin closing ends it cleanly.
// The stream contract is a property of file descriptors 1 and 2, so this runs
// the real binary as a child, as the other stdout contract tests do.
func TestMCPStdoutCarriesOnlyJSONRPC(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")

	encoded, err := json.Marshal([]string{"--project", projectDir, "--verbose", "mcp"})
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperProcessAconiq$")

	cmd.Env = append(os.Environ(), cliChildArgsEnv+"="+string(encoded))

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("start aconiq mcp: %v", err)
	}

	session := &mcpSession{stdin: stdin, stdout: bufio.NewReader(stdoutPipe)}
	session.initialize(t)

	if names := strings.Join(session.toolNames(t), ","); strings.Contains(names, "save_model") || !strings.Contains(names, "project_status") {
		t.Fatalf("a read-only server lists %s", names)
	}

	text, isError := session.callTool(t, "project_status", map[string]any{})
	if isError || !strings.Contains(text, `"name": "MCP"`) {
		t.Fatalf("project_status = %s (error %v)", text, isError)
	}

	err = stdin.Close()
	if err != nil {
		t.Fatalf("close stdin: %v", err)
	}

	rest, err := io.ReadAll(session.stdout)
	if err != nil {
		t.Fatalf("drain stdout: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		t.Fatalf("aconiq mcp did not exit cleanly on EOF: %v\nstderr:\n%s", err, stderr.String())
	}

	if len(rest) != 0 {
		t.Fatalf("stdout carried more than the responses: %q", rest)
	}

	if !strings.Contains(stderr.String(), "mcp started") || !strings.Contains(stderr.String(), "mcp stopped") {
		t.Fatalf("the log did not reach stderr:\n%s", stderr.String())
	}
}

// runMCPInProcess runs `aconiq mcp args...` in this process over pipes and
// returns the session and a channel that yields the command's error.
func runMCPInProcess(t *testing.T, args ...string) (*mcpSession, <-chan error) {
	t.Helper()

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	cmd := newRootCommand()
	cmd.SetArgs(args)
	cmd.SetIn(stdinReader)
	cmd.SetOut(stdoutWriter)
	cmd.SetErr(io.Discard)

	done := make(chan error, 1)

	go func() {
		err := cmd.ExecuteContext(t.Context())
		_ = stdoutWriter.Close()

		done <- err
	}()

	return &mcpSession{stdin: stdinWriter, stdout: bufio.NewReader(stdoutReader)}, done
}

// mcpExitTimeout bounds how long a test waits for the command to end once its
// stdin is closed, so a server that does not stop fails the test rather than
// hanging the suite.
const mcpExitTimeout = 30 * time.Second

// finish closes stdin, drains whatever else the command writes to stdout —
// a server blocked on an unread pipe could never end — and returns the
// bytes drained and the command's error.
func (s *mcpSession) finish(t *testing.T, done <-chan error) ([]byte, error) {
	t.Helper()

	_ = s.stdin.Close()

	drained := make(chan []byte, 1)

	go func() {
		rest, _ := io.ReadAll(s.stdout)
		drained <- rest
	}()

	select {
	case err := <-done:
		return <-drained, err
	case <-time.After(mcpExitTimeout):
		t.Fatal("aconiq mcp did not stop after stdin closed")

		return nil, nil
	}
}

// The tools that change the project exist only under --allow-writes, and the
// write they offer is audited into the project's own log.
func TestMCPAllowWritesOffersAnAuditedModelSave(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")

	session, done := runMCPInProcess(t, "--project", projectDir, "mcp", "--allow-writes")
	session.initialize(t)

	if names := strings.Join(session.toolNames(t), ","); !strings.Contains(names, "save_model") {
		t.Fatalf("a writable server lists %s", names)
	}

	model := `{"type":"FeatureCollection","features":[{"type":"Feature","id":"rcv-1",` +
		`"properties":{"kind":"receiver","height_m":4},"geometry":{"type":"Point","coordinates":[500050,5700050]}}]}`

	text, isError := session.callTool(t, "save_model", map[string]any{"model": json.RawMessage(model), "confirm": "none"})
	if isError {
		t.Fatalf("save_model: %s", text)
	}

	rest, err := session.finish(t, done)
	if err != nil || len(rest) != 0 {
		t.Fatalf("aconiq mcp: %v, trailing stdout %q", err, rest)
	}

	audit, err := os.ReadFile(filepath.Join(projectDir, ".noise", "logs", "mcp-audit.jsonl"))
	if err != nil || strings.Count(string(audit), "\n") != 1 || !strings.Contains(string(audit), `"outcome":"ok"`) {
		t.Fatalf("audit log = %q (%v)", audit, err)
	}
}

// A run a previous process left open is closed when the server starts, as
// `aconiq serve` closes it: nothing else would ever come back to it.
func TestMCPClosesRunsAPreviousProcessLeftOpen(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	proj.Runs = append(proj.Runs, project.Run{
		ID:       "run-orphan",
		Status:   project.RunStatusRunning,
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
	})

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	session, done := runMCPInProcess(t, "--project", projectDir, "mcp")

	rest, err := session.finish(t, done)
	if err != nil || len(rest) != 0 {
		t.Fatalf("aconiq mcp: %v, trailing stdout %q", err, rest)
	}

	proj, err = store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if proj.Runs[0].Status != project.RunStatusFailed {
		t.Fatalf("orphaned run status = %s, want failed", proj.Runs[0].Status)
	}
}

func TestMCPRefusesArguments(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")

	err := runCLI("--project", projectDir, "mcp", "other-project")
	if err == nil {
		t.Fatal("aconiq mcp accepted a positional argument")
	}
}

// A run's provenance reaches the agent through the real command as the run
// recorded it, with the CRS pair and the build beside the standard, and a
// read-only server offers it.
func TestMCPServesARunsProvenance(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "phase8", "model.geojson"))
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	proj, err := store.Load()
	if err != nil || len(proj.Runs) != 1 {
		t.Fatalf("load: %d runs (%v)", len(proj.Runs), err)
	}

	run := proj.Runs[0]

	want, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	session, done := runMCPInProcess(t, "--project", projectDir, "mcp")
	session.initialize(t)

	uri := "aconiq://runs/" + run.ID + "/provenance"
	contents, _ := session.request(t, "resources/read", map[string]any{"uri": uri})["contents"].([]any)

	if len(contents) != 1 {
		t.Fatalf("%s: %d contents", uri, len(contents))
	}

	first, _ := contents[0].(map[string]any)
	if text, _ := first["text"].(string); text != string(want) {
		t.Fatalf("%s differs from provenance.json:\n got %s\nwant %s", uri, text, want)
	}

	var provenance project.ProvenanceManifest

	err = json.Unmarshal(want, &provenance)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["project_crs"] != "EPSG:25832" || provenance.Metadata["compute_crs"] == "" ||
		provenance.Metadata["evidence_tier"] == "" || provenance.ToolVersion == "" {
		t.Fatalf("provenance lacks the CRS pair, tier or build: %+v", provenance)
	}

	rest, err := session.finish(t, done)
	if err != nil || len(rest) != 0 {
		t.Fatalf("aconiq mcp: %v, trailing stdout %q", err, rest)
	}
}

// mcpRunTimeout bounds how long a test polls get_run for a run to finish.
const mcpRunTimeout = 60 * time.Second

// A run started over MCP is a real `aconiq run` child of the server, and an
// agent follows it to its results with the read tools alone: get_run until it
// has finished, then its receiver table, contours and log.
func TestMCPStartsARunAndReadsItsResults(t *testing.T) {
	// The API spawns the run as a child of this process, and the stand-in
	// makes this test binary that child. t.Setenv rules out t.Parallel.
	t.Setenv(cliStandInEnv, "1")

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "MCP", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "phase8", "model.geojson"))

	session, done := runMCPInProcess(t, "--project", projectDir, "mcp", "--allow-writes")
	session.initialize(t)

	request := map[string]any{
		"scenario_id":      "default",
		"standard_id":      "dummy-freefield",
		"standard_version": "v0",
		"standard_profile": "default",
		"receiver_mode":    "auto-grid",
	}

	text, isError := session.callTool(t, "start_run", request)
	if isError {
		t.Fatalf("start_run: %s", text)
	}

	var started struct {
		ID string `json:"id"`
	}

	err := json.Unmarshal([]byte(text), &started)
	if err != nil || started.ID == "" {
		t.Fatalf("start_run answered %s (%v)", text, err)
	}

	status := awaitMCPRun(t, session, started.ID)
	if status != project.RunStatusCompleted {
		log, _ := session.callTool(t, "get_run_log", map[string]any{"run_id": started.ID})
		t.Fatalf("run %s ended %s:\n%s", started.ID, status, log)
	}

	text, isError = session.callTool(t, "get_run_results", map[string]any{"run_id": started.ID})

	var table struct {
		IndicatorOrder []string          `json:"indicator_order"`
		Units          map[string]string `json:"units"`
		Records        []json.RawMessage `json:"records"`
	}

	err = json.Unmarshal([]byte(text), &table)
	if isError || err != nil || len(table.IndicatorOrder) == 0 || len(table.Records) == 0 ||
		table.Units[table.IndicatorOrder[0]] == "" {
		t.Fatalf("get_run_results (is_error %v, %v): %.300s", isError, err, text)
	}

	text, isError = session.callTool(t, "get_contours", map[string]any{"run_id": started.ID, "interval": 5})

	var contours struct {
		CRS   string            `json:"crs"`
		Lines []json.RawMessage `json:"lines"`
	}

	err = json.Unmarshal([]byte(text), &contours)
	if isError || err != nil || contours.CRS == "" {
		t.Fatalf("get_contours (is_error %v, %v): %.300s", isError, err, text)
	}

	for _, tool := range []string{"get_run_log", "list_runs"} {
		args := map[string]any{"run_id": started.ID}
		if tool == "list_runs" {
			args = map[string]any{}
		}

		if text, isError := session.callTool(t, tool, args); isError || !strings.Contains(text, started.ID) {
			t.Fatalf("%s (is_error %v): %.300s", tool, isError, text)
		}
	}

	// A finished run has nothing left to stop; the refusal is the API's.
	if text, isError := session.callTool(t, "cancel_run", map[string]any{"run_id": started.ID}); !isError || !strings.Contains(text, "run_not_running") {
		t.Fatalf("cancel_run of a finished run (is_error %v): %s", isError, text)
	}

	// An estimate's timings are measured and differ between calls (ADR-002,
	// Deterministic outputs), so only what is stable is asserted: which
	// standard it priced, whether it could, and that it created no run.
	text, isError = session.callTool(t, "estimate_run", request)

	var estimate struct {
		StandardID string `json:"standard_id"`
		Available  *bool  `json:"available"`
	}

	err = json.Unmarshal([]byte(text), &estimate)
	if isError || err != nil || estimate.StandardID != "dummy-freefield" || estimate.Available == nil {
		t.Fatalf("estimate_run (is_error %v, %v): %s", isError, err, text)
	}

	if text, _ := session.callTool(t, "list_runs", map[string]any{}); strings.Count(text, `"log_path"`) != 1 {
		t.Fatalf("an estimate created a run: %.300s", text)
	}

	rest, err := session.finish(t, done)
	if err != nil || len(rest) != 0 {
		t.Fatalf("aconiq mcp: %v, trailing stdout %q", err, rest)
	}

	assertAuditedRunTools(t, projectDir, started.ID)
}

// awaitMCPRun polls get_run, as an agent does, until the run has ended, and
// returns its final status.
func awaitMCPRun(t *testing.T, session *mcpSession, runID string) string {
	t.Helper()

	deadline := time.Now().Add(mcpRunTimeout)

	for time.Now().Before(deadline) {
		text, isError := session.callTool(t, "get_run", map[string]any{"run_id": runID})
		if isError {
			t.Fatalf("get_run: %s", text)
		}

		var run struct {
			Status string `json:"status"`
		}

		err := json.Unmarshal([]byte(text), &run)
		if err != nil {
			t.Fatalf("get_run answered %s (%v)", text, err)
		}

		if run.Status != project.RunStatusPending && run.Status != project.RunStatusRunning {
			return run.Status
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("run %s did not finish within %s", runID, mcpRunTimeout)

	return ""
}

// assertAuditedRunTools checks the audit log recorded the start, with the run
// it started, and the refused cancel, with the code it was refused with.
func assertAuditedRunTools(t *testing.T, projectDir string, runID string) {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	data, err := os.ReadFile(store.MCPAuditLogPath())
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}

	var entries []map[string]any

	for line := range strings.Lines(string(data)) {
		var entry map[string]any

		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Fatalf("audit line is not JSON: %q (%v)", line, err)
		}

		entries = append(entries, entry)
	}

	if len(entries) != 2 ||
		entries[0]["tool"] != "start_run" || entries[0]["outcome"] != "ok" || entries[0]["run_id"] != runID ||
		entries[1]["tool"] != "cancel_run" || entries[1]["outcome"] != "run_not_running" || entries[1]["run_id"] != runID {
		t.Fatalf("audit = %v", entries)
	}
}
