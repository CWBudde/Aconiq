package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aconiq/backend/internal/api/httpv1"
	"github.com/aconiq/backend/internal/buildinfo"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/mcpserver"
	"github.com/aconiq/backend/internal/standards"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
	var allowWrites bool

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

Without --allow-writes the server is read-only and lists no tool that changes
the project. With it, save_model is offered: it replaces the whole model, and
its confirm argument must be the hash of the model being replaced (or "none"),
so a model that changed since the agent read it is refused, not overwritten.
Every state-changing call is appended to .noise/logs/mcp-audit.jsonl.

The server stops when stdin closes or on SIGINT/SIGTERM.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCP(cmd, allowWrites)
		},
	}

	cmd.Flags().BoolVar(&allowWrites, "allow-writes", false,
		"Offer the tools that change the project (save_model); every such call is audited")

	return cmd
}

func runMCP(cmd *cobra.Command, allowWrites bool) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "command state unavailable", nil)
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
		AllowWrites: allowWrites,
		Version:     buildinfo.Version(),
		AuditPath:   store.MCPAuditLogPath(),
		Logger:      state.Logger,
	})
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.mcp", "build MCP server", err)
	}

	state.Logger.Info("mcp started", "project", store.Root(), "allow_writes", allowWrites)

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
