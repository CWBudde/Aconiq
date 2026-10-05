package httpv1

import (
	"net/http"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/standards/descriptorjson"
)

// Standards API response types.
//
// The descriptor shape itself lives in internal/standards/descriptorjson,
// because the WASM kernel publishes the same descriptors through
// `aconiq.standards()` and a second encoding would be a second answer to the
// question of what parameters a standard takes. The alias keeps the name the
// handler tests read: those tests assert the response bytes, and renaming the
// type here would read like a contract change there.
type standardResponse = descriptorjson.Standard

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
	// Model is absent until a model has been saved.
	Model *projectModelStatus `json:"model,omitempty"`
}

// projectModelStatus lets a client decide whether its local draft still matches
// the project without fetching the model. The hash is the same receipt
// POST /api/v1/model returned; the client compares strings and never hashes.
type projectModelStatus struct {
	Hash      string    `json:"hash"`
	UpdatedAt time.Time `json:"updated_at"`
}

type lastRunStatus struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Context    string    `json:"context,omitempty"`
	StandardID string    `json:"standard_id"`
	Version    string    `json:"version"`
	Profile    string    `json:"profile,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
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

	writeJSON(w, http.StatusOK, h.projectStatus(proj))
}

// projectStatus renders a loaded project as the status payload. It is the one
// builder behind both "project status" surfaces — the REST response and the
// SSE `project_status` event — so the two cannot answer the same question
// differently; a test marshals both and compares the bytes.
func (h Handler) projectStatus(proj project.Project) projectStatusResponse {
	status := projectStatusResponse{
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
		status.LastRun = &lastRunStatus{
			ID:         last.ID,
			Status:     last.Status,
			Context:    last.Standard.Context,
			StandardID: last.Standard.ID,
			Version:    last.Standard.Version,
			Profile:    last.Standard.Profile,
			StartedAt:  last.StartedAt,
			FinishedAt: last.FinishedAt,
		}
	}

	status.Model = h.modelStatus(proj)

	return status
}

// modelStatus reports the saved model's receipt, or nil when there is none.
//
// The manifest is consulted first, so a fresh project opens no file at all:
// without a model artifact ref there is nothing to hash. A hash that then fails
// is reported as absence rather than failing the whole status request — a
// client that cannot compare falls back to fetching the model, which is a
// worse answer than this one but not a broken one.
func (h Handler) modelStatus(proj project.Project) *projectModelStatus {
	var ref project.ArtifactRef

	for _, a := range proj.Artifacts {
		if a.ID == project.ArtifactIDModelNormalized {
			ref = a
			break
		}
	}

	if ref.ID == "" {
		return nil
	}

	hash, err := h.store.ModelHash()
	if err != nil {
		return nil
	}

	return &projectModelStatus{
		Hash:      hash,
		UpdatedAt: ref.CreatedAt,
	}
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

	writeJSON(w, http.StatusOK, descriptorjson.FromDescriptors(h.registry.List()))
}
