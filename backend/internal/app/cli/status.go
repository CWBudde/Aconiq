package cli

import (
	"bufio"
	"fmt"
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
	var limit int
	var tailLines int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show project status, run list, and recent logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.status", "command state unavailable", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
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
				type runEntry struct {
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

				runs := make([]runEntry, 0, len(proj.Runs))
				start := max(len(proj.Runs)-limit, 0)
				for _, r := range proj.Runs[start:] {
					runs = append(runs, runEntry{
						ID:              r.ID,
						Status:          string(r.Status),
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
					"project_path":     store.Root(),
					"manifest_version": proj.ManifestVersion,
					"crs":              proj.CRS,
					"scenario_count":   len(proj.Scenarios),
					"runs":             runs,
				}
				if hasLatest {
					payload["last_run_id"] = latestRun.ID
					payload["last_run_status"] = string(latestRun.Status)
				}
				return writeCommandOutput(cmd.OutOrStdout(), true, payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Project: %s (%s)\n", proj.Name, proj.ProjectID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Path: %s\n", store.Root())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Manifest Version: v%d\n", proj.ManifestVersion)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CRS: %s\n", proj.CRS)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Scenarios: %d\n", len(proj.Scenarios))
			if hasLatest {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last Run Status: %s (%s)\n", latestRun.Status, latestRun.ID)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Last Run Status: none")
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Runs:")

			if len(proj.Runs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (no runs yet)")
			} else {
				start := max(len(proj.Runs)-limit, 0)

				for _, run := range proj.Runs[start:] {
					_, _ = fmt.Fprintf(
						cmd.OutOrStdout(),
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

			if hasLatest && tailLines > 0 {
				fullLogPath := filepath.Join(store.Root(), filepath.FromSlash(latestRun.LogPath))

				tail, err := readTail(fullLogPath, tailLines)
				if err != nil {
					return err
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Recent log lines (%s):\n", latestRun.ID)
				if len(tail) == 0 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (no log lines)")
				} else {
					for _, line := range tail {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", line)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of runs to show")
	cmd.Flags().IntVar(&tailLines, "tail", 5, "Number of lines to show from latest run log")

	return cmd
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
