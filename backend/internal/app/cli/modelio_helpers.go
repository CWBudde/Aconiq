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
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", "encode "+path, err)
	}

	encoded = append(encoded, '\n')

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", "create directory for "+path, err)
	}

	// G703 follows the taint from the CLI's --project / --out flags. Callers
	// build path from the project root plus a fixed artifact name, and a CLI
	// writes with the invoking user's own rights; request-supplied paths on the
	// HTTP path are constrained in httpv1.createRunRequest.validate.
	//nolint:gosec // path is project-root derived, not caller-controlled text
	err = os.WriteFile(path, encoded, 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", "write "+path, err)
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

func summarizeValidationErrors(errors []string, limit int) string {
	if len(errors) == 0 {
		return ""
	}

	if limit <= 0 || limit > len(errors) {
		limit = len(errors)
	}

	parts := make([]string, 0, limit)
	parts = append(parts, errors[:limit]...)

	summary := strings.Join(parts, "; ")
	if len(errors) > limit {
		summary += fmt.Sprintf(" (+%d more)", len(errors)-limit)
	}

	return summary
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
