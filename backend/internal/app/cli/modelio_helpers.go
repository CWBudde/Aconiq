package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

func resolvePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
}

func relativePath(baseDir string, absPath string) string {
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", fmt.Sprintf("encode %s", path), err)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", fmt.Sprintf("create directory for %s", path), err)
	}

	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", fmt.Sprintf("write %s", path), err)
	}

	return nil
}

func upsertArtifact(artifacts []project.ArtifactRef, artifact project.ArtifactRef) []project.ArtifactRef {
	out := make([]project.ArtifactRef, 0, len(artifacts)+1)
	for _, current := range artifacts {
		if current.ID == artifact.ID {
			continue
		}
		out = append(out, current)
	}
	out = append(out, artifact)
	return out
}

func summarizeValidationErrors(errors []string, max int) string {
	if len(errors) == 0 {
		return ""
	}

	if max <= 0 || max > len(errors) {
		max = len(errors)
	}

	parts := make([]string, 0, max)
	for _, msg := range errors[:max] {
		parts = append(parts, msg)
	}

	summary := strings.Join(parts, "; ")
	if len(errors) > max {
		summary += fmt.Sprintf(" (+%d more)", len(errors)-max)
	}
	return summary
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
