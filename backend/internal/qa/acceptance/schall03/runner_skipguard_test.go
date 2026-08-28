package schall03runner

import (
	"testing"
	"time"

	"github.com/aconiq/backend/internal/qa/acceptance"
)

// A Schall 03 task skips when its manifest entry configures no expected
// snapshot. Before the skip guard was wired in, buildReport counted only
// passes and failures, so a suite in which every task skipped reported
// "passed" while carrying no evidence at all.
func skippedTasks(count int) []TaskResult {
	tasks := make([]TaskResult, 0, count)
	for i := range count {
		tasks = append(tasks, TaskResult{
			Name:     string(rune('a' + i)),
			Category: "emission",
			Status:   statusSkipped,
		})
	}

	return tasks
}

func TestFullySkippedSuiteIsNotReportedAsPassed(t *testing.T) {
	t.Parallel()

	report := buildReport(suiteManifest{Name: "schall03-ci-safe"}, ModeCISafe, time.Time{}, skippedTasks(3))

	if report.Status != acceptance.StatusSkipped {
		t.Errorf("expected status=%q for a suite without evidence, got %q", acceptance.StatusSkipped, report.Status)
	}

	if report.SkippedCount != 3 {
		t.Errorf("expected skipped_count=3, got %d", report.SkippedCount)
	}

	if report.SkipReason == "" {
		t.Error("expected an explicit skip reason, got none")
	}

	if coverage := report.CategoryCoverage["emission"]; coverage.SkipCount != 3 {
		t.Errorf("expected the category coverage to record 3 skips, got %d", coverage.SkipCount)
	}
}

func TestFullySkippedSuiteFailsUnderStrictMode(t *testing.T) {
	t.Setenv(acceptance.StrictSuiteEnv, "1")

	report := buildReport(suiteManifest{Name: "schall03-ci-safe"}, ModeCISafe, time.Time{}, skippedTasks(2))

	if report.Status != acceptance.StatusFailed {
		t.Errorf("expected status=%q under %s, got %q", acceptance.StatusFailed, acceptance.StrictSuiteEnv, report.Status)
	}
}

func TestPartiallySkippedSuiteKeepsItsOwnStatus(t *testing.T) {
	t.Parallel()

	tasks := append(skippedTasks(2), TaskResult{Name: "d", Category: "emission", Status: statusPassed})

	report := buildReport(suiteManifest{Name: "schall03-ci-safe"}, ModeCISafe, time.Time{}, tasks)

	if report.Status != statusPassed {
		t.Errorf("expected a suite that executed something to keep status=passed, got %q", report.Status)
	}

	if report.SkipReason != "" {
		t.Errorf("expected no skip reason on an executed suite, got %q", report.SkipReason)
	}
}
