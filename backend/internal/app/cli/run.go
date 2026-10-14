package cli

import (
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var (
		scenarioID      string
		standardID      string
		standardVersion string
		standardProfile string
		modelPath       string
		receiverMode    string
		rawParams       []string
		inputPaths      []string
		experimental    bool
		workers         int
		cpuProfile      string
		memProfile      string
		runID           string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a run and persist result artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, err := startRunProfiles(cpuProfile, memProfile)
			if err != nil {
				return err
			}

			runErr := executeRunCommand(cmd, runCommandRequest{
				scenarioID:      scenarioID,
				standardID:      standardID,
				standardVersion: standardVersion,
				standardProfile: standardProfile,
				modelPath:       modelPath,
				receiverMode:    receiverMode,
				rawParams:       rawParams,
				inputPaths:      inputPaths,
				experimental:    experimental,
				workers:         workers,
				runID:           runID,
			})

			return finishRunProfiles(cmd, profiles, runErr)
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
	cmd.Flags().BoolVar(&experimental, "experimental", false, "Acknowledge that a scaffold-tier standard emits invented levels and run it anyway")
	cmd.Flags().IntVar(&workers, "workers", 0, "Compute worker count (0 = one per available CPU)")
	cmd.Flags().StringVar(&cpuProfile, "cpuprofile", "", "Write a pprof CPU profile of the command to this file (written on failure and cancellation too)")
	cmd.Flags().StringVar(&memProfile, "memprofile", "", "Write a pprof heap profile, taken after a GC at the end of the command, to this file")

	// Hidden: it is how `aconiq serve` names the run it starts, not something
	// a person at a terminal has a reason to choose.
	cmd.Flags().StringVar(&runID, "run-id", "", "Record the run under this id instead of minting one")
	cmd.Flags().Lookup("run-id").Hidden = true

	return cmd
}
