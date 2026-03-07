package cli

import (
	"fmt"
	"path/filepath"

	"github.com/aconiq/backend/internal/api/httpv1"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/spf13/cobra"
)

func newOpenAPICommand() *cobra.Command {
	var outPath string
	var serverURL string

	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "Export OpenAPI contract for local API v1",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.openapi", "command state unavailable", nil)
			}

			if outPath == "" {
				outPath = filepath.Join(".noise", "api", "openapi.v1.json")
			}

			resolvedOut := resolvePath(state.Config.ProjectPath, outPath)

			err := httpv1.WriteOpenAPISpec(resolvedOut, serverURL)
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.openapi", "write openapi spec", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "OpenAPI exported: %s\n", resolvedOut)

			return nil
		},
	}

	cmd.Flags().StringVar(&outPath, "out", filepath.Join(".noise", "api", "openapi.v1.json"), "Output path for OpenAPI JSON file")
	cmd.Flags().StringVar(&serverURL, "server-url", "", "Server URL to embed in OpenAPI servers[0].url")

	return cmd
}
