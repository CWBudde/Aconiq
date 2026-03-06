package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	var listenAddr string
	var shutdownTimeout time.Duration
	var corsOrigins []string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start local API server",
		Long: `Start a local HTTP API server for use with the Aconiq frontend or integrations.

CORS is enabled by default for localhost and 127.0.0.1 origins (any port),
covering the Vite dev server (typically :5173) and other local tooling.
Use --cors-origins to allow additional origins, or leave it empty for local-only use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.serve", "command state unavailable", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			registry, err := standards.NewRegistry()
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.serve", "build standards registry", err)
			}

			handler := httpv1.NewServeHandler(store, nowUTC, registry, corsOrigins)
			server := &http.Server{
				Addr:         listenAddr,
				Handler:      handler,
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

			state.Logger.Info("serve started", "address", listenAddr, "project", store.Root(), "cors_origins", strings.Join(corsOrigins, ","))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Serving local API on http://%s\n", listenAddr)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Health: http://%s/api/v1/health\n", listenAddr)

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}

				return domainerrors.New(domainerrors.KindInternal, "cli.serve", "listen on "+listenAddr, err)
			case <-runCtx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
				defer cancel()

				err = server.Shutdown(shutdownCtx)
				if err != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.serve", "graceful shutdown", err)
				}

				err := <-errCh
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return domainerrors.New(domainerrors.KindInternal, "cli.serve", "server stop", err)
				}

				state.Logger.Info("serve stopped")

				return nil
			}
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:8080", "Address for local API server")
	cmd.Flags().DurationVar(&shutdownTimeout, "shutdown-timeout", 5*time.Second, "Graceful shutdown timeout")
	cmd.Flags().StringArrayVar(&corsOrigins, "cors-origins", nil,
		"Additional allowed CORS origins (localhost/127.0.0.1 are always allowed). Example: https://myapp.example.com")

	return cmd
}
