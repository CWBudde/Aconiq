// Package fixtures locates the licensed third-party test data that some
// packages need and a fresh clone does not have.
//
// The data lives outside the repository on purpose: interoperability/ holds
// gigabytes of third-party project files that must never be committed. Tests
// that need it obtain the path here so that a missing fixture skips a test
// rather than failing it, and so that no single test hardcodes where the data
// sits or what it is called.
package fixtures

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// SoundPLANFixturesEnv names the environment variable that points at the
// SoundPLAN project directory used by the import and comparison tests.
//
// It takes precedence over discovery, which is what lets CI mount the data
// anywhere it likes, and what lets a developer with several SoundPLAN projects
// say which one to run against.
const SoundPLANFixturesEnv = "ACONIQ_SOUNDPLAN_FIXTURES"

// soundPLANProjectMarker is the file every SoundPLAN project directory
// contains. Discovery keys on it rather than on a directory name so that the
// name of a third-party project never has to appear in tracked source.
const soundPLANProjectMarker = "Project.sp"

// SoundPLANProjectDir returns the directory of the SoundPLAN project the
// interoperability tests read.
//
// A configured but wrong SoundPLANFixturesEnv is a hard failure: someone said
// where the data is and was mistaken, and silently skipping would hide that.
// An absent fixture skips, because a clean checkout legitimately has none.
func SoundPLANProjectDir(t *testing.T) string {
	t.Helper()

	configured, ok := os.LookupEnv(SoundPLANFixturesEnv)
	if ok && configured != "" {
		err := checkSoundPLANProject(configured)
		if err != nil {
			t.Fatalf("%s is set to %q but does not name a SoundPLAN project: %v", SoundPLANFixturesEnv, configured, err)
		}

		return configured
	}

	dir, err := discoverSoundPLANProject()
	if err != nil {
		t.Skipf("SoundPLAN project fixture not available (%v); set %s to the project directory, "+
			"or place the project under the repository-root 'interoperability/' directory, to enable this test",
			err, SoundPLANFixturesEnv)
	}

	return dir
}

// checkSoundPLANProject reports whether dir looks like a SoundPLAN project.
func checkSoundPLANProject(dir string) error {
	info, err := os.Stat(filepath.Join(dir, soundPLANProjectMarker))
	if err != nil {
		return fmt.Errorf("stat %s: %w", soundPLANProjectMarker, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", soundPLANProjectMarker)
	}

	return nil
}

// discoverSoundPLANProject scans the repository-root interoperability/
// directory for exactly one SoundPLAN project.
func discoverSoundPLANProject() (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}

	interopDir := filepath.Join(root, "interoperability")

	entries, err := os.ReadDir(interopDir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", interopDir, err)
	}

	candidates := make([]string, 0, 1)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		candidate := filepath.Join(interopDir, entry.Name())
		if checkSoundPLANProject(candidate) == nil {
			candidates = append(candidates, candidate)
		}
	}

	sort.Strings(candidates)

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no directory under %s contains a %s", interopDir, soundPLANProjectMarker)
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("%d SoundPLAN projects under %s, so the choice is ambiguous", len(candidates), interopDir)
	}
}

// repoRoot resolves the repository root from this file's compile-time path,
// which is stable regardless of the working directory a test runs in.
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot resolve the caller path")
	}

	// backend/internal/qa/fixtures/soundplan.go -> repository root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..")), nil
}
