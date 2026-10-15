package schall03runner_test

import (
	"testing"

	schall03runner "github.com/aconiq/backend/internal/qa/acceptance/schall03"
	"github.com/aconiq/backend/internal/qa/golden"
)

func TestRunCISafeSuiteProducesPassingReport(t *testing.T) {
	t.Parallel()

	report, err := schall03runner.Run(schall03runner.Options{
		Mode:      schall03runner.ModeCISafe,
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The snapshots are compared within a tolerance, so they hold on arm64 as
	// well as on amd64: what is left between the two is rounding, and no
	// geometric decision in the suite turns on it any more (b5 did, through a
	// collinear rubber band; see geo.HullCollinearToleranceM).
	if report.Status != "passed" {
		t.Errorf("expected status=passed, got %q", report.Status)

		for _, task := range report.Tasks {
			if task.Status == "failed" {
				t.Logf("FAIL: %s — %s", task.Name, task.Details)
			}
		}
	}

	if report.TaskCount == 0 {
		t.Error("expected at least one task in the report")
	}
}

func TestUpdateGoldenSnapshots(t *testing.T) {
	if !golden.UpdateEnabled() {
		t.Skip("set UPDATE_GOLDEN=1 to regenerate expected snapshots")
	}

	err := schall03runner.WriteGoldenSnapshots()
	if err != nil {
		t.Fatalf("WriteGoldenSnapshots: %v", err)
	}

	t.Log("Golden snapshots updated successfully.")
}
