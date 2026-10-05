package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
)

// Every registered standard is run under a context that is already cancelled.
// Each one must stop in its compute step and leave a run the project can
// account for: recorded as failed, with the reason in its log, and with no
// result payload behind it. A module that ignores the context would complete
// the run instead, which is what this test exists to catch.
func TestEveryStandardStopsOnACancelledContext(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new standards registry: %v", err)
	}

	for _, descriptor := range registry.List() {
		t.Run(descriptor.ID, func(t *testing.T) {
			t.Parallel()

			fixture, ok := registryRunFixtures[descriptor.ID]
			if !ok {
				t.Fatalf("standard %q is registered but declares no run fixture", descriptor.ID)
			}

			projectDir := t.TempDir()

			mustRunCLI(t, "--project", projectDir, "init", "--name", "Cancel", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, fixture...))

			args := append([]string{"--project", projectDir, "run", "--standard", descriptor.ID}, tierRunArgs[descriptor.EvidenceTier]...)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := runCLIContext(ctx, args...)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled, got %v", err)
			}

			run := onlyRun(t, projectDir)
			if run.Status != project.RunStatusFailed {
				t.Fatalf("expected status %q, got %q", project.RunStatusFailed, run.Status)
			}

			runDir := filepath.Join(projectDir, ".noise", "runs", run.ID)

			logBytes, err := os.ReadFile(filepath.Join(runDir, "run.log"))
			if err != nil {
				t.Fatalf("read run.log: %v", err)
			}

			if !strings.Contains(string(logBytes), "run cancelled") {
				t.Fatalf("expected run.log to say the run was cancelled:\n%s", logBytes)
			}

			entries, err := os.ReadDir(filepath.Join(runDir, "results"))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("read results dir: %v", err)
			}

			if len(entries) != 0 {
				t.Fatalf("expected no result payload after a cancelled run, found %d entries", len(entries))
			}
		})
	}
}

func runCLIContext(ctx context.Context, args ...string) error {
	cmd := newRootCommand() //nolint:contextcheck // the context reaches the command through ExecuteContext, as in Execute
	cmd.SetArgs(args)

	return cmd.ExecuteContext(ctx)
}

func onlyRun(t *testing.T, projectDir string) project.Run {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) != 1 {
		t.Fatalf("expected exactly one run, got %d", len(proj.Runs))
	}

	return proj.Runs[0]
}
