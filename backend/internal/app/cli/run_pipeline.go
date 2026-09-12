package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
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
}

func executeRunCommand(cmd *cobra.Command, req runCommandRequest) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.run", "command state unavailable", nil)
	}

	prepared, err := prepareRun(cmd, state, req)
	if err != nil {
		return err
	}

	result, err := computeRun(prepared, state, req)
	if err != nil {
		return err
	}

	err = completeRun(prepared, result)
	if err != nil {
		return err
	}

	return reportRunCompletion(cmd, state, prepared)
}

// prepareRun turns the request into a run the project knows about: it resolves
// the standard, refuses what must be refused, and creates the manifest entry.
//
// Every refusal here happens before store.CreateRun, so a rejected run leaves
// the project exactly as it found it — no manifest entry, no run directory, no
// log.
func prepareRun(cmd *cobra.Command, state commandState, req runCommandRequest) (preparedRun, error) {
	params, err := parseKeyValueFlags(req.rawParams)
	if err != nil {
		return preparedRun{}, err
	}

	err = validateReceiverMode(req.receiverMode)
	if err != nil {
		return preparedRun{}, err
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		return preparedRun{}, domainerrors.New(domainerrors.KindInternal, "cli.run", "initialize standards registry", err)
	}

	resolvedStandard, err := registry.Resolve(req.standardID, req.standardVersion, req.standardProfile)
	if err != nil {
		return preparedRun{}, domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
	}

	resolvedParams, err := resolvedStandard.RunParameterSchema.NormalizeAndValidate(params)
	if err != nil {
		return preparedRun{}, domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
	}

	// A tier whose levels are invented may not be run by accident.
	err = requireExperimentalOptIn(resolvedStandard, req.experimental)
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

	run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
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
		log: newRunLog(
			run.StartedAt.Format(time.RFC3339)+" run started",
			fmt.Sprintf("%s standard=%s version=%s profile=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.StandardID, resolvedStandard.Version, resolvedStandard.Profile),
			fmt.Sprintf("%s evidence_tier=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.EvidenceTier),
			fmt.Sprintf("%s model=%s", run.StartedAt.Format(time.RFC3339), relModelPath),
			fmt.Sprintf("%s receiver_mode=%s", run.StartedAt.Format(time.RFC3339), req.receiverMode),
		),
	}, nil
}

// computeRun loads the inputs and hands them to the standard's module. From
// here on a failure is a failed run: it is recorded in the manifest and in the
// log, rather than returned as if nothing had happened.
func computeRun(prepared preparedRun, state commandState, req runCommandRequest) (runModuleResult, error) {
	model, err := loadValidatedModel(prepared.modelPath, prepared.project.CRS, prepared.relModelPath)
	if err != nil {
		prepared.log.addf("failed to load model: %v", err)

		return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), err)
	}

	module, err := runModuleFor(prepared.standard.StandardID)
	if err != nil {
		prepared.log.addf("run wiring missing: %v", err)

		return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), err)
	}

	result, err := module(runModuleInput{
		standard:     prepared.standard,
		params:       prepared.params,
		model:        model,
		terrain:      loadRunTerrain(prepared, state),
		receiverMode: req.receiverMode,
		runDir:       prepared.runDir,
		runID:        prepared.run.ID,
		cacheDir:     state.Config.CacheDir,
		log:          prepared.log,
		mergeProvenance: func(metadata map[string]string) error {
			return prepared.store.MergeRunProvenanceMetadata(prepared.run.ID, metadata)
		},
	})
	if err != nil {
		// A failure before the module touched anything leaves the run as it
		// found it; anything else is a failed run and is recorded as one.
		var before beforeRunError
		if errors.As(err, &before) {
			return runModuleResult{}, before.err
		}

		return runModuleResult{}, finalizeRunFailure(prepared.store, prepared.run, prepared.log.all(), err)
	}

	return result, nil
}

// loadRunTerrain returns the project's imported DTM, or nil. A terrain that
// fails to load is a warning rather than a failure: the run continues without
// elevation, and says so in both the log and the structured logger.
func loadRunTerrain(prepared preparedRun, state commandState) terrain.Model {
	artifactPath := findArtifactPath(prepared.project, "artifact-terrain")
	if artifactPath == "" {
		return nil
	}

	model, err := terrain.Load(filepath.Join(prepared.store.Root(), artifactPath))
	if err != nil {
		state.Logger.Warn("terrain DTM load failed, continuing without terrain", "error", err)
		prepared.log.addf("terrain load warning: %v", err)

		return nil
	}

	prepared.log.addf("terrain loaded from %s", artifactPath)

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
func reportRunCompletion(cmd *cobra.Command, state commandState, prepared preparedRun) error {
	run := prepared.run
	resultsPath := relativePath(prepared.store.Root(), filepath.Join(prepared.runDir, "results"))

	state.Logger.Info(
		"run completed",
		"run_id", run.ID,
		"status", project.RunStatusCompleted,
		"standard_id", run.Standard.ID,
		"provenance", prepared.provenance.ManifestPath,
	)

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":          "run",
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
