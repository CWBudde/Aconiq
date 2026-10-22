package cli

import (
	"time"

	"github.com/aconiq/backend/internal/runestimate"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/spf13/cobra"
)

// runCommandName is the command's name, and the "command" its JSON output
// reports.
const runCommandName = "run"

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
		estimate        bool
		estimateBudget  time.Duration
		facade          facadeRequest
	)

	cmd := &cobra.Command{
		Use:   runCommandName,
		Short: "Execute a run and persist result artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, err := startRunProfiles(cpuProfile, memProfile)
			if err != nil {
				return err
			}

			// A spacing of 0 is refused by the generator, but an absent one
			// is refused here, naming the flag: there is no default.
			facade.spacingSet = cmd.Flags().Changed("facade-spacing-m")

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
				estimate:        estimate,
				estimateBudget:  estimateBudget,
				facade:          facade,
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
	cmd.Flags().StringVar(&receiverMode, "receiver-mode", receiverModeAutoGrid, "Receiver mode: auto-grid, custom or facade")
	cmd.Flags().StringArrayVar(&facade.buildings, "facade-building", nil, "Building feature id whose facades carry receivers in receiver mode facade (repeatable)")
	cmd.Flags().Float64Var(&facade.spacingM, "facade-spacing-m", 0, "Station spacing along each facade in metres, required in receiver mode facade")
	cmd.Flags().BoolVar(&facade.includeCourtyard, "facade-courtyard", false, "Also place facade receivers on courtyard facades")
	cmd.Flags().BoolVar(&facade.allowUnconfirmed, "allow-unconfirmed", false, "Run a normative standard over facade receivers that are proposed or have an assumed slab height, and record their counts")
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Run parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&inputPaths, "input", nil, "Input path to hash into provenance (repeatable)")
	cmd.Flags().BoolVar(&experimental, "experimental", false, "Acknowledge that a scaffold-tier standard emits invented levels and run it anyway")
	cmd.Flags().IntVar(&workers, "workers", 0, "Compute worker count (0 = one per available CPU)")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Report what the run would cost in time, from a short probe of its own kernel, and exit without creating a run")
	cmd.Flags().DurationVar(&estimateBudget, "estimate-budget", runestimate.DefaultBudget, "Wall-clock budget of the --estimate probe")
	cmd.Flags().StringVar(&cpuProfile, "cpuprofile", "", "Write a pprof CPU profile of the command to this file (written on failure and cancellation too)")
	cmd.Flags().StringVar(&memProfile, "memprofile", "", "Write a pprof heap profile, taken after a GC at the end of the command, to this file")

	// Hidden: it is how `aconiq serve` names the run it starts, not something
	// a person at a terminal has a reason to choose.
	cmd.Flags().StringVar(&runID, "run-id", "", "Record the run under this id instead of minting one")
	cmd.Flags().Lookup("run-id").Hidden = true

	return cmd
}
