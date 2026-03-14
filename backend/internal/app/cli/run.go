package cli

import (
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var scenarioID string
	var standardID string
	var standardVersion string
	var standardProfile string
	var modelPath string
	var receiverMode string
	var rawParams []string
	var inputPaths []string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a run and persist result artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeRunCommand(cmd, runCommandRequest{
				scenarioID:      scenarioID,
				standardID:      standardID,
				standardVersion: standardVersion,
				standardProfile: standardProfile,
				modelPath:       modelPath,
				receiverMode:    receiverMode,
				rawParams:       rawParams,
				inputPaths:      inputPaths,
			})
		},
	}

	cmd.Flags().StringVar(&scenarioID, "scenario", "default", "Scenario ID")
	cmd.Flags().StringVar(&standardID, "standard", freefield.StandardID, "Standard identifier")
	cmd.Flags().StringVar(&standardID, "standard-id", freefield.StandardID, "Deprecated alias for --standard")
	cmd.Flags().StringVar(&standardVersion, "standard-version", "", "Standard version (defaults to standard default)")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "", "Standard profile (defaults to version profile default)")
	cmd.Flags().StringVar(&modelPath, "model", defaultModelPath, "Path to normalized GeoJSON model")
	cmd.Flags().StringVar(&receiverMode, "receiver-mode", receiverModeAutoGrid, "Receiver mode: auto-grid or custom")
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Run parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&inputPaths, "input", nil, "Input path to hash into provenance (repeatable)")

	return cmd
}
