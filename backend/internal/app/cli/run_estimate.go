package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/runestimate"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/spf13/cobra"
)

// runEstimator prices one standard's run: it prepares what the run would
// prepare and probes it, computing no result.
type runEstimator func(context.Context, runModuleInput, runestimate.Options) (runestimate.Estimate, error)

// runEstimatorTable holds the standards that can say what a run will cost.
// A standard missing here answers "no estimate available", which is the
// truthful answer for a module whose cost nobody has measured — not an error,
// and not a guess.
var runEstimatorTable = map[string]runEstimator{
	rls19road.StandardID: estimateRLS19RoadRun,
}

// estimateRLS19RoadRun probes the RLS-19 run prepareRLS19RoadRun describes, on
// the worker count and the chunk schedule computeRLS19RoadReceivers would
// use.
func estimateRLS19RoadRun(ctx context.Context, input runModuleInput, opts runestimate.Options) (runestimate.Estimate, error) {
	run, err := prepareRLS19RoadRun(input)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	opts.Workers = resolveComputeWorkers(input.workers)
	opts.Schedule = runestimate.SchedulePull

	estimate, err := rls19road.Estimate(ctx, run.receivers, run.roadSources, run.barriers, run.config, opts)
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("estimate RLS-19 run: %w", err)
	}

	return estimate, nil
}

// executeRunEstimate is `aconiq run --estimate`: the run's own pipeline up to
// the prepared scene, then a probe, then a report — and no run.
//
// It makes every refusal `aconiq run` makes before its first receiver, and in
// the same words, because it calls the same functions: resolveRunRequest,
// loadValidatedModel, resolveComputeModel and the standard's own preparation.
// What it never calls is store.CreateRun, so the manifest, the runs directory
// and the logs are left exactly as they were. The run log the preparation
// writes to is an in-memory one that is dropped.
func executeRunEstimate(cmd *cobra.Command, state commandState, req runCommandRequest) error {
	standard, params, err := resolveRunRequest(req)
	if err != nil {
		return err
	}

	if !state.Config.JSONLogs {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Evidence tier: %s\n", standard.EvidenceTier.Headline())
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	proj, err := store.Load()
	if err != nil {
		return fmt.Errorf("load project manifest: %w", err)
	}

	modelPath := resolvePath(store.Root(), req.modelPath)

	inputs, err := loadRunInputs(store, proj, state, standard.StandardID, modelPath, relativePath(store.Root(), modelPath))
	if err != nil {
		return err
	}

	standardID := standard.StandardID
	log := newRunLog()

	estimate := runestimate.Unavailable(standardID, fmt.Sprintf(
		"no estimate available: %s has no kernel probe, so nothing has measured what its receivers cost", standardID,
	))

	if estimator, ok := runEstimatorTable[standardID]; ok {
		estimate, err = estimator(cmd.Context(), runModuleInput{
			standard:     standard,
			params:       params,
			model:        inputs.model,
			terrain:      inputs.terrain,
			receiverMode: req.receiverMode,
			workers:      req.workers,
			cacheDir:     state.Config.CacheDir,
			log:          log,
			projection:   inputs.projection,
		}, runestimate.Options{Budget: req.estimateBudget})
		if err != nil {
			var before beforeRunError
			if errors.As(err, &before) {
				return before.err
			}

			return err
		}
	}

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":          runCommandName,
			"mode":             "estimate",
			"standard":         standardID,
			"standard_version": standard.Version,
			"standard_profile": standard.Profile,
			evidenceTierKey:    string(standard.EvidenceTier),
			"estimate":         estimate,
		})
	}

	writeEstimateText(cmd.OutOrStdout(), estimate)

	return nil
}

// writeEstimateText is the estimate for a person at a terminal.
func writeEstimateText(w io.Writer, estimate runestimate.Estimate) {
	if !estimate.Available {
		_, _ = fmt.Fprintf(w, "Estimate for %s: %s\n", estimate.StandardID, estimate.Reason)
		_, _ = fmt.Fprintln(w, "No run was created.")

		return
	}

	inputs, probe := estimate.Inputs, estimate.Probe

	_, _ = fmt.Fprintf(w, "Estimate for %s: %s on %d workers (CPU %s)\n",
		estimate.StandardID, runestimate.FormatBand(*estimate.WallSeconds), inputs.Workers,
		runestimate.FormatBand(*estimate.CPUSeconds))
	_, _ = fmt.Fprintf(w, "  receivers: %d, work units: %d (%.0f per receiver)\n",
		inputs.Receivers, inputs.Units, float64(inputs.Units)/float64(max(inputs.Receivers, 1)))

	if len(inputs.Counts) > 0 {
		_, _ = fmt.Fprintf(w, "  scene: %s\n", formatCounts(inputs.Counts))
	}

	_, _ = fmt.Fprintf(w, "  probe: %d of %d receivers measured, %d units timed in %.1f s; scene prepared in %.1f s\n",
		probe.MeasuredReceivers, probe.ProbeReceivers, probe.UnitsTimed, probe.ElapsedSeconds, probe.PrepareSeconds)

	for _, alt := range estimate.Alts {
		_, _ = fmt.Fprintf(w, "  with %s: %s (%d work units)\n",
			formatParams(alt.Params), runestimate.FormatBand(alt.WallSeconds), alt.Units)
	}

	_, _ = fmt.Fprintln(w, "No run was created.")
}

func formatCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}

	return strings.Join(parts, " ")
}

func formatParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}

	return strings.Join(parts, " ")
}
