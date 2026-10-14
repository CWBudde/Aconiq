package httpv1

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// childModeEnv selects what TestHelperProcessCLIChild does when it is started
// as a child. Unset, it is skipped like any other test.
const childModeEnv = "ACONIQ_HTTPV1_CLI_CHILD"

// childRefusal is the error the refusing children log, quotes and all.
const childRefusal = `cli.loadValidatedModel: model file not found: open "model.geojson": no such file or directory`

// TestHelperProcessCLIChild is not a test: it is the child the tests below
// start, standing in for `aconiq run` with each of its stream layouts — log
// records on stderr, the command's output on stdout.
func TestHelperProcessCLIChild(t *testing.T) {
	mode := os.Getenv(childModeEnv)
	if mode == "" {
		t.Skip("helper process for the run and estimate child-process tests")
	}

	quoted := quoteJSON(childRefusal)

	switch mode {
	case "estimate":
		// What `aconiq --json run --estimate` writes: JSON records on
		// stderr, before and after the indented output on stdout.
		writeChild(os.Stderr, `{"time":"t","level":"INFO","msg":"command started","command":"aconiq run"}`+"\n")
		writeChild(os.Stdout, "{\n  \"command\": \"run\",\n  \"mode\": \"estimate\",\n"+
			"  \"estimate\": {\n    \"available\": true,\n    \"standard_id\": \"rls19-road\"\n  }\n}\n")
		writeChild(os.Stderr, `{"time":"t","level":"INFO","msg":"command finished","status":"ok"}`+"\n")
		os.Exit(0)
	case "estimate-refused":
		writeChild(os.Stderr, `{"time":"t","level":"INFO","msg":"command started"}`+"\n")
		writeChild(os.Stderr, `{"time":"t","level":"ERROR","msg":"command finished","status":"error","error":`+quoted+"}\n")
		writeChild(os.Stderr, `{"time":"t","level":"ERROR","msg":"command failed","user_error":true,"error":`+quoted+"}\n")
		os.Exit(2)
	case "run-refused":
		// What `aconiq run` writes without --json: text records on stderr,
		// the evidence-tier banner on stdout.
		writeChild(os.Stderr, `time=t level=INFO msg="command started" command="aconiq run" args=[]`+"\n")
		writeChild(os.Stdout, "Evidence tier: normative\n")
		writeChild(os.Stderr, `time=t level=ERROR msg="command finished" status=error error=`+quoted+"\n")
		writeChild(os.Stderr, `time=t level=ERROR msg="command failed" user_error=true error=`+quoted+"\n")
		os.Exit(2)
	case "run-killed":
		// A run killed mid-computation — out of memory, or past the
		// interrupt grace — after a warning that carries an error attribute
		// but did not fail the run (a terrain DTM that would not load).
		writeChild(os.Stderr, `time=t level=INFO msg="command started" command="aconiq run" args=[]`+"\n")
		writeChild(os.Stderr, `time=t level=WARN msg="terrain DTM load failed, continuing without terrain" error="open dtm.tif: no such file or directory"`+"\n")
		killSelf()
	case "run-panicked":
		// What the Go runtime writes for an unrecovered panic, after the
		// records the command had logged: the panic line, then the trace,
		// and exit status 2.
		writeChild(os.Stderr, `time=t level=INFO msg="command started" command="aconiq run" args=[]`+"\n")
		writeChild(os.Stderr, childPanic+"\n\ngoroutine 1 [running]:\nmain.main()\n\t/src/main.go:12 +0x1d\nexit status 2\n")
		os.Exit(2)
	}

	os.Exit(1)
}

// childPanic is the panic line the panicking child writes.
const childPanic = "panic: runtime error: index out of range [3] with length 3"

// killSelf ends this process the way the kernel's OOM killer or a SIGKILL
// would: without a word on any stream.
func killSelf() {
	self, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = self.Kill()
	}

	time.Sleep(time.Minute)
	os.Exit(1)
}

func writeChild(stream *os.File, text string) {
	_, _ = stream.WriteString(text)
}

// quoteJSON quotes a string the way both slog handlers quote one that needs
// it: a JSON string is also a valid Go-quoted text attribute for the
// characters used here.
func quoteJSON(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return string(encoded)
}

// newCLIChild prepares this test binary as a run child in the given mode, the
// way the executors prepare `aconiq run`.
func newCLIChild(t *testing.T, mode string) *exec.Cmd {
	t.Helper()

	cmd := newRunProcess(t.Context(), os.Args[0], []string{"-test.run=^TestHelperProcessCLIChild$"})

	cmd.Env = append(os.Environ(), childModeEnv+"="+mode)

	return cmd
}

// assertRefusalInTheCommandsWords checks err is a user error whose message is
// the child's own, unquoted — what the API answers a refused run with.
func assertRefusalInTheCommandsWords(t *testing.T, err error) {
	t.Helper()

	var appErr *domainerrors.AppError
	if !stderrors.As(err, &appErr) {
		t.Fatalf("error %v is not an AppError", err)
	}

	if appErr.Kind != domainerrors.KindUserInput || appErr.Msg != childRefusal {
		t.Fatalf("kind %q, message %q; want a user error saying %q", appErr.Kind, appErr.Msg, childRefusal)
	}
}

// A child that logs to stderr and prints the estimate to stdout is answered
// with the estimate, decoded from stdout alone.
func TestEstimateProcessReadsTheEstimateFromStdoutAndLogsFromStderr(t *testing.T) {
	t.Parallel()

	estimate, err := awaitEstimateProcess(newCLIChild(t, "estimate"))
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	var body struct {
		Available  bool   `json:"available"`
		StandardID string `json:"standard_id"`
	}

	err = json.Unmarshal(estimate, &body)
	if err != nil || !body.Available || body.StandardID != "rls19-road" {
		t.Fatalf("estimate %s (%v)", estimate, err)
	}
}

// A refused estimate is answered in the run command's words, read from the
// JSON records on its stderr.
func TestEstimateProcessAnswersARefusalInTheCommandsWords(t *testing.T) {
	t.Parallel()

	_, err := awaitEstimateProcess(newCLIChild(t, "estimate-refused"))
	assertRefusalInTheCommandsWords(t, err)
}

// A refused run is answered in the run command's words, read from the text
// records on its stderr — the failure reason the run dialog shows.
func TestRunProcessAnswersARefusalInTheCommandsWords(t *testing.T) {
	t.Parallel()

	err := awaitRunProcess(newCLIChild(t, "run-refused"))
	assertRefusalInTheCommandsWords(t, err)
}

// A killed child says nothing, and its log ends with the records it wrote
// while it was well — here a warning with an error attribute that did not
// fail the run. Neither is the failure: the reason is the process's own exit.
func TestRunProcessKilledReportsTheSignalNotItsLastLogRecord(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("a killed process exits with a status, not a signal, on Windows")
	}

	err := awaitRunProcess(newCLIChild(t, "run-killed"))
	if err == nil {
		t.Fatal("a killed run returned no error")
	}

	if domainerrors.IsUserError(err) {
		t.Fatalf("a killed run is not a user error: %v", err)
	}

	if err.Error() != "execute run command: signal: killed" {
		t.Fatalf("error %q, want the signal", err)
	}
}

// A panicking child's reason is its panic line — the first line of which the
// run supervisor's note keeps — not a log record and not the trace. A panic
// exits with status 2, as a refusal does, but it is not the user's doing.
func TestRunProcessPanickedReportsThePanicLine(t *testing.T) {
	t.Parallel()

	err := awaitRunProcess(newCLIChild(t, "run-panicked"))
	if err == nil {
		t.Fatal("a panicked run returned no error")
	}

	if domainerrors.IsUserError(err) {
		t.Fatalf("a panic is not a user error: %v", err)
	}

	if err.Error() != "execute run command: "+childPanic {
		t.Fatalf("error %q, want the panic line", err)
	}
}
