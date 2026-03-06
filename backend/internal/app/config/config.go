package config

import (
	"log/slog"
	"os"
	"path/filepath"
)

// Config contains process-wide runtime settings resolved from CLI flags.
type Config struct {
	ProjectPath string
	CacheDir    string
	LogLevel    slog.Level
	JSONLogs    bool
}

// FromFlags resolves runtime configuration from the root CLI flags.
func FromFlags(projectPath string, cacheDir string, verbose bool, jsonLogs bool) (Config, error) {
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, err
		}

		projectPath = cwd
	}

	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return Config{}, err
	}

	if cacheDir == "" {
		cacheDir = filepath.Join(absProjectPath, ".noise", "cache")
	}

	absCacheDir, err := filepath.Abs(cacheDir)
	if err != nil {
		return Config{}, err
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	return Config{
		ProjectPath: absProjectPath,
		CacheDir:    absCacheDir,
		LogLevel:    level,
		JSONLogs:    jsonLogs,
	}, nil
}
