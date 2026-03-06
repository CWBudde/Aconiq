package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/soundplan/soundplan/backend/internal/app/config"
	"github.com/soundplan/soundplan/backend/internal/app/logging"
	domainerrors "github.com/soundplan/soundplan/backend/internal/domain/errors"
	"github.com/spf13/cobra"
)

type commandState struct {
	Config config.Config
	Logger *slog.Logger
	Run    logging.CommandRun
}

type commandStateKey struct{}

// Execute runs the noise CLI and maps known user errors to a dedicated exit code.
func Execute(args []string) int {
	rootCmd := newRootCommand()
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	state, hasState := stateFromCommand(rootCmd)
	if hasState {
		state.Run.End(err)
	}

	if err == nil {
		return 0
	}

	if hasState {
		state.Logger.Error(
			"command failed",
			slog.Bool("user_error", domainerrors.IsUserError(err)),
			slog.Any("error", err),
		)
	} else {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}

	if domainerrors.IsUserError(err) {
		return 2
	}

	return 1
}

func newRootCommand() *cobra.Command {
	var projectPath string
	var cacheDir string
	var verbose bool
	var jsonLogs bool

	rootCmd := &cobra.Command{
		Use:           "noise",
		Short:         "Environmental noise modeling CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromFlags(projectPath, cacheDir, verbose, jsonLogs)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "config.FromFlags", "failed to resolve runtime configuration", err)
			}

			logger := logging.New(cfg.LogLevel, cfg.JSONLogs)
			run := logging.Begin(logger, cmd.CommandPath(), args)

			setState(cmd, commandState{
				Config: cfg,
				Logger: logger,
				Run:    run,
			})

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&projectPath, "project", "", "Path to project directory (defaults to current working directory)")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "", "Path to cache directory (defaults to <project>/.noise/cache)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable debug-level logs")
	rootCmd.PersistentFlags().BoolVar(&jsonLogs, "json", false, "Emit logs as JSON")

	rootCmd.AddCommand(
		newInitCommand(),
		newImportCommand(),
		newValidateCommand(),
		newRunCommand(),
		newStatusCommand(),
		newExportCommand(),
		newServeCommand(),
		newOpenAPICommand(),
		newPlaceholderCommand("bench", "Run benchmark scenarios"),
	)

	return rootCmd
}

func newPlaceholderCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if ok {
				state.Logger.Info(
					"placeholder command executed",
					slog.String("command", cmd.CommandPath()),
					slog.String("project", state.Config.ProjectPath),
					slog.String("cache_dir", state.Config.CacheDir),
				)
			}

			return nil
		},
	}
}

func setState(cmd *cobra.Command, state commandState) {
	ctx := context.WithValue(cmd.Root().Context(), commandStateKey{}, state)
	cmd.Root().SetContext(ctx)
	cmd.SetContext(ctx)
}

func stateFromCommand(cmd *cobra.Command) (commandState, bool) {
	ctx := cmd.Root().Context()
	if ctx == nil {
		return commandState{}, false
	}

	state, ok := ctx.Value(commandStateKey{}).(commandState)
	if !ok {
		return commandState{}, false
	}

	return state, true
}
