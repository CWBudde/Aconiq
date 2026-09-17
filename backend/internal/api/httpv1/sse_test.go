package httpv1

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func waitForSSEEventData(body io.ReadCloser, timeout time.Duration, done func(seen map[string]string) bool) (map[string]string, error) {
	resultCh := make(chan map[string]string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		// bufio.Reader rather than bufio.Scanner, and the difference is the
		// whole point. A Scanner hands back the final partial line as an
		// ordinary token at EOF, so a stream that ends mid-frame yields a
		// truncated `data:` payload that looks complete to the loop below —
		// which then reports it and leaves the caller to fail on
		// "unexpected end of JSON input". ReadString returns the unterminated
		// tail together with io.EOF, so it can be told apart and dropped.
		reader := bufio.NewReader(body)
		currentEvent := ""
		seen := make(map[string]string)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				// Whatever `line` holds here was never terminated, so the
				// server had not finished writing it. It is not an event.
				if errors.Is(err, io.EOF) {
					errCh <- errors.New("sse stream ended before expected events")
				} else {
					errCh <- err
				}

				return
			}

			line = strings.TrimRight(line, "\r\n")

			if after, ok := strings.CutPrefix(line, "event: "); ok {
				currentEvent = strings.TrimSpace(after)
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				if currentEvent == "" {
					continue
				}

				seen[currentEvent] = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				if done(seen) {
					resultCh <- seen
					return
				}
			}
		}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-timer.C:
		_ = body.Close()
		return nil, errors.New("timed out waiting for sse events")
	}
}

// A stream that stops mid-frame must be reported as a truncated stream, not as
// an event.
//
// This is a regression test for a flake, not for a feature. The helper used to
// scan with bufio.Scanner, which yields the final unterminated line as an
// ordinary token at EOF; a `data:` frame cut in half therefore reached the
// caller looking complete, and the caller failed on "unexpected end of JSON
// input" somewhere far from the cause. It needed an unlucky connection close to
// show up, so it surfaced once on a loaded `-race` runner and passed 30 local
// reruns.
func TestWaitForSSEEventDataRejectsATruncatedFrame(t *testing.T) {
	t.Parallel()

	// One frame, cut off mid-payload: no closing brace, no newline. The `done`
	// predicate below is the one every caller in this file uses — "I have seen
	// a project_status" — which is exactly what makes the old helper hand the
	// fragment over as though it were the event.
	stream := "event: project_status\n" + `data: {"project_avail`

	body := io.NopCloser(strings.NewReader(stream))

	seen, err := waitForSSEEventData(body, time.Second, func(seen map[string]string) bool {
		return seen["project_status"] != ""
	})
	if err == nil {
		t.Fatalf("expected a truncated stream to be refused, got %#v", seen)
	}

	if !strings.Contains(err.Error(), "ended before expected events") {
		t.Errorf("error should name the truncated stream, got %v", err)
	}
}

// A complete frame is still read, so the guard above did not buy its
// robustness by refusing everything.
func TestWaitForSSEEventDataReadsACompleteFrame(t *testing.T) {
	t.Parallel()

	stream := "event: project_status\n" +
		`data: {"project_available":false}` + "\n\n"

	body := io.NopCloser(strings.NewReader(stream))

	seen, err := waitForSSEEventData(body, time.Second, func(seen map[string]string) bool {
		return seen["project_status"] != ""
	})
	if err != nil {
		t.Fatalf("read a complete frame: %v", err)
	}

	var payload map[string]any

	decodeResponse(t, []byte(seen["project_status"]), &payload)

	if available, ok := payload["project_available"].(bool); !ok || available {
		t.Errorf("project_available = %#v, want false", payload["project_available"])
	}
}
