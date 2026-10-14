package cli

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cliChildArgsEnv carries the argv TestHelperProcessAconiq runs, as a JSON
// array. Unset, the helper is skipped like any other test.
const cliChildArgsEnv = "ACONIQ_CLI_CHILD_ARGS"

// TestHelperProcessAconiq is not a test: it is `aconiq` itself, Execute and
// os.Exit, for the tests below to start as a child. The stream contract is a
// property of the process — what lands on file descriptors 1 and 2 — and an
// in-process command with SetOut never sees a write that bypasses cobra.
func TestHelperProcessAconiq(t *testing.T) {
	raw := os.Getenv(cliChildArgsEnv)
	if raw == "" {
		t.Skip("helper process for the stdout contract tests")
	}

	var args []string

	err := json.Unmarshal([]byte(raw), &args)
	if err != nil {
		_, _ = os.Stderr.WriteString("bad " + cliChildArgsEnv + ": " + err.Error() + "\n")

		os.Exit(1)
	}

	os.Exit(Execute(args))
}

// runAconiqChild runs `aconiq args...` as a child process and returns its
// stdout, its stderr and its exit code.
func runAconiqChild(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperProcessAconiq$")

	cmd.Env = append(os.Environ(), cliChildArgsEnv+"="+string(encoded))

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	code := 0

	var exitErr *exec.ExitError
	if stderrors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("start aconiq %v: %v", args, err)
	}

	return stdout.String(), stderr.String(), code
}

// assertOneJSONDocument fails unless stdout is exactly one JSON document, and
// returns it.
func assertOneJSONDocument(t *testing.T, stdout string) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(stdout))

	var document map[string]any

	err := decoder.Decode(&document)
	if err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, stdout)
	}

	_, err = decoder.Token()
	if !stderrors.Is(err, io.EOF) {
		t.Fatalf("stdout carries more than one JSON document:\n%s", stdout)
	}

	if _, isLogRecord := document["level"]; isLogRecord {
		t.Fatalf("stdout is a log record, not the command's result:\n%s", stdout)
	}

	return document
}

// jsonLogRecords decodes stderr as JSON log records, one per line, and fails
// on a line that is not one.
func jsonLogRecords(t *testing.T, stderr string) []map[string]any {
	t.Helper()

	var records []map[string]any

	for line := range strings.SplitSeq(strings.TrimSpace(stderr), "\n") {
		var record map[string]any

		err := json.Unmarshal([]byte(line), &record)
		if err != nil {
			t.Fatalf("stderr line %q is not a JSON log record: %v", line, err)
		}

		records = append(records, record)
	}

	return records
}

func hasRecord(records []map[string]any, msg string) bool {
	for _, record := range records {
		if record["msg"] == msg {
			return true
		}
	}

	return false
}

// Under --json, stdout is the command's result — one JSON document — and the
// log records, JSON as well, are on stderr, as the flag's help says.
func TestJSONCommandWritesOneDocumentToStdoutAndItsLogToStderr(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Streams", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "phase17", "rls19_road_model.geojson"))

	for _, testCase := range []struct {
		name    string
		args    []string
		command string
		mode    string
	}{
		{name: "status", args: []string{"--project", projectDir, "--json", "status"}, command: "status"},
		{
			name: "run --estimate",
			args: []string{
				"--project", projectDir, "--json", "run", "--standard", "rls19-road",
				"--estimate", "--estimate-budget", "100ms", "--workers", "2",
			},
			command: "run",
			mode:    "estimate",
		},
		{
			name:    "openapi",
			args:    []string{"--project", projectDir, "--json", "openapi", "--out", filepath.Join(t.TempDir(), "openapi.v1.json")},
			command: "openapi",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := runAconiqChild(t, testCase.args...)
			if code != 0 {
				t.Fatalf("exit %d\nstderr:\n%s", code, stderr)
			}

			document := assertOneJSONDocument(t, stdout)
			if document["command"] != testCase.command {
				t.Fatalf("command = %v, want %q", document["command"], testCase.command)
			}

			if testCase.mode != "" && document["mode"] != testCase.mode {
				t.Fatalf("mode = %v, want %q", document["mode"], testCase.mode)
			}

			records := jsonLogRecords(t, stderr)
			if !hasRecord(records, "command started") || !hasRecord(records, "command finished") {
				t.Fatalf("stderr is missing the command's log records:\n%s", stderr)
			}
		})
	}
}

// A refused --json command prints no result, and its log — the error the
// HTTP API reads back to the run dialog — is on stderr.
func TestJSONCommandRefusalLeavesStdoutEmptyAndLogsTheErrorToStderr(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Refusal", "--crs", "EPSG:25832")

	stdout, stderr, code := runAconiqChild(t, "--project", projectDir, "--json", "run", "--standard", "rls19-road", "--estimate")
	if code != 2 {
		t.Fatalf("exit %d, want 2 for a user error\nstderr:\n%s", code, stderr)
	}

	if stdout != "" {
		t.Fatalf("a refused command wrote to stdout:\n%s", stdout)
	}

	records := jsonLogRecords(t, stderr)

	last := records[len(records)-1]
	if message, _ := last["error"].(string); message == "" {
		t.Fatalf("the last log record carries no error: %v", last)
	}
}

// Without --json the log is slog text, and it is on stderr too: stdout is the
// command's human-readable output alone.
func TestTextCommandKeepsItsLogOffStdout(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Text", "--crs", "EPSG:25832")

	stdout, stderr, code := runAconiqChild(t, "--project", projectDir, "status")
	if code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, stderr)
	}

	if strings.Contains(stdout, "level=") || strings.Contains(stdout, `msg="command`) {
		t.Fatalf("stdout carries log records:\n%s", stdout)
	}

	if !strings.Contains(stderr, `msg="command started"`) || !strings.Contains(stderr, `msg="command finished"`) {
		t.Fatalf("stderr is missing the command's log records:\n%s", stderr)
	}
}
