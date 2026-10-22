package httpv1

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

type createRunRequest struct {
	ScenarioID      string            `json:"scenario_id,omitempty"`
	StandardID      string            `json:"standard_id,omitempty"`
	StandardVersion string            `json:"standard_version,omitempty"`
	StandardProfile string            `json:"standard_profile,omitempty"`
	ModelPath       string            `json:"model_path,omitempty"`
	ReceiverMode    string            `json:"receiver_mode,omitempty"`
	Params          map[string]string `json:"params,omitempty"`
	InputPaths      []string          `json:"input_paths,omitempty"`
	Experimental    bool              `json:"experimental,omitempty"`

	// The facade receiver set of receiver_mode "facade": the model buildings
	// that carry points, the station spacing along each facade, and whether
	// courtyard facades are included.
	FacadeBuildings        []string `json:"facade_buildings,omitempty"`
	FacadeSpacingM         *float64 `json:"facade_spacing_m,omitempty"`
	FacadeIncludeCourtyard bool     `json:"facade_include_courtyard,omitempty"`

	// AllowUnconfirmed lets a normative run compute at facade points that are
	// still proposed or whose slab height is assumed. The run command refuses
	// them without it; there is no fast path here, because counting them
	// needs the model.
	AllowUnconfirmed bool `json:"allow_unconfirmed,omitempty"`
}

func (h Handler) handleRunCreate(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest

	if !decodeJSONBody(w, r, maxRunCreateBodyBytes, &req) {
		return
	}

	err := req.validate()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
		})

		return
	}

	if gateErr, blocked := h.experimentalOptInGate(req); blocked {
		writeAPIError(w, http.StatusBadRequest, gateErr)
		return
	}

	// A project that is not there is answered here, in its own envelope — 404
	// not_found with the init hint, as every other route answers it — rather
	// than by starting a child whose exit code 2 would come back as a 400
	// with the child's stderr for a message.
	_, err = h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// The server names the run, so that from here on it can find it, cancel it
	// and close it by id rather than by working out which row is whose.
	runID := projectfs.NewRunID()

	run, started := h.runs.start(r.Context(), runID, func(ctx context.Context) error {
		return h.runExecutor(ctx, runID, req)
	}, h.settleRun)
	if !started {
		writeAPIError(w, http.StatusServiceUnavailable, apiError{
			Code:    errorCodeServerStopping,
			Message: "the server is shutting down and starts no new runs",
		})

		return
	}

	h.answerStartedRun(w, r, runID, run)
}

// answerStartedRun waits until the run's row is in the manifest and answers
// 202 with it — or, if the child stops before it ever creates one, answers
// with the reason it gave.
//
// # Why wait for the row at all
//
// A 202 names a run, and a client's next request is about that run: a GET
// straight after must find it. The row is written by the child, in
// projectfs.CreateRun, and the child's refusals all come before it — an
// unknown standard, a bad parameter, a missing scenario — so waiting for the
// row is also what keeps those refusals synchronous, as the 400s they were.
//
// # Why this does not write the row itself
//
// It would be the simpler way to have a row before answering, but the child
// would then have to adopt a row it did not create, with the provenance and
// run directory CreateRun writes beside it, and every refusal it makes up
// front would leave a row behind for the server to clean up. So the server
// only reads here, and the child stays the one writer of its own row; the
// manifest lock (projectfs.Store.LockManifest) keeps that write from racing
// any other.
//
// The run belongs to the server and not to this request: a client that goes
// away while this waits leaves the run computing, which is the point.
func (h Handler) answerStartedRun(w http.ResponseWriter, r *http.Request, runID string, run *supervisedRun) {
	timeout := time.NewTimer(h.runStartTimeout)
	defer timeout.Stop()

	poll := time.NewTicker(runStartPollInterval)
	defer poll.Stop()

	for {
		// A manifest that cannot be read is "not yet" here, and only here: the
		// child is writing it, this is polling, and there is a deadline below.
		proj, row, found, _ := h.findRun(runID)
		if found {
			w.Header().Set("Location", "/api/v1/runs/"+runID)
			writeJSON(w, http.StatusAccepted, h.summarizeRunWithProgress(proj, row))

			return
		}

		select {
		case <-run.done:
			// One last look: a run quick enough to create its row and finish
			// between two polls is still a run, and is answered as one.
			proj, row, found, loadErr := h.findRun(runID)
			if found {
				w.Header().Set("Location", "/api/v1/runs/"+runID)
				writeJSON(w, http.StatusAccepted, h.summarizeRunWithProgress(proj, row))

				return
			}

			if run.err != nil {
				writeRunCreateError(w, run.err)

				return
			}

			if loadErr != nil {
				writeDomainError(w, loadErr)

				return
			}

			writeAPIError(w, http.StatusInternalServerError, apiError{
				Code:    errorCodeRunFailed,
				Message: "run execution completed without creating a run record",
			})

			return
		case <-timeout.C:
			h.runs.cancel(runID, errRunNotStarted)
			writeAPIError(w, http.StatusInternalServerError, apiError{
				Code:    errorCodeRunFailed,
				Message: fmt.Sprintf("run %q did not start within %s and was stopped", runID, h.runStartTimeout),
			})

			return
		case <-r.Context().Done():
			return
		case <-poll.C:
		}
	}
}

// findRun loads the manifest and looks the run up in it. "Not in the
// manifest" (false, nil) and "the manifest could not be read" (false, err)
// are different answers: the first is a 404, the second a storage failure,
// and conflating them told a client its run did not exist while it went on
// computing.
func (h Handler) findRun(runID string) (project.Project, project.Run, bool, error) {
	proj, err := h.store.Load()
	if err != nil {
		return project.Project{}, project.Run{}, false, fmt.Errorf("load project manifest: %w", err)
	}

	for _, run := range proj.Runs {
		if run.ID == runID {
			return proj, run, true, nil
		}
	}

	return project.Project{}, project.Run{}, false, nil
}

// experimentalOptInGate refuses a run against a scaffold-tier standard that the
// caller has not acknowledged, before the executor is reached. Rejecting here
// rather than reading the refusal back out of the run command's exit code keeps
// the answer a first-class envelope — code, details and hint — and costs no
// process: the caller learns which standard and which tier blocked the request
// instead of receiving a subprocess's stderr as prose.
//
// The gate applies the same framework predicate the run command applies, so the
// two cannot disagree about what a tier requires. It is a fast path, not the
// authority: a handler built without a registry cannot resolve a tier at all,
// and a request naming an unknown standard is left for the run command to
// reject in its own words. Both fall through to the run command, which gates
// again before it persists anything.
func (h Handler) experimentalOptInGate(req createRunRequest) (apiError, bool) {
	if req.Experimental || h.registry == nil || req.StandardID == "" {
		return apiError{}, false
	}

	resolved, err := h.registry.Resolve(req.StandardID, req.StandardVersion, req.StandardProfile)
	if err != nil {
		return apiError{}, false
	}

	if !resolved.EvidenceTier.RequiresExperimentalOptIn() {
		return apiError{}, false
	}

	return apiError{
		Code: errorCodeExperimentalOptInRequired,
		Message: fmt.Sprintf(
			"standard %q is evidence tier %q: it carries no normative coefficients, its base levels are invented and it has no octave bands, "+
				"so the levels it would emit are not an implementation of the standard it names and must not be used for assessment",
			resolved.StandardID, resolved.EvidenceTier,
		),
		Details: map[string]any{
			"standard_id":     resolved.StandardID,
			evidenceTierField: string(resolved.EvidenceTier),
		},
		Hint: `Set "experimental": true in the request body to acknowledge that and run it anyway.`,
	}, true
}

func writeRunCreateError(w http.ResponseWriter, err error) {
	var appErr *domainerrors.AppError
	if stderrors.As(err, &appErr) {
		writeDomainError(w, err)
		return
	}

	writeAPIError(w, http.StatusInternalServerError, apiError{
		Code:    errorCodeRunFailed,
		Message: err.Error(),
	})
}

// The run executor turns a request into argv for the aconiq binary. The API is
// unauthenticated and reachable cross-origin, so every field that becomes an
// argument is constrained here rather than trusted: an entry starting with "-"
// would be read as a flag, and an absolute or "../"-escaping path would let a
// caller point a run at any file the server process can read.
var (
	// Standard, scenario and profile identifiers as the registry declares them
	// ("rls19-road", "iso9613", "auto-grid").
	runIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// Parameter names as the standards modules declare them ("road_speed_kph").
	runParamKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func (req createRunRequest) validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{"scenario_id", req.ScenarioID},
		{"standard_id", req.StandardID},
		{"standard_version", req.StandardVersion},
		{"standard_profile", req.StandardProfile},
		{"receiver_mode", req.ReceiverMode},
	}

	for _, field := range fields {
		if field.value == "" {
			continue
		}

		if !runIdentifierPattern.MatchString(field.value) {
			return fmt.Errorf("%s must match %s", field.name, runIdentifierPattern)
		}
	}

	if req.ModelPath != "" {
		err := validateRunInputPath("model_path", req.ModelPath)
		if err != nil {
			return err
		}
	}

	for _, inputPath := range req.InputPaths {
		err := validateRunInputPath("input_paths", inputPath)
		if err != nil {
			return err
		}
	}

	err := req.validateFacadeSet()
	if err != nil {
		return err
	}

	// Sorted so that a request with several invalid keys always reports the same
	// one.
	paramKeys := make([]string, 0, len(req.Params))
	for key := range req.Params {
		paramKeys = append(paramKeys, key)
	}

	slices.Sort(paramKeys)

	for _, key := range paramKeys {
		value := req.Params[key]
		if !runParamKeyPattern.MatchString(key) {
			return fmt.Errorf("params key %q must match %s", key, runParamKeyPattern)
		}

		if strings.ContainsAny(value, "\x00\n\r") {
			return fmt.Errorf("params value for %q must not contain control characters", key)
		}
	}

	return nil
}

// validateFacadeSet checks the facade fields that become arguments. A building
// id is a model feature id, so its spelling is the model's and is not
// constrained beyond what argv needs: it is passed as `--facade-building=<id>`,
// so a leading "-" is a value, and a control character is refused. Whether the
// set is complete, and whether the ids name buildings, is the run command's to
// decide.
func (req createRunRequest) validateFacadeSet() error {
	for _, id := range req.FacadeBuildings {
		if id == "" {
			return stderrors.New("facade_buildings entries must not be empty")
		}

		if strings.ContainsAny(id, "\x00\n\r") {
			return fmt.Errorf("facade_buildings entry %q must not contain control characters", id)
		}
	}

	if req.FacadeSpacingM != nil && !(*req.FacadeSpacingM > 0) {
		return stderrors.New("facade_spacing_m must be > 0")
	}

	return nil
}

// validateRunInputPath keeps a path inside the project root. filepath.IsLocal
// rejects absolute paths, ".." escapes and the empty string; the leading-dash
// check stops a path from being parsed as a flag instead.
func validateRunInputPath(field, path string) error {
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("%s must not start with %q", field, "-")
	}

	if !filepath.IsLocal(path) {
		return fmt.Errorf("%s must be a relative path inside the project", field)
	}

	return nil
}
