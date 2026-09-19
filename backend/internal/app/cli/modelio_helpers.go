package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/jsonio"
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
	encoded, err := jsonio.Marshal(value)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", "encode "+path, err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.writeJSONFile", "create directory for "+path, err)
	}

	// Atomic, so a crash part-way through leaves the previous artifact rather
	// than a truncated one. This used to be a bare os.WriteFile while the
	// projectfs twin renamed a temporary file into place, which meant the same
	// model files were replaced atomically through the HTTP API and
	// non-atomically through the CLI.
	//
	// The G703 suppression the direct write carried is gone with it: the taint
	// followed the CLI's --project / --out flags into os.WriteFile, and callers
	// build path from the project root plus a fixed artifact name.
	err = atomicfile.WriteFile(path, encoded)
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
