package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	var (
		limit     int
		tailLines int
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show project status, run list, and recent logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, limit, tailLines)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of runs to show")
	cmd.Flags().IntVar(&tailLines, "tail", 5, "Number of lines to show from latest run log")

	return cmd
}

func runStatusCommand(cmd *cobra.Command, limit, tailLines int) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.status", "command state unavailable", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	proj, err := store.Load()
	if err != nil {
		return fmt.Errorf("load project manifest: %w", err)
	}

	latestRun, hasLatest := latestRun(proj.Runs)
	if hasLatest {
		state.Logger.Info("status requested", "project_id", proj.ProjectID, "run_count", len(proj.Runs), "last_status", latestRun.Status)
	} else {
		state.Logger.Info("status requested", "project_id", proj.ProjectID, "run_count", len(proj.Runs), "last_status", "none")
	}

	if limit <= 0 {
		limit = 10
	}

	if state.Config.JSONLogs {
		return writeStatusJSON(cmd, store.Root(), proj, latestRun, hasLatest, limit)
	}

	writeStatusSummary(cmd.OutOrStdout(), store.Root(), proj, latestRun, hasLatest)
	writeStatusRuns(cmd.OutOrStdout(), proj, limit)

	return writeStatusLogTail(cmd, store.Root(), latestRun, hasLatest, tailLines)
}

type statusRunEntry struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ScenarioID      string `json:"scenario"`
	StandardID      string `json:"standard"`
	StandardVersion string `json:"standard_version"`
	StandardProfile string `json:"standard_profile"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	LogPath         string `json:"log_path"`
}

func writeStatusJSON(
	cmd *cobra.Command,
	root string,
	proj project.Project,
	latest project.Run,
	hasLatest bool,
	limit int,
) error {
	runs := make([]statusRunEntry, 0, len(proj.Runs))

	start := max(len(proj.Runs)-limit, 0)
	for _, r := range proj.Runs[start:] {
		runs = append(runs, statusRunEntry{
			ID:              r.ID,
			Status:          r.Status,
			ScenarioID:      r.ScenarioID,
			StandardID:      r.Standard.ID,
			StandardVersion: r.Standard.Version,
			StandardProfile: r.Standard.Profile,
			StartedAt:       r.StartedAt.Format(time.RFC3339),
			FinishedAt:      r.FinishedAt.Format(time.RFC3339),
			LogPath:         r.LogPath,
		})
	}

	payload := map[string]any{
		"command":          "status",
		"project_id":       proj.ProjectID,
		"project_name":     proj.Name,
		"project_path":     root,
		"manifest_version": proj.ManifestVersion,
		"crs":              proj.CRS,
		"scenario_count":   len(proj.Scenarios),
		"runs":             runs,
	}
	if hasLatest {
		payload["last_run_id"] = latest.ID
		payload["last_run_status"] = latest.Status
	}

	return writeCommandOutput(cmd.OutOrStdout(), true, payload)
}

func writeStatusSummary(out io.Writer, root string, proj project.Project, latest project.Run, hasLatest bool) {
	_, _ = fmt.Fprintf(out, "Project: %s (%s)\n", proj.Name, proj.ProjectID)
	_, _ = fmt.Fprintf(out, "Path: %s\n", root)
	_, _ = fmt.Fprintf(out, "Manifest Version: v%d\n", proj.ManifestVersion)
	_, _ = fmt.Fprintf(out, "CRS: %s\n", proj.CRS)

	_, _ = fmt.Fprintf(out, "Scenarios: %d\n", len(proj.Scenarios))
	if hasLatest {
		_, _ = fmt.Fprintf(out, "Last Run Status: %s (%s)\n", latest.Status, latest.ID)
	} else {
		_, _ = fmt.Fprintln(out, "Last Run Status: none")
	}
}

func writeStatusRuns(out io.Writer, proj project.Project, limit int) {
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Runs:")

	if len(proj.Runs) == 0 {
		_, _ = fmt.Fprintln(out, "  (no runs yet)")

		return
	}

	start := max(len(proj.Runs)-limit, 0)

	for _, run := range proj.Runs[start:] {
		_, _ = fmt.Fprintf(
			out,
			"  - %s status=%s scenario=%s standard=%s@%s/%s started=%s finished=%s log=%s\n",
			run.ID,
			run.Status,
			run.ScenarioID,
			run.Standard.ID,
			run.Standard.Version,
			run.Standard.Profile,
			run.StartedAt.Format(time.RFC3339),
			run.FinishedAt.Format(time.RFC3339),
			run.LogPath,
		)
	}
}

func writeStatusLogTail(cmd *cobra.Command, root string, latest project.Run, hasLatest bool, tailLines int) error {
	if !hasLatest || tailLines <= 0 {
		return nil
	}

	fullLogPath := filepath.Join(root, filepath.FromSlash(latest.LogPath))

	tail, err := readTail(fullLogPath, tailLines)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(out, "")

	_, _ = fmt.Fprintf(out, "Recent log lines (%s):\n", latest.ID)
	if len(tail) == 0 {
		_, _ = fmt.Fprintln(out, "  (no log lines)")
	} else {
		for _, line := range tail {
			_, _ = fmt.Fprintf(out, "  %s\n", line)
		}
	}

	return nil
}

func latestRun(runs []project.Run) (project.Run, bool) {
	if len(runs) == 0 {
		return project.Run{}, false
	}

	return runs[len(runs)-1], true
}

func readTail(path string, lines int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.readTail", "open run log: "+path, err)
	}
	defer file.Close()

	all := make([]string, 0, lines)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		all = append(all, strings.TrimRight(scanner.Text(), "\r\n"))
	}

	err = scanner.Err()
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.readTail", "scan run log: "+path, err)
	}

	if lines >= len(all) {
		return all, nil
	}

	return all[len(all)-lines:], nil
}
