package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// A run the inputs refuse — here, a project with no model in it — is refused
// before its row exists, as a user error. `aconiq serve` answers once the row
// exists, so a refusal made after it reached the API as a 202 for a run that
// was already failed, with the reason only in run.log.
func TestRunRefusedOnItsInputsLeavesNoRow(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Refused", "--crs", "EPSG:25832")

	err := runCLI("--project", projectDir, "run", "--standard", "rls19-road")

	if !domainerrors.IsUserError(err) {
		t.Fatalf("expected a user error for a project without a model, got %v", err)
	}

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(proj.Runs) != 0 {
		t.Fatalf("a run refused on its inputs left %d rows", len(proj.Runs))
	}

	entries, err := os.ReadDir(filepath.Join(projectDir, ".noise", "runs"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read runs dir: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("a run refused on its inputs left %d run directories", len(entries))
	}
}

// A module's own refusal — a receiver mode the standard does not support —
// comes after the row exists, so it is recorded: failed, with the reason as
// the last thing the log says before "run failed". It used to leave the row
// reading `running`.
func TestModuleRefusalIsRecordedAsAFailedRunWithItsReason(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Module Refusal", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, registryRunFixtures["beb-exposure"]...))

	err := runCLI("--project", projectDir, "run", "--standard", "beb-exposure", "--receiver-mode", "custom")
	if !domainerrors.IsUserError(err) {
		t.Fatalf("expected the module's user-input refusal, got %v", err)
	}

	run := onlyRun(t, projectDir)
	if run.Status != project.RunStatusFailed {
		t.Fatalf("status = %q, want %q", run.Status, project.RunStatusFailed)
	}

	logBytes, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.LogPath)))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}

	if !strings.Contains(string(logBytes), "run refused: ") || !strings.Contains(string(logBytes), "custom receiver mode") {
		t.Fatalf("run log does not say why the run was refused:\n%s", logBytes)
	}
}
