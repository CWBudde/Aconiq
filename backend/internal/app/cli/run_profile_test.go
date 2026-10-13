package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// The tests in this file that start a CPU profile do not call t.Parallel: the
// CPU profiler is process-wide, and a second StartCPUProfile while one is
// running fails. Sequential top-level tests finish before any parallel one
// resumes, so they cannot overlap each other or the rest of the package.

func profiledRunArgs(projectDir string, extra ...string) []string {
	return slices.Concat([]string{
		"--project", projectDir, "run",
		"--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10",
		"--param", "grid_padding_m=0",
		"--param", "source_emission_db=90",
		"--param", "receiver_height_m=4",
	}, extra)
}

func newProfiledProject(t *testing.T) string {
	t.Helper()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Profile", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "phase8", "model.geojson"))

	return projectDir
}

// assertProfileWritten checks that path holds a pprof profile — gzip-compressed
// protobuf, so it starts with the gzip magic — and that no temporary file was
// left beside it.
func assertProfileWritten(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}

	if !bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		t.Fatalf("%s is %d bytes and not a gzip-compressed pprof profile", filepath.Base(path), len(data))
	}

	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	if len(leftovers) != 0 {
		t.Fatalf("temporary profile files left behind: %v", leftovers)
	}
}

func TestRunWritesCPUAndHeapProfiles(t *testing.T) {
	projectDir := newProfiledProject(t)
	profileDir := t.TempDir()
	cpuPath := filepath.Join(profileDir, "cpu.pprof")
	memPath := filepath.Join(profileDir, "mem.pprof")

	mustRunCLI(t, profiledRunArgs(projectDir, "--cpuprofile", cpuPath, "--memprofile", memPath)...)

	assertProfileWritten(t, cpuPath)
	assertProfileWritten(t, memPath)

	if run := onlyRun(t, projectDir); run.Status != project.RunStatusCompleted {
		t.Fatalf("expected status %q, got %q", project.RunStatusCompleted, run.Status)
	}
}

// A cancelled run is the case the profile is most wanted for — a scene too
// slow to wait for — so it must be written on that path too, and the run's own
// error must still be what the command returns.
func TestRunWritesProfilesWhenCancelled(t *testing.T) {
	projectDir := newProfiledProject(t)
	profileDir := t.TempDir()
	cpuPath := filepath.Join(profileDir, "cpu.pprof")
	memPath := filepath.Join(profileDir, "mem.pprof")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := runCLIContext(ctx, profiledRunArgs(projectDir, "--cpuprofile", cpuPath, "--memprofile", memPath)...)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	assertProfileWritten(t, cpuPath)
	assertProfileWritten(t, memPath)

	if run := onlyRun(t, projectDir); run.Status != project.RunStatusFailed {
		t.Fatalf("expected status %q, got %q", project.RunStatusFailed, run.Status)
	}
}

// A run refused before it starts still writes its profile, and the refusal
// keeps its user-error classification, which is what the exit code follows.
func TestRunWritesProfilesWhenRefused(t *testing.T) {
	projectDir := newProfiledProject(t)
	cpuPath := filepath.Join(t.TempDir(), "cpu.pprof")

	err := runCLI(profiledRunArgs(projectDir, "--receiver-mode", "nonsense", "--cpuprofile", cpuPath)...)
	if err == nil {
		t.Fatal("expected the run to be refused")
	}

	if !domainerrors.IsUserError(err) {
		t.Fatalf("expected a user error, got %v", err)
	}

	assertProfileWritten(t, cpuPath)
}

// An unwritable profile path is refused before the run exists, rather than
// discovered after it has finished.
func TestRunRefusesAnUnwritableProfilePathBeforeRunning(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "does-not-exist")

	for _, flag := range []string{"--cpuprofile", "--memprofile"} {
		projectDir := newProfiledProject(t)

		err := runCLI(profiledRunArgs(projectDir, flag, filepath.Join(missingDir, "p.pprof"))...)
		if !domainerrors.IsUserError(err) {
			t.Fatalf("%s: expected a user error, got %v", flag, err)
		}

		store, err := projectfs.New(projectDir)
		if err != nil {
			t.Fatalf("open project: %v", err)
		}

		proj, err := store.Load()
		if err != nil {
			t.Fatalf("load project: %v", err)
		}

		if len(proj.Runs) != 0 {
			t.Fatalf("%s: expected no run to be created, got %d", flag, len(proj.Runs))
		}
	}
}

func TestRunRefusesOneFileForBothProfiles(t *testing.T) {
	t.Parallel()

	projectDir := newProfiledProject(t)
	path := filepath.Join(t.TempDir(), "both.pprof")

	err := runCLI(profiledRunArgs(projectDir, "--cpuprofile", path, "--memprofile", path)...)
	if !domainerrors.IsUserError(err) {
		t.Fatalf("expected a user error, got %v", err)
	}

	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no profile file, stat returned %v", statErr)
	}
}
