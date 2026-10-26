package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aconiq/backend/internal/api/httpv1"
	"github.com/aconiq/backend/internal/buildinfo"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/mcpserver"
	"github.com/aconiq/backend/internal/standards"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
	var (
		allowWrites       bool
		photoTransmission string
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the project to an AI agent over the Model Context Protocol (stdio)",
		Long: `Serve the project to an AI agent over the Model Context Protocol.

The server speaks MCP over stdin and stdout and opens no socket: whoever can
talk to it started it. stdout carries the protocol and nothing else; logs go to
stderr. The project is the one --project names, fixed for the life of the
process, and no tool takes a path.

Each tool is a thin adapter over the local HTTP API, served in process, so a
tool accepts and refuses exactly what the API does and answers with the API's
own JSON.

The read tools are always offered: project_status, list_standards (with each
standard's evidence tier and parameter units), get_model, transform, and the
run tools list_runs, get_run (polled for progress), get_run_log,
get_run_results (the receiver table; one over 256 KiB as a link), get_contours
and estimate_run, which prices a run without starting it. So are the resources
aconiq://project, aconiq://standards, aconiq://runs/{id},
aconiq://runs/{id}/provenance (the run's provenance.json, as recorded) and
aconiq://artifacts/{id}.

Without --allow-writes the server is read-only and lists no tool that changes
the project. With it, start_run and cancel_run are offered, and save_model: it
replaces the whole model, and its confirm argument must be the hash of the
model being replaced (or "none"), so a model that changed since the agent read
it is refused, not overwritten. start_run takes the scenario, standard,
version, profile and receiver mode explicitly, never a default, and computes
the saved model. Every state-changing call is appended to
.noise/logs/mcp-audit.jsonl.

With --allow-writes and --photo-transmission, propose_windows is offered too:
an agent that read windows off a facade image adds them to the model as
proposals, proposed and not sensitive, for a person to confirm in the facade
elevation; it adds only new windows, and takes confirm as save_model does.
Sending a site photo to a hosted model is a disclosure of personal data,
decided per engagement before the first image leaves the machine
(docs/policies/data-handling.md). --photo-transmission records that decision:
it names the engagement's decision record (an opaque reference, never a path),
and is stamped into every proposal's evidence and every propose_windows audit
line. The agent cannot set it.

The server stops when stdin closes or on SIGINT/SIGTERM, and stops the runs it
started first.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCP(cmd, allowWrites, photoTransmission)
		},
	}

	cmd.Flags().BoolVar(&allowWrites, "allow-writes", false,
		"Offer the tools that change the project (start_run, cancel_run, save_model); every such call is audited")
	cmd.Flags().StringVar(&photoTransmission, "photo-transmission", "",
		"The engagement's decision that site photos may be sent to the model reading them; with --allow-writes, offers propose_windows")

	return cmd
}

func runMCP(cmd *cobra.Command, allowWrites bool, photoTransmission string) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "command state unavailable", nil)
	}

	if cmd.Flags().Changed("photo-transmission") {
		if !allowWrites {
			return domainerrors.New(domainerrors.KindUserInput, "cli.mcp",
				"--photo-transmission offers propose_windows, which writes the model; it needs --allow-writes", nil)
		}

		err := facadeset.ValidateTransmissionBasis(photoTransmission)
		if err != nil {
			return domainerrors.New(domainerrors.KindUserInput, "cli.mcp", "--photo-transmission", err)
		}
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "build standards registry", err)
	}

	// The same sweep `aconiq serve` makes, for the same reason: a server that is
	// starting has started nothing, so a run still marked running belongs to a
	// process that is gone.
	interrupted, err := store.FailInterruptedRuns(nowUTC, nil)
	if err != nil {
		return fmt.Errorf("reconcile interrupted runs: %w", err)
	}

	if interrupted > 0 {
		state.Logger.Warn("closed interrupted runs", "count", interrupted)
	}

	handler := httpv1.NewServeHandler(store, nowUTC, registry, httpv1.ServeOptions{Logger: state.Logger})

	server, err := mcpserver.New(handler, mcpserver.Options{
		AllowWrites:       allowWrites,
		Version:           buildinfo.Version(),
		AuditPath:         store.MCPAuditLogPath(),
		Logger:            state.Logger,
		PhotoTransmission: photoTransmission,
	})
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "build MCP server", err)
	}

	state.Logger.Info("mcp started", "project", store.Root(), "allow_writes", allowWrites,
		"photo_transmission", photoTransmission != "")

	// cobra's streams rather than mcp.StdioTransport, which binds os.Stdin and
	// os.Stdout itself: they are the same files in production, and a test can
	// hand the command its own.
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(cmd.InOrStdin()),
		Writer: nopWriteCloser{cmd.OutOrStdout()},
	}

	// The root context ends on SIGINT/SIGTERM, and stdin reaching EOF ends the
	// session. Either way the runs this server started are stopped before the
	// process exits, as `aconiq serve` stops its own.
	runErr := server.Run(cmd.Context(), transport)

	stopServeRuns(handler, state)

	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, io.EOF) {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "serve MCP session", runErr)
	}

	state.Logger.Info("mcp stopped")

	return nil
}

// nopWriteCloser keeps the session from closing the process's stdout when it
// ends.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
