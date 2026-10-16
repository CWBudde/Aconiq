package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/spf13/cobra"
)

type runCommandRequest struct {
	scenarioID      string
	standardID      string
	standardVersion string
	standardProfile string
	modelPath       string
	receiverMode    string
	rawParams       []string
	inputPaths      []string
	experimental    bool

	// workers is --workers: how many goroutines the compute walks receivers
	// over, 0 meaning one per available CPU.
	//
	// It is a flag and not a --param on purpose. A declared run parameter
	// lands in provenance.parameters, and recording a scheduling knob as an
	// input to the calculation would say the result depends on it -- when the
	// whole guarantee, and the 1-worker-vs-N hash test behind it, is that it
	// does not. It goes in the run log instead.
	workers int

	// runID is the hidden --run-id: the id to record the run under, chosen by
	// the caller. `aconiq serve` is the caller — it mints the id before it
	// starts this process, so it can answer 202 with the id, find the row, and
	// close that one row should this process die without closing it. Empty
	// mints one here. projectfs.CreateRun validates it, so a malformed or
	// already-used id is refused before anything is written.
	runID string

	// estimate is --estimate: resolve, load and prepare everything the run
	// would, report what it would cost and stop — before a run row exists.
	// estimateBudget is the probe's wall-clock budget.
	estimate       bool
	estimateBudget time.Duration
}

// requireExperimentalOptIn refuses a run against a standard whose tier demands
// an explicit acknowledgement unless the operator gave one. The message carries
// the whole disclosure — which standard, which tier, why the tier exists and
// which flag proceeds — because domainerrors.AppError has no separate hint
// field and this text is all the user gets.
func requireExperimentalOptIn(resolved framework.ResolvedProfile, experimental bool) error {
	if experimental || !resolved.EvidenceTier.RequiresExperimentalOptIn() {
		return nil
	}

	return domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf(
		"standard %q is evidence tier %q: it carries no normative coefficients, its base levels are invented and it has no octave bands, "+
			"so the levels it would emit are not an implementation of the standard it names and must not be used for assessment; "+
			"pass --experimental to acknowledge that and run it anyway",
		resolved.StandardID, resolved.EvidenceTier,
	), nil)
}

// preparedRun is a run that exists in the project manifest and has not
// computed anything yet. Everything in it is settled before the first number is
// produced, which is what lets the steps after it be about the run rather than
// about validating the request.
type preparedRun struct {
	store        projectfs.Store
	project      project.Project
	run          project.Run
	provenance   project.ProvenanceManifest
	standard     framework.ResolvedProfile
	params       map[string]string
	modelPath    string
	relModelPath string
	runDir       string
	log          *runLog
	inputs       runInputs
}

func executeRunCommand(cmd *cobra.Command, req runCommandRequest) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.run", "command state unavailable", nil)
	}

	if req.estimate {
		return executeRunEstimate(cmd, state, req)
	}

	prepared, err := prepareRun(cmd, state, req)
	if err != nil {
		return err
	}

	result, err := computeRun(cmd.Context(), prepared, state, req)
	if err != nil {
		return err
	}

	err = completeRun(prepared, result)
	if err != nil {
		return err
	}

	return reportRunCompletion(cmd, state, prepared, result)
}

// prepareRun turns the request into a run the project knows about: it resolves
// the standard, refuses what must be refused, and creates the manifest entry.
//
// Every refusal here happens before store.CreateRun, so a rejected run leaves
// the project exactly as it found it — no manifest entry, no run directory, no
// log.
func prepareRun(cmd *cobra.Command, state commandState, req runCommandRequest) (preparedRun, error) {
	resolvedStandard, resolvedParams, err := resolveRunRequest(req)
	if err != nil {
		return preparedRun{}, err
	}

	// How far the numbers about to be produced may be trusted is stated before
	// the run starts, not buried in the artifacts it leaves behind.
	if !state.Config.JSONLogs {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Evidence tier: %s\n", resolvedStandard.EvidenceTier.Headline())
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return preparedRun{}, fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	proj, err := store.Load()
	if err != nil {
		return preparedRun{}, fmt.Errorf("load project manifest: %w", err)
	}

	resolvedModelPath := resolvePath(store.Root(), req.modelPath)
	relModelPath := relativePath(store.Root(), resolvedModelPath)

	standardData, err := buildRunStandardData(resolvedStandard)
	if err != nil {
		return preparedRun{}, err
	}

	// Everything that can refuse the run on its inputs is decided here, before
	// the row exists: the model, the CRS it is computed in, the terrain in that
	// CRS, and whether the standard is wired at all. A refusal then leaves no
	// row, and `aconiq serve` — which answers once the row exists — still
	// answers it synchronously, with the child's exit code, rather than with a
	// 202 for a run that is already failed.
	inputs, err := loadRunInputs(store, proj, state, resolvedStandard.StandardID, resolvedModelPath, relModelPath)
	if err != nil {
		return preparedRun{}, err
	}

	run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
		RunID:      req.runID,
		ScenarioID: req.scenarioID,
		Standard: project.StandardRef{
			Context: resolvedStandard.Context,
			ID:      resolvedStandard.StandardID,
			Version: resolvedStandard.Version,
			Profile: resolvedStandard.Profile,
		},
		ReceiverMode:  req.receiverMode,
		ReceiverSetID: receiverSetID(req.receiverMode),
		Parameters:    resolvedParams,
		Metadata:      buildRunProvenanceMetadata(resolvedStandard, resolvedParams, req.receiverMode),
		StandardData:  standardData,
		InputPaths:    mergeInputPaths(append([]string{relModelPath}, req.inputPaths...)),
		Status:        project.RunStatusRunning,
		LogLines: []string{
			nowUTC().Format(time.RFC3339) + " run started",
		},
	})
	if err != nil {
		return preparedRun{}, fmt.Errorf("create run for scenario %s: %w", req.scenarioID, err)
	}

	log := newRunLog(
		run.StartedAt.Format(time.RFC3339)+" run started",
		fmt.Sprintf("%s standard=%s version=%s profile=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.StandardID, resolvedStandard.Version, resolvedStandard.Profile),
		fmt.Sprintf("%s evidence_tier=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.EvidenceTier),
		fmt.Sprintf("%s model=%s", run.StartedAt.Format(time.RFC3339), relModelPath),
		fmt.Sprintf("%s receiver_mode=%s", run.StartedAt.Format(time.RFC3339), req.receiverMode),
	)

	// What loading the inputs had to say, in the order it said it.
	log.addLines(inputs.logLines)

	// From here on run.log is written as it grows, so a run that is still
	// computing can be read — by `GET /runs/{id}/log` above all.
	log.streamTo(filepath.Join(store.Root(), filepath.FromSlash(run.LogPath)))

	return preparedRun{
		store:        store,
		project:      proj,
		run:          run,
		provenance:   provenance,
		standard:     resolvedStandard,
		params:       resolvedParams,
		modelPath:    resolvedModelPath,
		relModelPath: relModelPath,
		runDir:       filepath.Join(store.Root(), ".noise", "runs", run.ID),
		log:          log,
		inputs:       inputs,
	}, nil
}

// resolveRunRequest settles what the request asks for: its parameters, its
// receiver mode, the standard version and profile they resolve to, and the
// evidence-tier opt-in that standard needs. Every refusal is a user-input
// error, made before anything is read from the project.
func resolveRunRequest(req runCommandRequest) (framework.ResolvedProfile, map[string]string, error) {
	params, err := parseKeyValueFlags(req.rawParams)
	if err != nil {
		return framework.ResolvedProfile{}, nil, err
	}

	err = validateReceiverMode(req.receiverMode)
	if err != nil {
		return framework.ResolvedProfile{}, nil, err
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		return framework.ResolvedProfile{}, nil, domainerrors.New(domainerrors.KindInternal, "cli.run", "initialize standards registry", err)
	}

	resolvedStandard, err := registry.Resolve(req.standardID, req.standardVersion, req.standardProfile)
	if err != nil {
		return framework.ResolvedProfile{}, nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
	}

	resolvedParams, err := resolvedStandard.RunParameterSchema.NormalizeAndValidate(params)
	if err != nil {
		return framework.ResolvedProfile{}, nil, domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
	}

	// A tier whose levels are invented may not be run by accident.
	err = requireExperimentalOptIn(resolvedStandard, req.experimental)
	if err != nil {
		return framework.ResolvedProfile{}, nil, err
	}

	return resolvedStandard, resolvedParams, nil
}

// runInputs is what a run computes from, loaded and checked before the run
// exists.
type runInputs struct {
	model      modelgeojson.Model
	projection computeProjection
	terrain    terrain.Model
	module     runModule
	// logLines is what loading them had to say, stamped when it was said, for
	// the run log once there is one.
	logLines []string
}

// loadRunInputs loads the model, moves it into the compute CRS, aligns the
// terrain with that CRS and looks up the standard's module. Each refusal is a
// user-input error where the cause is the project or the request — a model
// that is missing or does not validate, a CRS that cannot be projected, a
// terrain that cannot be queried in it — so the run command exits 2.
func loadRunInputs(
	store projectfs.Store,
	proj project.Project,
	state commandState,
	standardID string,
	modelPath string,
	relModelPath string,
) (runInputs, error) {
	var inputs runInputs

	logf := func(format string, args ...any) {
		inputs.logLines = append(inputs.logLines, nowUTC().Format(time.RFC3339)+" "+fmt.Sprintf(format, args...))
	}

	model, err := loadValidatedModel(modelPath, proj.CRS, relModelPath)
	if err != nil {
		return runInputs{}, err
	}

	model, projection, err := resolveComputeModel(model, proj.CRS)
	if err != nil {
		return runInputs{}, err
	}

	if projection.Applied {
		logf("project_crs=%s is geographic; computing in compute_crs=%s", projection.ProjectCRS, projection.ComputeCRS)
	}

	// The terrain is stored in the project CRS and is not moved with the
	// model, so a projected run has to query it through a transform.
	runTerrain, err := newTerrainInComputeCRS(loadRunTerrain(store, proj, state, logf), projection)
	if err != nil {
		return runInputs{}, domainerrors.New(domainerrors.KindUserInput, "cli.run",
			"the project terrain cannot be queried in the compute CRS: "+err.Error(), err)
	}

	module, err := runModuleFor(standardID)
	if err != nil {
		return runInputs{}, err
	}

	inputs.model = model
	inputs.projection = projection
	inputs.terrain = runTerrain
	inputs.module = module

	return inputs, nil
}

// computeRun loads the inputs and hands them to the standard's module. From
// here on a failure is a failed run: it is recorded in the manifest and in the
// log, rather than returned as if nothing had happened.
func computeRun(ctx context.Context, prepared preparedRun, state commandState, req runCommandRequest) (runModuleResult, error) {
	projection := prepared.inputs.projection

	err := prepared.store.MergeRunProvenanceMetadata(prepared.run.ID, map[string]string{
		provenanceProjectCRSKey: projection.ProjectCRS,
		provenanceComputeCRSKey: projection.ComputeCRS,
	})
	if err != nil {
		prepared.log.addf("failed to record compute CRS: %v", err)

		return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), err)
	}

	result, err := prepared.inputs.module(ctx, runModuleInput{
		standard:     prepared.standard,
		params:       prepared.params,
		model:        prepared.inputs.model,
		terrain:      prepared.inputs.terrain,
		receiverMode: req.receiverMode,
		workers:      req.workers,
		runDir:       prepared.runDir,
		runID:        prepared.run.ID,
		cacheDir:     state.Config.CacheDir,
		log:          prepared.log,
		progress:     newRunProgress(ctx, prepared.store, prepared.run.ID, prepared.log),
		projection:   projection,
		mergeProvenance: func(metadata map[string]string) error {
			return prepared.store.MergeRunProvenanceMetadata(prepared.run.ID, metadata)
		},
	})
	if err != nil {
		// A refusal before the module touched anything — an option it cannot
		// parse, a receiver mode it does not support — used to leave the row
		// reading `running`, for `aconiq serve` to close later with no reason
		// given. It is a failed run like any other now, with the refusal as
		// the last thing its log says, so the run panel can show why.
		var before beforeRunError
		if errors.As(err, &before) {
			prepared.log.addf("run refused: %v", before.err)

			return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), before.err)
		}

		// A cancellation is recorded as a failed run, as any other stop in the
		// module is; the log says which it was, since the status cannot.
		if errors.Is(err, context.Canceled) {
			prepared.log.addf("run cancelled: %v", context.Cause(ctx))
		}

		return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), err)
	}

	return result, nil
}

// loadRunTerrain returns the project's imported DTM, or nil. A terrain that
// fails to load is a warning rather than a failure: the run continues without
// elevation, and says so in both the log and the structured logger.
func loadRunTerrain(store projectfs.Store, proj project.Project, state commandState, logf func(string, ...any)) terrain.Model {
	artifactPath := findArtifactPath(proj, project.ArtifactIDTerrain)
	if artifactPath == "" {
		return nil
	}

	model, err := terrain.Load(filepath.Join(store.Root(), artifactPath))
	if err != nil {
		state.Logger.Warn("terrain DTM load failed, continuing without terrain", "error", err)
		logf("terrain load warning: %v", err)

		return nil
	}

	logf("terrain loaded from %s", artifactPath)

	return model
}

// completeRun records the finished run: its artifacts, its closing log lines
// and its status.
func completeRun(prepared preparedRun, result runModuleResult) error {
	artifacts := buildRunArtifacts(prepared.store.Root(), prepared.run.ID, result.persisted)

	prepared.log.addf("output_hash=%s", result.outputHash)
	prepared.log.addf("persisted=%s", relativePath(prepared.store.Root(), result.persisted.SummaryPath))
	prepared.log.addf("run completed")

	return finalizeRun(prepared.store, prepared.run, project.RunStatusCompleted, result.finishedAt, prepared.log.all(), artifacts)
}

// reportRunCompletion writes what the operator sees, in whichever form they
// asked for.
func reportRunCompletion(cmd *cobra.Command, state commandState, prepared preparedRun, result runModuleResult) error {
	run := prepared.run
	resultsPath := relativePath(prepared.store.Root(), filepath.Join(prepared.runDir, "results"))

	// output_hash belongs in the completion event: it is how an operator ties a
	// structured log line back to the results on disk, and determinism is only
	// auditable if the hash travels with every record of a run.
	state.Logger.Info(
		"run completed",
		"run_id", run.ID,
		"status", project.RunStatusCompleted,
		"standard_id", run.Standard.ID,
		"provenance", prepared.provenance.ManifestPath,
		"output_hash", result.outputHash,
	)

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":          runCommandName,
			"run_id":           run.ID,
			"status":           string(project.RunStatusCompleted),
			"scenario":         run.ScenarioID,
			"standard":         run.Standard.ID,
			"standard_version": run.Standard.Version,
			"standard_profile": run.Standard.Profile,
			evidenceTierKey:    string(prepared.standard.EvidenceTier),
			"provenance_path":  prepared.provenance.ManifestPath,
			"results_path":     resultsPath,
		})
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Completed run %s (%s)\n", run.ID, project.RunStatusCompleted)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provenance: %s\n", prepared.provenance.ManifestPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Results: %s\n", resultsPath)

	return nil
}

// findArtifactPath returns the path for the artifact with the given ID, or empty string if not found.
func findArtifactPath(proj project.Project, id string) string {
	for _, a := range proj.Artifacts {
		if a.ID == id {
			return a.Path
		}
	}

	return ""
}
