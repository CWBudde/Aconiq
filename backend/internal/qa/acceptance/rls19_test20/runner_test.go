package rls19_test20

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

func TestRunCISafeSuiteProducesPassingReport(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()

	report, err := Run(Options{
		Mode:      ModeCISafe,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("run ci-safe suite: %v", err)
	}

	if report.Status != "passed" {
		t.Fatalf("expected passed report, got %#v", report)
	}

	if report.TaskCount == 0 || report.PassedCount != report.TaskCount {
		t.Fatalf("unexpected task counts: %#v", report)
	}

	if report.ReportPath == "" {
		t.Fatal("expected report path")
	}

	if _, err := os.Stat(report.ReportPath); err != nil {
		t.Fatalf("expected report artifact: %v", err)
	}
}

func TestRunLocalSuiteModeSkipsWithExplicitReason(t *testing.T) {
	t.Parallel()

	report, err := Run(Options{
		Mode:          ModeLocalSuite,
		LocalSuiteDir: filepath.Join(t.TempDir(), "missing"),
		OutputDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run local suite mode: %v", err)
	}

	if report.Status != "skipped" {
		t.Fatalf("expected skipped report, got %#v", report)
	}

	if report.SkipReason == "" {
		t.Fatalf("expected explicit skip reason, got %#v", report)
	}
}

func TestUpdateCISafeExpectedSnapshots(t *testing.T) {
	t.Parallel()

	if !golden.UpdateEnabled() {
		t.Skip("golden update disabled")
	}

	manifestPath := filepath.Join(packageDir(), "testdata", "ci_safe_suite.json")

	suite, suiteDir, err := loadSuiteManifest(manifestPath)
	if err != nil {
		t.Fatalf("load suite manifest: %v", err)
	}

	for _, task := range suite.Tasks {
		var scenario scenarioFile

		scenarioPath := filepath.Join(suiteDir, filepath.FromSlash(task.ScenarioPath))
		if err := decodeJSONFile(scenarioPath, &scenario); err != nil {
			t.Fatalf("decode scenario %s: %v", task.Name, err)
		}

		outputs, err := rls19road.ComputeReceiverOutputs(
			scenario.Receivers,
			scenario.Sources,
			scenario.Barriers,
			scenario.PropagationConfig.toPropagationConfig(scenario.Buildings),
		)
		if err != nil {
			t.Fatalf("compute scenario %s: %v", task.Name, err)
		}

		expectedPath := filepath.Join(suiteDir, filepath.FromSlash(task.ExpectedPath))
		if err := writeJSONFile(expectedPath, expectedSnapshotFile{
			Receivers: snapshotsFromOutputs(outputs),
		}); err != nil {
			t.Fatalf("write expected snapshot %s: %v", task.Name, err)
		}
	}
}
