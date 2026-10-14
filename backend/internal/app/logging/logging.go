package logging

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"time"
)

// CommandRun stores metadata used for structured start/finish logs.
type CommandRun struct {
	RunID     string
	Command   string
	StartedAt time.Time
	logger    *slog.Logger
}

// New creates a structured logger that writes text or JSON records to w.
//
// The CLI passes its stderr. A command's stdout is its result — under --json
// one JSON document a caller decodes as a whole — and a log record there would
// make it a stream the caller has to pick the result out of. The `aconiq serve`
// handlers that spawn `aconiq run` read stdout for the result and stderr for
// the error the last record carries, and rely on the two not mixing.
func New(w io.Writer, level slog.Level, jsonLogs bool) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{Level: level}

	var handler slog.Handler

	if jsonLogs {
		handler = slog.NewJSONHandler(w, handlerOptions)
	} else {
		handler = slog.NewTextHandler(w, handlerOptions)
	}

	return slog.New(handler)
}

// Begin starts a timed command run span and logs metadata with a new run ID.
func Begin(logger *slog.Logger, command string, args []string) CommandRun {
	runID := newRunID()
	commandLogger := logger.With(
		slog.String("run_id", runID),
		slog.String("command", command),
	)

	startedAt := time.Now().UTC()
	commandLogger.Info("command started", slog.Time("started_at", startedAt), slog.Any("args", args))

	return CommandRun{
		RunID:     runID,
		Command:   command,
		StartedAt: startedAt,
		logger:    commandLogger,
	}
}

// End finishes a timed command run span and logs outcome and duration.
func (r CommandRun) End(err error) {
	durationMs := time.Since(r.StartedAt).Milliseconds()
	if err != nil {
		r.logger.Error("command finished", slog.String("status", "error"), slog.Int64("duration_ms", durationMs), slog.Any("error", err))
		return
	}

	r.logger.Info("command finished", slog.String("status", "ok"), slog.Int64("duration_ms", durationMs))
}

func newRunID() string {
	buf := make([]byte, 8)
	{
		_, err := io.ReadFull(rand.Reader, buf)
		if err != nil {
			return "runid-unavailable"
		}
	}

	return hex.EncodeToString(buf)
}
