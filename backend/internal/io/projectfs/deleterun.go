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

// exportArtifactKindPrefix marks an artifact as belonging to an export bundle.
//
// The kind is the classification, never the path: `aconiq export --out` writes
// bundles wherever it is pointed, so testing for the default directory would
// recognise only the bundles that happened to land there and drop the rest
// without a word.
const exportArtifactKindPrefix = "export."

// The refusals DeleteRun makes about a named run. They are sentinels rather
// than error kinds because a caller has to answer them differently — the HTTP
// API owes a 404 for one and a 409 for the other, each with its own code — and
// the error taxonomy classifies for exit codes, which cannot carry that.
var (
	// ErrRunNotFound reports that no run in the manifest has the requested ID.
	ErrRunNotFound = errors.New("run does not exist")
	// ErrRunNotFinished reports a run that is still pending or running. Its
	// directory is being written by a live `aconiq run`, so it is left alone.
	ErrRunNotFinished = errors.New("run has not finished")
	// ErrExportInsideRun reports an export bundle written inside the run's own
	// directory, where removing the directory would take the bundle with it.
	ErrExportInsideRun = errors.New("export bundle lies inside the run directory")
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
// Export bundles are deliberately kept, wherever `aconiq export --out` put
// them. Their refs are dropped with the rest, but a bundle may already have been
// handed to someone, and deleting a Gutachten as a side effect of "delete run"
// is not what the word means. A bundle written inside the run's own directory
// cannot be kept and removed at once, so that delete is refused outright.
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

	err = refuseExportsInsideRun(op, runID, result.RetainedPaths)
	if err != nil {
		return DeleteRunResult{}, err
	}

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
		result.RemovedPaths = append(result.RemovedPaths, runDirPath(runID))
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

		if strings.HasPrefix(ref.Kind, exportArtifactKindPrefix) {
			retained = append(retained, ref.Path)
		}
	}

	slices.Sort(retained)

	return kept, slices.Compact(retained)
}

// runDirPath names a run's directory the way the manifest and the delete result
// spell paths: project-relative, forward slashes, no trailing separator.
func runDirPath(runID string) string {
	return ".noise/runs/" + runID
}

// refuseExportsInsideRun stops a delete that would destroy the bundles it
// promises to keep. `aconiq export --out` accepts any directory, including one
// under the run's own, and removeRunDirectory cannot spare a file inside the
// tree it removes. Refusing names the bundle and leaves the run intact, which
// beats reporting a path as retained while deleting it.
func refuseExportsInsideRun(op string, runID string, retained []string) error {
	prefix := runDirPath(runID) + "/"

	inside := make([]string, 0, len(retained))

	for _, path := range retained {
		if strings.HasPrefix(path, prefix) {
			inside = append(inside, path)
		}
	}

	if len(inside) == 0 {
		return nil
	}

	return domainerrors.New(
		domainerrors.KindUserInput, op,
		fmt.Sprintf(
			"run %q cannot be deleted: it holds export bundles inside its own directory (%s)",
			runID, strings.Join(inside, ", "),
		),
		ErrExportInsideRun,
	)
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
