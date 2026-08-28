package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aconiq/backend/internal/api/httpv1"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	var (
		listenAddr      string
		shutdownTimeout time.Duration
		corsOrigins     []string
		apiToken        string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start local API server",
		Long: `Start a local HTTP API server for use with the Aconiq frontend or integrations.

CORS is enabled by default for localhost and 127.0.0.1 origins (any port),
covering the Vite dev server (typically :5173) and other local tooling.
Use --cors-origins to allow additional origins, or leave it empty for local-only use.

The server accepts a request only if its Host header names a loopback address or
the host part of --listen. That is what closes DNS rebinding, which an origin
check cannot see. A wildcard bind (--listen 0.0.0.0:8080) names no host, so only
loopback stays reachable; bind the address you want to serve instead.

State-changing requests must also carry a non-empty X-Aconiq-Client header and
send their body as application/json (multipart/form-data for a terrain upload).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, serveConfig{
				listenAddr:      listenAddr,
				shutdownTimeout: shutdownTimeout,
				corsOrigins:     corsOrigins,
				apiToken:        resolveAPIToken(apiToken),
			})
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:8080", "Address for local API server")
	cmd.Flags().DurationVar(&shutdownTimeout, "shutdown-timeout", 5*time.Second, "Graceful shutdown timeout")
	cmd.Flags().StringArrayVar(&corsOrigins, "cors-origins", nil,
		"Additional allowed CORS origins (localhost/127.0.0.1 are always allowed). Example: https://myapp.example.com")
	cmd.Flags().StringVar(&apiToken, "api-token", "",
		"Require this bearer token on every API request (env: "+apiAuthEnvVar+"). "+
			"Off by default: the Host allowlist, the media-type check and the required "+
			httpv1.ClientHeaderName+" header already close the browser-borne vectors. "+
			"Turn it on when other processes on this machine are not trusted.")

	return cmd
}

// apiAuthEnvVar names the environment variable that carries the same value as
// --api-token, so a token need not appear in the process list. It holds the
// variable's name, not a credential.
const apiAuthEnvVar = "ACONIQ_API_TOKEN"

func resolveAPIToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	return strings.TrimSpace(os.Getenv(apiAuthEnvVar))
}

// serveConfig is the flag set newServeCommand resolves, handed to runServe so
// the command builder stays a description of its flags.
type serveConfig struct {
	listenAddr      string
	shutdownTimeout time.Duration
	corsOrigins     []string
	apiToken        string
}

func runServe(cmd *cobra.Command, cfg serveConfig) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.serve", "command state unavailable", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.serve", "build standards registry", err)
	}

	server := &http.Server{
		Addr: cfg.listenAddr,
		Handler: httpv1.NewServeHandler(store, nowUTC, registry, httpv1.ServeOptions{
			CORSOrigins: cfg.corsOrigins,
			ListenAddr:  cfg.listenAddr,
			APIToken:    cfg.apiToken,
		}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	runCtx, stopSignals := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ListenAndServe()
	}()

	state.Logger.Info(
		"serve started",
		"address", cfg.listenAddr,
		"project", store.Root(),
		"cors_origins", strings.Join(cfg.corsOrigins, ","),
		"api_token_required", cfg.apiToken != "",
	)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Serving local API on http://%s\n", cfg.listenAddr)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Health: http://%s/api/v1/health\n", cfg.listenAddr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return domainerrors.New(domainerrors.KindInternal, "cli.serve", "listen on "+cfg.listenAddr, err)
	case <-runCtx.Done():
		return shutdownServer(server, errCh, cfg.shutdownTimeout, state)
	}
}

func shutdownServer(server *http.Server, errCh <-chan error, timeout time.Duration, state commandState) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.serve", "graceful shutdown", err)
	}

	err = <-errCh
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return domainerrors.New(domainerrors.KindInternal, "cli.serve", "server stop", err)
	}

	state.Logger.Info("serve stopped")

	return nil
}
