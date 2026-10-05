package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

func buildRunArtifacts(projectRoot string, runID string, persisted persistedRunOutputs) []project.ArtifactRef {
	now := nowUTC()

	artifacts := make([]project.ArtifactRef, 0, 5)
	if persisted.ReceiverJSONPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-json", runID), RunID: runID, Kind: project.ArtifactKindRunResultReceiverTableJSON, Path: relativePath(projectRoot, persisted.ReceiverJSONPath), CreatedAt: now})
	}

	if persisted.ReceiverCSVPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-csv", runID), RunID: runID, Kind: project.ArtifactKindRunResultReceiverTableCSV, Path: relativePath(projectRoot, persisted.ReceiverCSVPath), CreatedAt: now})
	}

	if persisted.RasterMetadataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-meta", runID), RunID: runID, Kind: project.ArtifactKindRunResultRasterMetadata, Path: relativePath(projectRoot, persisted.RasterMetadataPath), CreatedAt: now})
	}

	if persisted.RasterDataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-data", runID), RunID: runID, Kind: project.ArtifactKindRunResultRasterBinary, Path: relativePath(projectRoot, persisted.RasterDataPath), CreatedAt: now})
	}

	if persisted.SummaryPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-summary", runID), RunID: runID, Kind: project.ArtifactKindRunResultSummary, Path: relativePath(projectRoot, persisted.SummaryPath), CreatedAt: now})
	}

	return artifacts
}

func finalizeRunFailure(store projectfs.Store, run project.Run, logLines []string, runErr error) error {
	finishedAt := nowUTC()

	logLines = append(logLines, finishedAt.Format(time.RFC3339)+" run failed")

	err := finalizeRun(store, run, project.RunStatusFailed, finishedAt, logLines, nil)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRunFailure", "finalize failed run", errors.Join(runErr, err))
	}

	return runErr
}

func finalizeRun(
	store projectfs.Store,
	run project.Run,
	status string,
	finishedAt time.Time,
	logLines []string,
	artifacts []project.ArtifactRef,
) error {
	if finishedAt.IsZero() {
		finishedAt = nowUTC()
	}

	proj, err := store.Load()
	if err != nil {
		return fmt.Errorf("load project manifest: %w", err)
	}

	foundRun := false

	for i := range proj.Runs {
		if proj.Runs[i].ID != run.ID {
			continue
		}

		proj.Runs[i].Status = status
		proj.Runs[i].FinishedAt = finishedAt
		foundRun = true

		break
	}

	if !foundRun {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", fmt.Sprintf("run %s not found in project manifest", run.ID), nil)
	}

	for _, artifact := range artifacts {
		proj.Artifacts = upsertArtifact(proj.Artifacts, artifact)
	}

	err = store.Save(proj)
	if err != nil {
		return fmt.Errorf("save project manifest: %w", err)
	}

	if len(logLines) == 0 {
		logLines = []string{fmt.Sprintf("%s run finalized with status=%s", finishedAt.Format(time.RFC3339), status)}
	}

	logContent := strings.Join(logLines, "\n") + "\n"

	logPath := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))

	// The manifest above already says the run is done, so the UI may fetch
	// the log while this replaces it. A rename hands it the provisional log or
	// the final one; an in-place rewrite could hand it a truncated prefix.
	err = atomicfile.WriteFile(logPath, []byte(logContent))
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", "write run log "+logPath, err)
	}

	return nil
}
