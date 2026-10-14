package httpv1

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// maxChildOutputBytes bounds how much of a child's stderr is kept to find its
// error in. The error is the last thing the run command logs, so the tail is
// the part that matters, and a run that logs for an hour must not grow the
// server's memory with it.
const maxChildOutputBytes = 64 << 10

// tailBuffer keeps the last maxChildOutputBytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf = append(t.buf, p...)
	if over := len(t.buf) - maxChildOutputBytes; over > 0 {
		t.buf = t.buf[over:]
	}

	return len(p), nil
}

func (t *tailBuffer) Bytes() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()

	return bytes.Clone(t.buf)
}

// headBuffer keeps the first limit bytes written to it and remembers whether
// more came. It is for output read as one document, where a tail would be a
// document cut at the front and a silent cut anywhere a wrong answer: a caller
// that sees overflowed refuses the output instead.
//
// Writes past the limit are accepted and dropped rather than failed, so a
// child writing too much is not killed by a broken pipe before it exits.
type headBuffer struct {
	mu         sync.Mutex
	limit      int
	buf        []byte
	overflowed bool
}

func (h *headBuffer) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room := max(h.limit-len(h.buf), 0)
	if len(p) > room {
		h.overflowed = true
		h.buf = append(h.buf, p[:room]...)

		return len(p), nil
	}

	h.buf = append(h.buf, p...)

	return len(p), nil
}

// Bytes returns what was kept, and whether anything past it was dropped.
func (h *headBuffer) Bytes() ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return bytes.Clone(h.buf), h.overflowed
}

// Patterns over one slog text record. A record begins with its time, or with
// its level when the time is zero; the error attribute is the form the run
// command's failure records carry their error in:
// error="cli.loadValidatedModel: model file not found: …".
var (
	textRecordLevelPattern = regexp.MustCompile(`^(?:time=\S+ )?level=(\S+)`)
	textRecordErrorPattern = regexp.MustCompile(`\berror=("(?:[^"\\]|\\.)*"|\S+)`)
	textRecordUserPattern  = regexp.MustCompile(`\buser_error=true\b`)
)

// childFailure is what a failed run child's stderr says about its failure.
type childFailure struct {
	// message is the reason in the child's own words, or "" when stderr
	// names none and the caller has only the process's exit to go by.
	message string

	// mayBeUserError says whether an exit status of 2 may be read as a user
	// error. It is false when the child logged and never logged a refusal: a
	// Go panic exits 2 as well, and a panic is not the user's doing.
	mayBeUserError bool
}

// logRecord is the part of one slog record the failure is read from.
type logRecord struct {
	level     string
	err       string
	userError bool
}

// readChildFailure reads what the run command said was wrong, in its own
// words, from its stderr.
//
// The command logs to stderr through slog — text records by default, JSON
// records under --json — and ends a failed command with ERROR records that
// carry the error and whether it was the user's. The reason is the error of
// the last such record. Records at other levels are not the failure, even
// with an error attribute: a WARN that terrain would not load is a run that
// went on without it.
//
// A child that logged but never logged a failure was stopped by something
// other than itself. Killed, it says nothing more and the reason is "" — the
// caller falls back to the exit status. Panicking, the runtime adds the panic
// line and a trace, and the reason is the first line that is not a record.
// A child with no record at all failed before it had a logger, and printed
// the bare error; its first line is the reason.
func readChildFailure(stderr []byte) childFailure {
	var (
		failure     childFailure
		sawRecord   bool
		sawUserErr  bool
		firstNonLog string
	)

	scanner := bufio.NewScanner(bytes.NewReader(stderr))
	scanner.Buffer(make([]byte, 0, 4096), maxChildOutputBytes)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		record, isRecord := parseLogRecord(line)
		if !isRecord {
			if firstNonLog == "" {
				firstNonLog = string(line)
			}

			continue
		}

		sawRecord = true

		if strings.HasPrefix(record.level, "ERROR") && record.err != "" {
			failure.message = record.err
			sawUserErr = sawUserErr || record.userError
		}
	}

	switch {
	case failure.message != "":
		failure.mayBeUserError = sawUserErr
	case sawRecord:
		failure.message = firstNonLog
	default:
		failure.message = firstNonLog
		failure.mayBeUserError = true
	}

	return failure
}

// parseLogRecord reads one line as a slog record, JSON or text. A line that is
// neither — a panic, a bare error — is not a record.
func parseLogRecord(line []byte) (logRecord, bool) {
	if bytes.HasPrefix(line, []byte("{")) {
		var fields struct {
			Level     string `json:"level"`
			Error     any    `json:"error"`
			UserError bool   `json:"user_error"`
		}

		err := json.Unmarshal(line, &fields)
		if err == nil {
			message, _ := fields.Error.(string)

			return logRecord{level: fields.Level, err: message, userError: fields.UserError}, true
		}
	}

	level := textRecordLevelPattern.FindSubmatch(line)
	if level == nil {
		return logRecord{}, false
	}

	record := logRecord{
		level:     string(level[1]),
		userError: textRecordUserPattern.Match(line),
	}

	if match := textRecordErrorPattern.FindSubmatch(line); match != nil {
		record.err = string(match[1])
		if unquoted, err := strconv.Unquote(record.err); err == nil {
			record.err = unquoted
		}
	}

	return record, true
}
