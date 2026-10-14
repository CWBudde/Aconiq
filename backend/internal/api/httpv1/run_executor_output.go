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

// logErrorPattern finds the error attribute of a slog text record — the form
// the run command's "command finished" line carries its error in:
// error="cli.loadValidatedModel: model file not found: …".
var logErrorPattern = regexp.MustCompile(`\berror=("(?:[^"\\]|\\.)*"|\S+)`)

// childErrorMessage is what the run command said was wrong, in its own words,
// read from its stderr.
//
// The command logs to stderr, through slog — text records by default, JSON
// records under --json — and ends a failed command with a record carrying the
// error. The message is the error attribute of the last record that has one.
// Before the command has a logger (a flag cobra cannot parse) it prints the
// bare error instead, and with no record to read, stderr is the message.
func childErrorMessage(stderr []byte) string {
	var last string

	scanner := bufio.NewScanner(bytes.NewReader(stderr))
	scanner.Buffer(make([]byte, 0, 4096), maxChildOutputBytes)

	for scanner.Scan() {
		if message, ok := logRecordError(scanner.Bytes()); ok {
			last = message
		}
	}

	if last != "" {
		return last
	}

	return strings.TrimSpace(string(stderr))
}

// logRecordError returns the error attribute of one slog record, JSON or text.
func logRecordError(line []byte) (string, bool) {
	trimmed := bytes.TrimSpace(line)

	if bytes.HasPrefix(trimmed, []byte("{")) {
		var record struct {
			Error string `json:"error"`
		}

		err := json.Unmarshal(trimmed, &record)
		if err == nil {
			return record.Error, record.Error != ""
		}
	}

	match := logErrorPattern.FindSubmatch(trimmed)
	if match == nil {
		return "", false
	}

	value := string(match[1])
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	}

	return value, true
}
