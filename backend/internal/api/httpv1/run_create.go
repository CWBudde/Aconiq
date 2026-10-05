package httpv1

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
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

	before, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	err = h.runExecutor(r.Context(), req)
	if err != nil {
		h.closeRunInterruptedBy(before)
		writeRunCreateError(w, err)

		return
	}

	after, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if len(after.Runs) <= len(before.Runs) {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "run_failed",
			Message: "run execution completed without creating a run record",
		})

		return
	}

	writeJSON(w, http.StatusCreated, summarizeRun(after, after.Runs[len(after.Runs)-1]))
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
		Code:    "run_failed",
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
