package cli

import (
	"fmt"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

// newDeleteRunCommand is the CLI twin of DELETE /api/v1/runs/{id}. Every other
// mutating endpoint has one, and the command table is meant to say what the
// tool can do — an operation reachable only over HTTP would make it lie.
func newDeleteRunCommand() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "delete-run",
		Short: "Delete a run and its run directory",
		Long: "Removes the run from the project manifest, drops every artifact ref belonging to it, and " +
			"deletes .noise/runs/<id>/.\n\n" +
			"Export bundles under .noise/exports/ are kept: a bundle may already have been delivered, and " +
			"deleting one as a side effect of removing a run is not what the word means. The bundles whose " +
			"refs were dropped are listed as retained.\n\n" +
			"A run that is still pending or running is refused — its directory is being written.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteRunCommand(cmd, runID)
		},
	}

	cmd.Flags().StringVar(&runID, "run", "", "ID of the run to delete (required)")

	return cmd
}

func runDeleteRunCommand(cmd *cobra.Command, runID string) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.delete-run", "command state unavailable", nil)
	}

	if runID == "" {
		return domainerrors.New(domainerrors.KindUserInput, "cli.delete-run", "--run is required", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	result, err := store.DeleteRun(runID)
	if err != nil {
		return fmt.Errorf("delete run %s: %w", runID, err)
	}

	state.Logger.Info(
		"run deleted",
		"run_id", result.RunID,
		"removed", len(result.RemovedPaths),
		"retained", len(result.RetainedPaths),
	)

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":        "delete-run",
			"run_id":         result.RunID,
			"removed_paths":  result.RemovedPaths,
			"retained_paths": result.RetainedPaths,
		})
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted run %s\n", result.RunID)

	for _, path := range result.RemovedPaths {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  removed:  %s\n", path)
	}

	for _, path := range result.RetainedPaths {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  kept:     %s\n", path)
	}

	return nil
}
