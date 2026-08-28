package fixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSoundPLANProjectAcceptsAProjectDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, soundPLANProjectMarker), []byte("[Project]\n"), 0o600)
	if err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if err := checkSoundPLANProject(dir); err != nil {
		t.Errorf("expected a directory holding %s to be accepted, got %v", soundPLANProjectMarker, err)
	}
}

func TestCheckSoundPLANProjectRejectsWhatIsNotAProject(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	if err := checkSoundPLANProject(empty); err == nil {
		t.Error("expected a directory without a project marker to be rejected")
	}

	markerIsDir := t.TempDir()

	err := os.Mkdir(filepath.Join(markerIsDir, soundPLANProjectMarker), 0o750)
	if err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}

	if err := checkSoundPLANProject(markerIsDir); err == nil {
		t.Errorf("expected a directory whose %s is itself a directory to be rejected", soundPLANProjectMarker)
	}
}

// A configured fixture path takes precedence over discovery, so CI can mount
// the data anywhere.
func TestSoundPLANProjectDirHonoursTheConfiguredPath(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, soundPLANProjectMarker), []byte("[Project]\n"), 0o600)
	if err != nil {
		t.Fatalf("write marker: %v", err)
	}

	t.Setenv(SoundPLANFixturesEnv, dir)

	if got := SoundPLANProjectDir(t); got != dir {
		t.Errorf("expected the configured path %q, got %q", dir, got)
	}
}
