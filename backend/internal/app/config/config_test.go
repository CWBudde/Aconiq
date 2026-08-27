package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/app/config"
)

func TestFromFlagsResolvesAbsolutePaths(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cacheDir := t.TempDir()

	cfg, err := config.FromFlags(projectDir, cacheDir, false, false)
	if err != nil {
		t.Fatalf("from flags: %v", err)
	}

	if cfg.ProjectPath != projectDir {
		t.Fatalf("ProjectPath = %q, want %q", cfg.ProjectPath, projectDir)
	}

	if cfg.CacheDir != cacheDir {
		t.Fatalf("CacheDir = %q, want %q", cfg.CacheDir, cacheDir)
	}
}

func TestFromFlagsDefaultsCacheDirBelowProject(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	cfg, err := config.FromFlags(projectDir, "", false, false)
	if err != nil {
		t.Fatalf("from flags: %v", err)
	}

	want := filepath.Join(projectDir, ".noise", "cache")
	if cfg.CacheDir != want {
		t.Fatalf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}
}

func TestFromFlagsDefaultsProjectPathToWorkingDirectory(t *testing.T) {
	// Not parallel: os.Getwd is process-wide state and this test compares
	// against it.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	cfg, err := config.FromFlags("", "", false, false)
	if err != nil {
		t.Fatalf("from flags: %v", err)
	}

	if cfg.ProjectPath != cwd {
		t.Fatalf("ProjectPath = %q, want the working directory %q", cfg.ProjectPath, cwd)
	}

	if cfg.CacheDir != filepath.Join(cwd, ".noise", "cache") {
		t.Fatalf("CacheDir = %q, want it below the working directory", cfg.CacheDir)
	}
}

func TestFromFlagsMakesRelativePathsAbsolute(t *testing.T) {
	// Not parallel: relies on the process working directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	cfg, err := config.FromFlags(filepath.Join("testdata", "project"), filepath.Join("testdata", "cache"), false, false)
	if err != nil {
		t.Fatalf("from flags: %v", err)
	}

	if !filepath.IsAbs(cfg.ProjectPath) || !filepath.IsAbs(cfg.CacheDir) {
		t.Fatalf("expected absolute paths, got %#v", cfg)
	}

	if want := filepath.Join(cwd, "testdata", "project"); cfg.ProjectPath != want {
		t.Fatalf("ProjectPath = %q, want %q", cfg.ProjectPath, want)
	}

	if want := filepath.Join(cwd, "testdata", "cache"); cfg.CacheDir != want {
		t.Fatalf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}
}

func TestFromFlagsCleansPaths(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	messy := filepath.Join(projectDir, "sub", "..", ".", "sub")

	cfg, err := config.FromFlags(messy, "", false, false)
	if err != nil {
		t.Fatalf("from flags: %v", err)
	}

	if want := filepath.Join(projectDir, "sub"); cfg.ProjectPath != want {
		t.Fatalf("ProjectPath = %q, want the cleaned path %q", cfg.ProjectPath, want)
	}
}

// TestFromFlagsDoesNotRequireExistingDirectories keeps `aconiq init` working:
// the project directory is resolved before it is created.
func TestFromFlagsDoesNotRequireExistingDirectories(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist-yet")

	cfg, err := config.FromFlags(missing, "", false, false)
	if err != nil {
		t.Fatalf("from flags on a missing directory: %v", err)
	}

	if cfg.ProjectPath != missing {
		t.Fatalf("ProjectPath = %q, want %q", cfg.ProjectPath, missing)
	}
}

func TestFromFlagsMapsVerboseToDebugLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		verbose bool
		want    slog.Level
	}{
		{verbose: false, want: slog.LevelInfo},
		{verbose: true, want: slog.LevelDebug},
	}

	for _, tc := range cases {
		cfg, err := config.FromFlags(t.TempDir(), "", tc.verbose, false)
		if err != nil {
			t.Fatalf("from flags: %v", err)
		}

		if cfg.LogLevel != tc.want {
			t.Fatalf("LogLevel with verbose=%t is %v, want %v", tc.verbose, cfg.LogLevel, tc.want)
		}
	}
}

func TestFromFlagsPassesThroughJSONLogs(t *testing.T) {
	t.Parallel()

	for _, jsonLogs := range []bool{false, true} {
		cfg, err := config.FromFlags(t.TempDir(), "", false, jsonLogs)
		if err != nil {
			t.Fatalf("from flags: %v", err)
		}

		if cfg.JSONLogs != jsonLogs {
			t.Fatalf("JSONLogs = %t, want %t", cfg.JSONLogs, jsonLogs)
		}
	}
}
