package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// FromFlags derives both paths from the working directory when no flags are
// given, so it has to survive that directory being gone. A removed working
// directory is not exotic: it is what a shell left in a `git clean`ed or
// rebuilt tree sits in, and the failure mode to avoid is FromFlags returning a
// silently empty or relative ProjectPath that later writes `.noise/` somewhere
// unexpected.
func TestFromFlagsReportsAMissingWorkingDirectory(t *testing.T) {
	// Not parallel: it changes the process working directory.
	gone := filepath.Join(t.TempDir(), "gone")

	err := os.Mkdir(gone, 0o700)
	if err != nil {
		t.Fatalf("create working directory: %v", err)
	}

	t.Chdir(gone)

	err = os.Remove(gone)
	if err != nil {
		t.Fatalf("remove working directory: %v", err)
	}

	{
		_, err := os.Getwd()
		if err == nil {
			t.Skip("this platform still resolves a removed working directory")
		}
	}

	cfg, err := config.FromFlags("", "", false, false)
	if err == nil {
		t.Fatalf("expected an error with no working directory, got %#v", cfg)
	}

	if cfg != (config.Config{}) {
		t.Fatalf("expected the zero Config alongside the error, got %#v", cfg)
	}

	if !strings.Contains(err.Error(), "working directory") {
		t.Fatalf("error %q does not name the working directory", err)
	}
}

// An explicit --project keeps working even then: the path needs no lookup.
func TestFromFlagsWithExplicitAbsolutePathSurvivesAMissingWorkingDirectory(t *testing.T) {
	// Not parallel: it changes the process working directory.
	project := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")

	err := os.Mkdir(gone, 0o700)
	if err != nil {
		t.Fatalf("create working directory: %v", err)
	}

	t.Chdir(gone)

	err = os.Remove(gone)
	if err != nil {
		t.Fatalf("remove working directory: %v", err)
	}

	cfg, err := config.FromFlags(project, "", false, false)
	if err != nil {
		t.Fatalf("from flags with an absolute project path: %v", err)
	}

	if cfg.ProjectPath != project {
		t.Fatalf("ProjectPath = %q, want %q", cfg.ProjectPath, project)
	}
}

// A relative --project or --cache-dir can only be resolved against the working
// directory. When that is gone, each must name itself in the error rather than
// leaving the caller to guess which of the two flags was at fault.
func TestFromFlagsReportsWhichRelativePathCouldNotBeResolved(t *testing.T) {
	// Not parallel: it changes the process working directory.
	cases := []struct {
		name        string
		projectPath string
		cacheDir    string
		wantInError string
	}{
		{name: "relative project path", projectPath: "site", cacheDir: "", wantInError: `"site"`},
		{name: "relative cache dir", projectPath: "", cacheDir: "scratch", wantInError: `"scratch"`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectPath := testCase.projectPath
			if projectPath == "" {
				// Keep the project path resolvable so the cache dir is what fails.
				projectPath = t.TempDir()
			}

			gone := filepath.Join(t.TempDir(), "gone")

			err := os.Mkdir(gone, 0o700)
			if err != nil {
				t.Fatalf("create working directory: %v", err)
			}

			t.Chdir(gone)

			err = os.Remove(gone)
			if err != nil {
				t.Fatalf("remove working directory: %v", err)
			}

			{
				_, err := os.Getwd()
				if err == nil {
					t.Skip("this platform still resolves a removed working directory")
				}
			}

			cfg, err := config.FromFlags(projectPath, testCase.cacheDir, false, false)
			if err == nil {
				t.Fatalf("expected an error, got %#v", cfg)
			}

			if !strings.Contains(err.Error(), testCase.wantInError) {
				t.Fatalf("error %q does not name %s", err, testCase.wantInError)
			}
		})
	}
}
