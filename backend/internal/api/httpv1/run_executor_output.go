package httpv1

import (
	"bufio"
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// maxChildOutputBytes bounds how much of a child's output is kept to find its
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

// logErrorPattern finds the error attribute of a slog text record — the form
// the run command's "command finished" line carries its error in:
// error="cli.loadValidatedModel: model file not found: …".
var logErrorPattern = regexp.MustCompile(`\berror=("(?:[^"\\]|\\.)*"|\S+)`)

// childErrorMessage is what the run command said was wrong, in its own words.
//
// The command logs through slog to stdout and prints to stderr only when it
// has no logger yet, so stderr alone was empty for every refusal the run
// pipeline makes, and the API answered "request failed". stderr wins when it
// has something; otherwise it is the error attribute of the last log record
// that carries one.
func childErrorMessage(stdout, stderr []byte) string {
	if message := strings.TrimSpace(string(stderr)); message != "" {
		return message
	}

	var last string

	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 4096), maxChildOutputBytes)

	for scanner.Scan() {
		match := logErrorPattern.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}

		value := match[1]
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}

		last = value
	}

	return last
}
