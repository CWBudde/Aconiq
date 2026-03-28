package cli

import (
	"fmt"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var projectName string
	var crs string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project folder and manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.init", "command state unavailable", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Init(projectName, crs)
			if err != nil {
				return err
			}

			state.Logger.Info(
				"project initialized",
				"project_id", proj.ProjectID,
				"project", store.Root(),
				"manifest", store.ManifestPath(),
			)

			if state.Config.JSONLogs {
				return writeCommandOutput(cmd.OutOrStdout(), true, map[string]string{
					"command":       "init",
					"project_id":    proj.ProjectID,
					"project_name":  proj.Name,
					"project_path":  store.Root(),
					"manifest_path": store.ManifestPath(),
					"crs":           proj.CRS,
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Initialized project %q at %s\n", proj.Name, store.Root())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Manifest: %s\n", store.ManifestPath())

			return nil
		},
	}

	cmd.Flags().StringVar(&projectName, "name", "", "Project display name (defaults to project directory name)")
	cmd.Flags().StringVar(&crs, "crs", "EPSG:4326", "Project CRS identifier")

	return cmd
}
