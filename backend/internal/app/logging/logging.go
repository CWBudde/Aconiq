package logging

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"time"
)

// CommandRun stores metadata used for structured start/finish logs.
type CommandRun struct {
	RunID     string
	Command   string
	StartedAt time.Time
	logger    *slog.Logger
}

// New creates a structured logger that can emit text or JSON output.
func New(level slog.Level, jsonLogs bool) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{Level: level}
	var handler slog.Handler

	if jsonLogs {
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	} else {
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
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
