package projectfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

// runIDPattern is what a run identifier may look like. Run IDs are generated
// by buildID, but a delete request names one, so the name is validated before
// it is joined to a path rather than trusted because of where it came from.
var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// exportPathPrefix is the part of the project tree a run delete leaves alone.
const exportPathPrefix = ".noise/exports/"

// The two refusals DeleteRun makes about a named run. They are sentinels rather
// than error kinds because a caller has to answer them differently — the HTTP
// API owes a 404 for one and a 409 for the other, each with its own code — and
// the error taxonomy classifies for exit codes, which cannot carry that.
var (
	// ErrRunNotFound reports that no run in the manifest has the requested ID.
	ErrRunNotFound = errors.New("run does not exist")
	// ErrRunNotFinished reports a run that is still pending or running. Its
	// directory is being written by a live `aconiq run`, so it is left alone.
	ErrRunNotFinished = errors.New("run has not finished")
)

// DeleteRunResult reports what a delete did, so the caller can tell the user
// what is gone and what is not.
type DeleteRunResult struct {
	// RunID is the run that was removed.
	RunID string
	// RemovedPaths lists the project-relative paths that were deleted.
	RemovedPaths []string
	// RetainedPaths lists files whose manifest refs were dropped but whose bytes
	// were deliberately left on disk — export bundles.
	RetainedPaths []string
}

// DeleteRun removes a run from the project: its manifest entry, every artifact
// ref that belongs to it, and its `.noise/runs/<id>/` directory.
//
// Export bundles under .noise/exports/ are deliberately kept. Their refs are
// dropped with the rest, but a bundle may already have been handed to someone,
// and deleting a Gutachten as a side effect of "delete run" is not what the
// word means.
//
// A run that is still pending or running is refused: the `aconiq run`
// subprocess is still writing into that directory, and removing it underneath
// a live writer trades a clean refusal for a partial one.
func (s Store) DeleteRun(runID string) (DeleteRunResult, error) {
	const op = "projectfs.DeleteRun"

	err := validateRunID(op, runID)
	if err != nil {
		return DeleteRunResult{}, err
	}

	proj, err := s.Load()
	if err != nil {
		return DeleteRunResult{}, err
	}

	index := slices.IndexFunc(proj.Runs, func(r project.Run) bool { return r.ID == runID })
	if index < 0 {
		return DeleteRunResult{}, domainerrors.New(
			domainerrors.KindNotFound, op,
			fmt.Sprintf("run %q does not exist", runID),
			ErrRunNotFound,
		)
	}

	run := proj.Runs[index]
	if run.Status == project.RunStatusPending || run.Status == project.RunStatusRunning {
		return DeleteRunResult{}, domainerrors.New(
			domainerrors.KindUserInput, op,
			fmt.Sprintf("run %q is still %s; it cannot be deleted while it is being written", runID, run.Status),
			ErrRunNotFinished,
		)
	}

	result := DeleteRunResult{
		RunID:         runID,
		RemovedPaths:  []string{},
		RetainedPaths: []string{},
	}

	proj.Runs = slices.Delete(proj.Runs, index, index+1)
	proj.Artifacts, result.RetainedPaths = dropRunArtifacts(proj.Artifacts, runID)

	// The manifest goes first, on purpose. If the files went first and this save
	// then failed, the manifest would point at bytes that are no longer there.
	// This way a failed removal leaves orphan bytes under a consistent project,
	// and running the delete again is harmless.
	err = s.Save(proj)
	if err != nil {
		return DeleteRunResult{}, err
	}

	removed, err := s.removeRunDirectory(op, runID)
	if err != nil {
		return DeleteRunResult{}, err
	}

	if removed {
		result.RemovedPaths = append(result.RemovedPaths, ".noise/runs/"+runID)
	}

	return result, nil
}

// validateRunID keeps an identifier that arrives from outside — a URL path
// segment, a CLI flag — from becoming a path anywhere but under the runs
// directory. filepath.IsLocal rejects absolute paths and ".." escapes; the
// pattern additionally rules out separators and leading dashes.
func validateRunID(op string, runID string) error {
	if runID == "" {
		return domainerrors.New(domainerrors.KindUserInput, op, "run id is required", nil)
	}

	if !runIDPattern.MatchString(runID) || !filepath.IsLocal(runID) {
		return domainerrors.New(domainerrors.KindUserInput, op, "run id must match "+runIDPattern.String(), nil)
	}

	return nil
}

// dropRunArtifacts removes every artifact ref belonging to runID and reports
// the export bundle paths among them, whose bytes stay on disk.
func dropRunArtifacts(artifacts []project.ArtifactRef, runID string) ([]project.ArtifactRef, []string) {
	kept := make([]project.ArtifactRef, 0, len(artifacts))
	retained := []string{}

	for _, ref := range artifacts {
		if ref.RunID != runID {
			kept = append(kept, ref)
			continue
		}

		if strings.HasPrefix(ref.Path, exportPathPrefix) {
			retained = append(retained, ref.Path)
		}
	}

	slices.Sort(retained)

	return kept, slices.Compact(retained)
}

// removeRunDirectory deletes .noise/runs/<runID>. The removal is rooted at the
// runs directory with os.OpenRoot, so the operation cannot leave it even if the
// identifier check above were ever loosened: containment is structural, not a
// property of the string.
func (s Store) removeRunDirectory(op string, runID string) (bool, error) {
	root, err := os.OpenRoot(s.runsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, domainerrors.New(domainerrors.KindInternal, op, "open runs directory", err)
	}

	defer func() { _ = root.Close() }()

	err = root.RemoveAll(runID)
	if err != nil {
		return false, domainerrors.New(domainerrors.KindInternal, op, "remove run directory", err)
	}

	return true, nil
}
