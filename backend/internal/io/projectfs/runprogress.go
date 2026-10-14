package projectfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/jsonio"
)

// runProgressFileName is the file beside run.log that says how far a run's
// compute has got.
const runProgressFileName = "progress.json"

// RunProgress is how far a run's compute has got, as the process computing it
// last wrote it down.
//
// It is a report and not a result. Nothing the run computes reads it, it is
// not an artifact, it is not hashed into anything, and its timestamps are the
// wall clock of whichever machine computed the run — so it lives beside the
// log in `.noise/runs/<id>/` and never under `results/`.
//
// A run whose module reports no progress writes none, and a reader must
// treat the file's absence as "not known" rather than as "nothing done".
type RunProgress struct {
	// Done is how many receivers have been computed, of Total.
	Done  int `json:"done"`
	Total int `json:"total"`
	// StartedAt is when the receiver walk began — after the model was loaded
	// and the scene prepared, so ETASeconds extrapolates the walk's own rate
	// and not the set-up's.
	StartedAt time.Time `json:"started_at"`
	// UpdatedAt is when this report was written.
	UpdatedAt time.Time `json:"updated_at"`
	// ETASeconds is the straight-line extrapolation of the rate so far:
	// elapsed / done * (total - done). Absent until a receiver has completed,
	// since there is no rate to extrapolate before then, and zero once
	// done == total.
	ETASeconds *float64 `json:"eta_seconds,omitempty"`
}

// NewRunProgress fills in the estimate for a report taken at now.
func NewRunProgress(done, total int, startedAt, now time.Time) RunProgress {
	progress := RunProgress{
		Done:      done,
		Total:     total,
		StartedAt: startedAt.UTC(),
		UpdatedAt: now.UTC(),
	}

	if done <= 0 || total <= 0 {
		return progress
	}

	remaining := max(total-done, 0)
	elapsed := now.Sub(startedAt).Seconds()
	eta := max(elapsed/float64(done)*float64(remaining), 0)
	progress.ETASeconds = &eta

	return progress
}

// WriteRunProgress replaces a run's progress report.
//
// Atomically, like every other file a reader may open while it is being
// written: the server reads this file on every poll while the run is still
// writing it, and a torn read would be a JSON error in place of a number.
func (s Store) WriteRunProgress(runID string, progress RunProgress) error {
	err := validateRunID("projectfs.WriteRunProgress", runID)
	if err != nil {
		return err
	}

	payload, err := jsonio.Marshal(progress)
	if err != nil {
		return fmt.Errorf("encode run progress: %w", err)
	}

	err = atomicfile.WriteFile(s.runProgressPath(runID), payload)
	if err != nil {
		return fmt.Errorf("write run progress: %w", err)
	}

	return nil
}

// ReadRunProgress returns a run's last progress report, and false when the
// run has written none.
func (s Store) ReadRunProgress(runID string) (RunProgress, bool, error) {
	err := validateRunID("projectfs.ReadRunProgress", runID)
	if err != nil {
		return RunProgress{}, false, err
	}

	payload, err := os.ReadFile(s.runProgressPath(runID))
	if errors.Is(err, os.ErrNotExist) {
		return RunProgress{}, false, nil
	}

	if err != nil {
		return RunProgress{}, false, fmt.Errorf("read run progress: %w", err)
	}

	var progress RunProgress

	err = json.Unmarshal(payload, &progress)
	if err != nil {
		return RunProgress{}, false, fmt.Errorf("decode run progress: %w", err)
	}

	return progress, true, nil
}

func (s Store) runProgressPath(runID string) string {
	return filepath.Join(s.runsDir(), runID, runProgressFileName)
}
