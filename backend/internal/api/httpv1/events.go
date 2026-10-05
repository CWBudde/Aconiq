package httpv1

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

func (h Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "stream_not_supported",
			Message: "streaming is not supported by this server",
		})

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, err := io.WriteString(w, "retry: 3000\n\n")
	if err != nil {
		return
	}

	lastStatusKey := ""
	pushStatusEvent := func() error {
		event, key := h.buildProjectStatusStreamEvent()
		if key == lastStatusKey {
			return nil
		}

		err := writeSSEEvent(w, "project_status", event)
		if err != nil {
			return err
		}

		lastStatusKey = key

		flusher.Flush()

		return nil
	}

	err = pushStatusEvent()
	if err != nil {
		return
	}

	ticker := time.NewTicker(h.sseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			err := writeSSEEvent(w, "heartbeat", map[string]any{
				"time": h.now().UTC(),
			})
			if err != nil {
				return
			}

			err = pushStatusEvent()
			if err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

func (h Handler) buildProjectStatusStreamEvent() (map[string]any, string) {
	now := h.now().UTC()

	proj, err := h.store.Load()
	if err != nil {
		apiErr := apiError{
			Code:    errorCodeInternalError,
			Message: "failed to load project status",
		}

		var appErr *domainerrors.AppError
		if stderrors.As(err, &appErr) {
			if appErr.Msg != "" {
				apiErr.Message = appErr.Msg
			}

			switch appErr.Kind {
			case domainerrors.KindNotFound:
				apiErr.Code = errorCodeNotFound
			case domainerrors.KindValidation:
				apiErr.Code = "validation_error"
			case domainerrors.KindUserInput:
				apiErr.Code = "user_input_error"
			default:
				apiErr.Code = errorCodeInternalError
			}

			apiErr.Details = map[string]any{
				"operation": appErr.Op,
				"kind":      appErr.Kind,
			}
		}

		key := fmt.Sprintf("missing:%s:%s", apiErr.Code, apiErr.Message)

		return map[string]any{
			"time":              now,
			"project_available": false,
			"error":             apiErr,
		}, key
	}

	status := h.projectStatus(proj)
	lastRunID := ""
	lastRunState := ""
	lastRunUpdated := ""

	if len(proj.Runs) > 0 {
		last := proj.Runs[len(proj.Runs)-1]
		lastRunID = last.ID
		lastRunState = last.Status
		lastRunUpdated = last.FinishedAt.UTC().Format(time.RFC3339Nano)
	}

	// Every member the payload can change by must appear in the key, or the
	// stream serves the first snapshot forever. The model hash is the one that
	// moves without any run moving: saving a model changes the payload and
	// nothing else here.
	modelHash := ""
	if status.Model != nil {
		modelHash = status.Model.Hash
	}

	key := strings.Join([]string{
		"available",
		proj.ProjectID,
		strconv.Itoa(len(proj.Runs)),
		lastRunID,
		lastRunState,
		lastRunUpdated,
		modelHash,
	}, ":")

	return map[string]any{
		"time":              now,
		"project_available": true,
		"project":           status,
	}, key
}

func writeSSEEvent(w io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE event %q data: %w", event, err)
	}

	_, err = fmt.Fprintf(w, "event: %s\n", event)
	if err != nil {
		return fmt.Errorf("write SSE event %q header: %w", event, err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	if err != nil {
		return fmt.Errorf("write SSE event %q data: %w", event, err)
	}

	return nil
}
