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
