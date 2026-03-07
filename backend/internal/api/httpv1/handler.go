package httpv1

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	apiVersion = "v1"
)

type Handler struct {
	store       projectfs.Store
	now         func() time.Time
	sseInterval time.Duration
	registry    *framework.Registry
}

// Standards API response types.

type parameterDefinitionResponse struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value,omitempty"`
	Description  string   `json:"description,omitempty"`
	Enum         []string `json:"enum,omitempty"`
	Min          *float64 `json:"min,omitempty"`
	Max          *float64 `json:"max,omitempty"`
}

type profileResponse struct {
	Name                 string                        `json:"name"`
	SupportedSourceTypes []string                      `json:"supported_source_types"`
	SupportedIndicators  []string                      `json:"supported_indicators"`
	Parameters           []parameterDefinitionResponse `json:"parameters"`
}

type versionResponse struct {
	Name           string            `json:"name"`
	DefaultProfile string            `json:"default_profile"`
	Profiles       []profileResponse `json:"profiles"`
}

type artifactRefResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type runSummaryResponse struct {
	ID         string                `json:"id"`
	ScenarioID string                `json:"scenario_id"`
	StandardID string                `json:"standard_id"`
	Version    string                `json:"version"`
	Profile    string                `json:"profile,omitempty"`
	Status     string                `json:"status"`
	StartedAt  time.Time             `json:"started_at"`
	FinishedAt time.Time             `json:"finished_at"`
	LogPath    string                `json:"log_path"`
	Artifacts  []artifactRefResponse `json:"artifacts"`
}

type runLogResponse struct {
	RunID string   `json:"run_id"`
	Lines []string `json:"lines"`
}

type standardResponse struct {
	ID             string            `json:"id"`
	Description    string            `json:"description"`
	DefaultVersion string            `json:"default_version"`
	Versions       []versionResponse `json:"versions"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Hint    string         `json:"hint,omitempty"`
}

type healthResponse struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
}

type projectStatusResponse struct {
	ProjectID       string         `json:"project_id"`
	Name            string         `json:"name"`
	ProjectPath     string         `json:"project_path"`
	ManifestVersion int            `json:"manifest_version"`
	CRS             string         `json:"crs"`
	ScenarioCount   int            `json:"scenario_count"`
	RunCount        int            `json:"run_count"`
	LastRun         *lastRunStatus `json:"last_run,omitempty"`
}

type lastRunStatus struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	StandardID string    `json:"standard_id"`
	Version    string    `json:"version"`
	Profile    string    `json:"profile,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

func NewHandler(store projectfs.Store, clock func() time.Time) http.Handler {
	return newHandlerWithOptions(store, handlerOptions{
		clock:       clock,
		sseInterval: 2 * time.Second,
	})
}

// NewHandlerWithRegistry returns a handler that also exposes /api/v1/standards.
func NewHandlerWithRegistry(store projectfs.Store, clock func() time.Time, registry framework.Registry) http.Handler {
	return newHandlerWithOptions(store, handlerOptions{
		clock:       clock,
		sseInterval: 2 * time.Second,
		registry:    &registry,
	})
}

type handlerOptions struct {
	clock       func() time.Time
	sseInterval time.Duration
	registry    *framework.Registry
}

func newHandlerWithOptions(store projectfs.Store, opts handlerOptions) http.Handler {
	now := opts.clock
	if opts.clock == nil {
		now = time.Now
	} else {
		now = opts.clock
	}

	sseInterval := opts.sseInterval
	if sseInterval <= 0 {
		sseInterval = 2 * time.Second
	}

	handler := Handler{
		store:       store,
		now:         now,
		sseInterval: sseInterval,
		registry:    opts.registry,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", handler.handleHealth)
	mux.HandleFunc("/api/v1/project/status", handler.handleProjectStatus)
	mux.HandleFunc("/api/v1/standards", handler.handleStandards)
	mux.HandleFunc("/api/v1/runs", handler.handleRunsList)
	mux.HandleFunc("/api/v1/runs/{id}/log", handler.handleRunLog)
	mux.HandleFunc("/api/v1/events", handler.handleEvents)
	mux.HandleFunc("/api/v1/openapi.json", handler.handleOpenAPI)
	mux.HandleFunc("/", handler.handleNotFound)

	return mux
}

func (h Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: apiVersion,
		Time:    h.now().UTC(),
	})
}

func (h Handler) handleProjectStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	response := projectStatusResponse{
		ProjectID:       proj.ProjectID,
		Name:            proj.Name,
		ProjectPath:     h.store.Root(),
		ManifestVersion: proj.ManifestVersion,
		CRS:             proj.CRS,
		ScenarioCount:   len(proj.Scenarios),
		RunCount:        len(proj.Runs),
	}

	if len(proj.Runs) > 0 {
		last := proj.Runs[len(proj.Runs)-1]
		response.LastRun = &lastRunStatus{
			ID:         last.ID,
			Status:     last.Status,
			StandardID: last.Standard.ID,
			Version:    last.Standard.Version,
			Profile:    last.Standard.Profile,
			StartedAt:  last.StartedAt,
			FinishedAt: last.FinishedAt,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h Handler) handleRunsList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Index artifacts by run ID.
	artifactsByRun := make(map[string][]artifactRefResponse, len(proj.Artifacts))
	for _, a := range proj.Artifacts {
		if a.RunID == "" {
			continue
		}

		artifactsByRun[a.RunID] = append(artifactsByRun[a.RunID], artifactRefResponse{
			ID:        a.ID,
			Kind:      a.Kind,
			Path:      a.Path,
			CreatedAt: a.CreatedAt,
		})
	}

	summaries := make([]runSummaryResponse, 0, len(proj.Runs))
	for i := len(proj.Runs) - 1; i >= 0; i-- {
		run := proj.Runs[i]

		artifacts := artifactsByRun[run.ID]
		if artifacts == nil {
			artifacts = []artifactRefResponse{}
		}

		summaries = append(summaries, runSummaryResponse{
			ID:         run.ID,
			ScenarioID: run.ScenarioID,
			StandardID: run.Standard.ID,
			Version:    run.Standard.Version,
			Profile:    run.Standard.Profile,
			Status:     run.Status,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			LogPath:    run.LogPath,
			Artifacts:  artifacts,
		})
	}

	writeJSON(w, http.StatusOK, summaries)
}

func (h Handler) handleRunLog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	runID := r.PathValue("id")
	if runID == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    "bad_request",
			Message: "run id is required",
		})

		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var logPath string

	for _, run := range proj.Runs {
		if run.ID == runID {
			logPath = run.LogPath
			break
		}
	}

	if logPath == "" {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    "not_found",
			Message: fmt.Sprintf("run %q not found", runID),
		})

		return
	}

	absLogPath := h.store.Root() + "/" + strings.ReplaceAll(logPath, "\\", "/")

	raw, readErr := os.ReadFile(absLogPath)
	if readErr != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "internal_error",
			Message: "failed to read run log",
		})

		return
	}

	lines := splitLogLines(string(raw))
	writeJSON(w, http.StatusOK, runLogResponse{
		RunID: runID,
		Lines: lines,
	})
}

func (h Handler) handleStandards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if h.registry == nil {
		writeAPIError(w, http.StatusServiceUnavailable, apiError{
			Code:    "standards_unavailable",
			Message: "standards registry not configured",
		})

		return
	}

	descriptors := h.registry.List()

	standards := make([]standardResponse, 0, len(descriptors))
	for _, d := range descriptors {
		versions := make([]versionResponse, 0, len(d.Versions))
		for _, v := range d.Versions {
			profiles := make([]profileResponse, 0, len(v.Profiles))
			for _, p := range v.Profiles {
				params := make([]parameterDefinitionResponse, 0, len(p.ParameterSchema.Parameters))
				for _, param := range p.ParameterSchema.Parameters {
					params = append(params, parameterDefinitionResponse{
						Name:         param.Name,
						Kind:         string(param.Kind),
						Required:     param.Required,
						DefaultValue: param.DefaultValue,
						Description:  param.Description,
						Enum:         param.Enum,
						Min:          param.Min,
						Max:          param.Max,
					})
				}

				profiles = append(profiles, profileResponse{
					Name:                 p.Name,
					SupportedSourceTypes: p.SupportedSourceTypes,
					SupportedIndicators:  p.SupportedIndicators,
					Parameters:           params,
				})
			}

			versions = append(versions, versionResponse{
				Name:           v.Name,
				DefaultProfile: v.DefaultProfile,
				Profiles:       profiles,
			})
		}

		standards = append(standards, standardResponse{
			ID:             d.ID,
			Description:    d.Description,
			DefaultVersion: d.DefaultVersion,
			Versions:       versions,
		})
	}

	writeJSON(w, http.StatusOK, standards)
}

func (h Handler) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    "not_found",
		Message: "endpoint not found",
		Details: map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
		},
		Hint: "Use /api/v1/health, /api/v1/project/status, /api/v1/runs, /api/v1/runs/{id}/log, /api/v1/standards, /api/v1/events, or /api/v1/openapi.json.",
	})
}

func (h Handler) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, BuildOpenAPISpec(""))
}

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

	if _, err := io.WriteString(w, "retry: 3000\n\n"); err != nil {
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

	err := pushStatusEvent()
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
			Code:    "internal_error",
			Message: "failed to load project status",
		}

		var appErr *domainerrors.AppError
		if stderrors.As(err, &appErr) {
			if appErr.Msg != "" {
				apiErr.Message = appErr.Msg
			}

			switch appErr.Kind {
			case domainerrors.KindNotFound:
				apiErr.Code = "not_found"
			case domainerrors.KindValidation:
				apiErr.Code = "validation_error"
			case domainerrors.KindUserInput:
				apiErr.Code = "user_input_error"
			default:
				apiErr.Code = "internal_error"
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

	status := projectStatusResponse{
		ProjectID:       proj.ProjectID,
		Name:            proj.Name,
		ProjectPath:     h.store.Root(),
		ManifestVersion: proj.ManifestVersion,
		CRS:             proj.CRS,
		ScenarioCount:   len(proj.Scenarios),
		RunCount:        len(proj.Runs),
	}
	lastRunID := ""
	lastRunState := ""
	lastRunUpdated := ""

	if len(proj.Runs) > 0 {
		last := proj.Runs[len(proj.Runs)-1]
		status.LastRun = &lastRunStatus{
			ID:         last.ID,
			Status:     last.Status,
			StandardID: last.Standard.ID,
			Version:    last.Standard.Version,
			Profile:    last.Standard.Profile,
			StartedAt:  last.StartedAt,
			FinishedAt: last.FinishedAt,
		}
		lastRunID = last.ID
		lastRunState = last.Status
		lastRunUpdated = last.FinishedAt.UTC().Format(time.RFC3339Nano)
	}

	key := strings.Join([]string{
		"available",
		proj.ProjectID,
		strconv.Itoa(len(proj.Runs)),
		lastRunID,
		lastRunState,
		lastRunUpdated,
	}, ":")

	return map[string]any{
		"time":              now,
		"project_available": true,
		"project":           status,
	}, key
}

func splitLogLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func requireMethod(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method == expected {
		return true
	}

	writeAPIError(w, http.StatusMethodNotAllowed, apiError{
		Code:    "method_not_allowed",
		Message: "unsupported HTTP method",
		Details: map[string]any{
			"method":   r.Method,
			"expected": expected,
			"path":     r.URL.Path,
		},
	})

	return false
}

func writeSSEEvent(w io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}

	return nil
}

func writeDomainError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	apiErr := apiError{
		Code:    "internal_error",
		Message: "request failed",
	}

	var appErr *domainerrors.AppError
	if stderrors.As(err, &appErr) {
		if appErr.Msg != "" {
			apiErr.Message = appErr.Msg
		}

		apiErr.Details = map[string]any{
			"operation": appErr.Op,
			"kind":      appErr.Kind,
		}

		switch appErr.Kind {
		case domainerrors.KindUserInput:
			status = http.StatusBadRequest
			apiErr.Code = "user_input_error"
		case domainerrors.KindValidation:
			status = http.StatusBadRequest
			apiErr.Code = "validation_error"
		case domainerrors.KindNotFound:
			status = http.StatusNotFound
			apiErr.Code = "not_found"
			apiErr.Hint = "Initialize the project first with `noise init`."
		default:
			status = http.StatusInternalServerError
			apiErr.Code = "internal_error"
		}
	}

	writeAPIError(w, status, apiErr)
}

func writeAPIError(w http.ResponseWriter, status int, apiErr apiError) {
	writeJSON(w, status, errorResponse{Error: apiErr})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		encoded = []byte(`{"error":{"code":"internal_error","message":"failed to encode response"}}`)
		status = http.StatusInternalServerError
	}

	encoded = append(encoded, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}
