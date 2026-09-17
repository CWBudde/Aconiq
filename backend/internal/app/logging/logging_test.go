package logging_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/app/logging"
)

// captureStdout runs fn with os.Stdout replaced by a pipe and returns what fn
// wrote to it.
//
// The handlers this package builds bind os.Stdout at construction time, so the
// swap has to be in place before New is called — which is why fn takes the
// whole construct-and-log sequence rather than just the logging part.
//
// Tests using this cannot run in parallel: os.Stdout is process-wide.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer

	// Drain concurrently so a log line larger than the pipe buffer cannot
	// deadlock the test.
	captured := make(chan string, 1)

	go func() {
		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			captured <- ""

			return
		}

		captured <- string(data)
	}()

	func() {
		defer func() {
			os.Stdout = original

			_ = writer.Close()
		}()

		fn()
	}()

	out := <-captured

	_ = reader.Close()

	return out
}

// decodeJSONLines parses every non-empty line of out as a JSON log record.
func decodeJSONLines(t *testing.T, out string) []map[string]any {
	t.Helper()

	records := make([]map[string]any, 0, 2)

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var record map[string]any

		err := json.Unmarshal([]byte(line), &record)
		if err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}

		records = append(records, record)
	}

	return records
}

// The --json flag has to change the encoding, not just the wording: a caller
// piping the log into jq must get one JSON object per record, and a caller
// reading it must get slog's key=value text.
func TestNewSelectsHandlerByJSONFlag(t *testing.T) {
	jsonOut := captureStdout(t, func() {
		logging.New(slog.LevelInfo, true).Info("hello", slog.String("where", "there"))
	})

	records := decodeJSONLines(t, jsonOut)
	if len(records) != 1 {
		t.Fatalf("expected one JSON record, got %d from %q", len(records), jsonOut)
	}

	if records[0]["msg"] != "hello" || records[0]["where"] != "there" {
		t.Fatalf("JSON record lost its fields: %#v", records[0])
	}

	if records[0]["level"] != "INFO" {
		t.Fatalf("JSON record level = %v, want INFO", records[0]["level"])
	}

	textOut := captureStdout(t, func() {
		logging.New(slog.LevelInfo, false).Info("hello", slog.String("where", "there"))
	})

	if strings.HasPrefix(strings.TrimSpace(textOut), "{") {
		t.Fatalf("text handler emitted JSON: %q", textOut)
	}

	if !strings.Contains(textOut, `msg=hello`) || !strings.Contains(textOut, `where=there`) {
		t.Fatalf("text record lost its fields: %q", textOut)
	}
}

// The level argument is the one thing standing between `aconiq run` without
// --verbose and a wall of debug output, so it has to actually filter.
func TestNewHonoursTheLevelThreshold(t *testing.T) {
	cases := []struct {
		name      string
		level     slog.Level
		wantDebug bool
	}{
		{name: "info suppresses debug", level: slog.LevelInfo, wantDebug: false},
		{name: "debug admits debug", level: slog.LevelDebug, wantDebug: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				logger := logging.New(testCase.level, true)
				logger.Debug("quiet")
				logger.Info("loud")
			})

			records := decodeJSONLines(t, out)

			var sawDebug, sawInfo bool

			for _, record := range records {
				switch record["msg"] {
				case "quiet":
					sawDebug = true
				case "loud":
					sawInfo = true
				}
			}

			if sawDebug != testCase.wantDebug {
				t.Fatalf("debug record present = %t, want %t (output %q)", sawDebug, testCase.wantDebug, out)
			}

			if !sawInfo {
				t.Fatalf("info record missing at level %v: %q", testCase.level, out)
			}
		})
	}
}

// runIDPattern is what newRunID produces: eight random bytes, hex-encoded.
var runIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Begin/End are what makes a command's log lines correlatable after the fact:
// both records must carry the same run_id and the same command name, and the
// run ID must look like a run ID rather than the "runid-unavailable" fallback.
func TestBeginAndEndBracketACommandWithOneRunID(t *testing.T) {
	var run logging.CommandRun

	out := captureStdout(t, func() {
		run = logging.Begin(logging.New(slog.LevelInfo, true), "run", []string{"--standard", "rls19-road"})
		run.End(nil)
	})

	records := decodeJSONLines(t, out)
	if len(records) != 2 {
		t.Fatalf("expected a start and a finish record, got %d from %q", len(records), out)
	}

	start, finish := records[0], records[1]

	if start["msg"] != "command started" || finish["msg"] != "command finished" {
		t.Fatalf("unexpected messages: %v / %v", start["msg"], finish["msg"])
	}

	if start["run_id"] != finish["run_id"] {
		t.Fatalf("run_id differs between start (%v) and finish (%v)", start["run_id"], finish["run_id"])
	}

	if start["run_id"] != run.RunID {
		t.Fatalf("logged run_id %v does not match CommandRun.RunID %q", start["run_id"], run.RunID)
	}

	if !runIDPattern.MatchString(run.RunID) {
		t.Fatalf("RunID = %q, want 16 hex characters", run.RunID)
	}

	if start["command"] != "run" || finish["command"] != "run" {
		t.Fatalf("command name lost: %v / %v", start["command"], finish["command"])
	}

	if run.Command != "run" {
		t.Fatalf("CommandRun.Command = %q, want %q", run.Command, "run")
	}

	// The argument vector is what makes a run log reproducible.
	args, ok := start["args"].([]any)
	if !ok || len(args) != 2 || args[0] != "--standard" || args[1] != "rls19-road" {
		t.Fatalf("args not logged verbatim: %#v", start["args"])
	}

	if finish["status"] != "ok" {
		t.Fatalf("status = %v, want ok", finish["status"])
	}

	if finish["level"] != "INFO" {
		t.Fatalf("a successful command finished at level %v, want INFO", finish["level"])
	}

	if _, present := finish["error"]; present {
		t.Fatalf("a successful command logged an error field: %#v", finish)
	}
}

// A failing command must be visible to a log consumer filtering on level, and
// must carry the reason rather than just "error".
func TestEndReportsFailureAtErrorLevelWithTheCause(t *testing.T) {
	out := captureStdout(t, func() {
		run := logging.Begin(logging.New(slog.LevelInfo, true), "export", nil)
		run.End(os.ErrPermission)
	})

	records := decodeJSONLines(t, out)
	if len(records) != 2 {
		t.Fatalf("expected two records, got %d from %q", len(records), out)
	}

	finish := records[1]

	if finish["level"] != "ERROR" {
		t.Fatalf("level = %v, want ERROR", finish["level"])
	}

	if finish["status"] != "error" {
		t.Fatalf("status = %v, want error", finish["status"])
	}

	message, ok := finish["error"].(string)
	if !ok || !strings.Contains(message, os.ErrPermission.Error()) {
		t.Fatalf("error field %#v does not carry the cause", finish["error"])
	}
}

// StartedAt must be UTC: run logs from two machines in different zones are read
// side by side, and a local-time stamp would make the ordering a guess.
func TestBeginStampsStartedAtInUTC(t *testing.T) {
	var run logging.CommandRun

	before := time.Now().UTC()

	out := captureStdout(t, func() {
		run = logging.Begin(logging.New(slog.LevelInfo, true), "status", nil)
	})

	after := time.Now().UTC()

	if run.StartedAt.Location() != time.UTC {
		t.Fatalf("StartedAt location = %v, want UTC", run.StartedAt.Location())
	}

	if run.StartedAt.Before(before) || run.StartedAt.After(after) {
		t.Fatalf("StartedAt %v is outside [%v, %v]", run.StartedAt, before, after)
	}

	records := decodeJSONLines(t, out)
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}

	stamp, ok := records[0]["started_at"].(string)
	if !ok {
		t.Fatalf("started_at missing or not a string: %#v", records[0]["started_at"])
	}

	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatalf("started_at %q is not RFC 3339: %v", stamp, err)
	}

	if !parsed.Equal(run.StartedAt) {
		t.Fatalf("logged started_at %v differs from CommandRun.StartedAt %v", parsed, run.StartedAt)
	}
}

// The duration is measured, not invented: a command that took real time must
// not report zero.
func TestEndReportsAMeasuredDuration(t *testing.T) {
	const slept = 5 * time.Millisecond

	out := captureStdout(t, func() {
		run := logging.Begin(logging.New(slog.LevelInfo, true), "bench", nil)

		time.Sleep(slept)

		run.End(nil)
	})

	records := decodeJSONLines(t, out)
	if len(records) != 2 {
		t.Fatalf("expected two records, got %d", len(records))
	}

	duration, ok := records[1]["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms missing or not numeric: %#v", records[1]["duration_ms"])
	}

	if duration < float64(slept.Milliseconds()) {
		t.Fatalf("duration_ms = %v, want at least %d", duration, slept.Milliseconds())
	}
}

// Two commands in one process must not share a run ID, or their log lines
// cannot be told apart.
func TestBeginMintsADistinctRunIDPerCommand(t *testing.T) {
	const runs = 64

	seen := make(map[string]struct{}, runs)

	captureStdout(t, func() {
		logger := logging.New(slog.LevelInfo, true)
		for range runs {
			seen[logging.Begin(logger, "status", nil).RunID] = struct{}{}
		}
	})

	if len(seen) != runs {
		t.Fatalf("got %d distinct run IDs from %d commands", len(seen), runs)
	}

	for id := range seen {
		if !runIDPattern.MatchString(id) {
			t.Fatalf("run ID %q is not 16 hex characters", id)
		}
	}
}
