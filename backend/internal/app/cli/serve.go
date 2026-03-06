package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/soundplan/soundplan/backend/internal/api/httpv1"
	domainerrors "github.com/soundplan/soundplan/backend/internal/domain/errors"
	"github.com/soundplan/soundplan/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	var listenAddr string
	var shutdownTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start local API server (Phase 23 initial slice)",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.serve", "command state unavailable", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			handler := httpv1.NewHandler(store, nowUTC)
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

			state.Logger.Info("serve started", "address", listenAddr, "project", store.Root())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Serving local API on http://%s\n", listenAddr)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Health: http://%s/api/v1/health\n", listenAddr)

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return domainerrors.New(domainerrors.KindInternal, "cli.serve", fmt.Sprintf("listen on %s", listenAddr), err)
			case <-runCtx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
				defer cancel()
				if err := server.Shutdown(shutdownCtx); err != nil {
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

	return cmd
}
