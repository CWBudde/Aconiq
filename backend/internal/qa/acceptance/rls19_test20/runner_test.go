package rls19_test20

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/qa/acceptance"
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

	_, err = os.Stat(report.ReportPath)
	if err != nil {
		t.Fatalf("expected report artifact: %v", err)
	}
}

func TestRunLocalSuiteModeSkipsWithExplicitReason(t *testing.T) {
	// Not parallel: the strict-mode switch is process-wide environment state.
	t.Setenv(acceptance.StrictSuiteEnv, "0")

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

// TestRunLocalSuiteModeFailsUnderStrictAcceptance covers the other half of the
// policy: where the local fixtures are supposed to be installed, a suite that
// produced no evidence must be red rather than a silent green.
func TestRunLocalSuiteModeFailsUnderStrictAcceptance(t *testing.T) {
	// Not parallel: the strict-mode switch is process-wide environment state.
	t.Setenv(acceptance.StrictSuiteEnv, "1")

	report, err := Run(Options{
		Mode:          ModeLocalSuite,
		LocalSuiteDir: filepath.Join(t.TempDir(), "missing"),
		OutputDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run local suite mode: %v", err)
	}

	if report.Status != acceptance.StatusFailed {
		t.Fatalf("status = %q, want %q under %s=1", report.Status, acceptance.StatusFailed, acceptance.StrictSuiteEnv)
	}

	if !strings.Contains(report.SkipReason, acceptance.StrictSuiteEnv) {
		t.Fatalf("skip reason = %q, want it to explain the escalation", report.SkipReason)
	}
}

// TestCISafeSuiteExecutesTasks is the anti-silent-green guard for the
// repo-authored suite: it ships in testdata/ and is always available, so an
// all-skipped or empty run means the suite stopped checking anything.
func TestCISafeSuiteExecutesTasks(t *testing.T) {
	t.Parallel()

	report, err := Run(Options{
		Mode:      ModeCISafe,
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run ci-safe suite: %v", err)
	}

	if report.TaskCount == 0 {
		t.Fatal("the CI-safe suite executed no tasks at all")
	}

	if report.SkippedCount == report.TaskCount {
		t.Fatalf("every one of the %d CI-safe tasks skipped: %s", report.TaskCount, report.SkipReason)
	}

	if report.PassedCount == 0 {
		t.Fatalf("the CI-safe suite produced no passing task: %#v", report)
	}

	if report.SkipReason != "" {
		t.Fatalf("a suite that executed tasks must not carry a skip reason: %q", report.SkipReason)
	}
}

func TestConformanceReportContainsRequiredFields(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()

	report, err := Run(Options{
		Mode:      ModeCISafe,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("run ci-safe suite: %v", err)
	}

	if report.StandardID == "" {
		t.Fatal("expected standard_id")
	}

	if report.SuiteVersion == "" {
		t.Fatal("expected suite_version")
	}

	if report.EvidenceClass == "" {
		t.Fatal("expected evidence_class")
	}

	if report.Provenance == "" {
		t.Fatal("expected provenance")
	}

	// Category coverage summary.
	if report.CategoryCoverage == nil {
		t.Fatal("expected category_coverage")
	}

	categories := []string{"emission", "immission", "complex", "parking"}
	for _, cat := range categories {
		cs, ok := report.CategoryCoverage[cat]
		if !ok {
			t.Fatalf("expected category_coverage to include %q", cat)
		}

		if cs.TaskCount == 0 {
			t.Fatalf("expected non-zero task count for category %q", cat)
		}
	}

	// Verify report artifact roundtrips.
	data, err := os.ReadFile(report.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var parsed Report

	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("decode report: %v", err)
	}

	if parsed.CategoryCoverage == nil {
		t.Fatal("expected category_coverage in persisted report")
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

		err := decodeJSONFile(scenarioPath, &scenario)
		if err != nil {
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

		err = writeJSONFile(expectedPath, expectedSnapshotFile{
			Receivers: snapshotsFromOutputs(outputs),
		})
		if err != nil {
			t.Fatalf("write expected snapshot %s: %v", task.Name, err)
		}
	}
}

// TestParkingFixtureRelationsHoldByArithmetic checks the two relations between
// the parking fixtures that do not depend on the snapshots being right.
//
// The CI-safe suite otherwise pins Aconiq against itself, which proves nothing
// about agreement with RLS-19. These two do carry independent arithmetic: the
// P2/P1 delta follows from Eq. 10 alone, and P3 must be strictly quieter than
// P2 because a barrier stands between the lot and the receiver — which it was
// not before Parkplatz contributions were given D_z.
func TestParkingFixtureRelationsHoldByArithmetic(t *testing.T) {
	t.Parallel()

	levels := map[string]float64{}

	for _, name := range []string{"p1_parking_pr_pkw", "p2_parking_lkw_omnibus", "p3_parking_shielded"} {
		var snapshot expectedSnapshotFile

		path := filepath.Join(packageDir(), "testdata", "ci_safe", name+".golden.json")

		err := decodeJSONFile(path, &snapshot)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		if len(snapshot.Receivers) != 1 {
			t.Fatalf("%s: expected one receiver, got %d", name, len(snapshot.Receivers))
		}

		levels[name] = snapshot.Receivers[0].LrDay
	}

	// Eq. 10: the lots differ only in N (1.5 vs 0.3 movements per space and
	// hour) and in D_P,PT (10 dB vs 0 dB), so the level differs by
	// 10 lg(1.5/0.3) + 10 dB and by nothing else.
	wantDelta := 10*math.Log10(1.5/0.3) + 10

	gotDelta := levels["p2_parking_lkw_omnibus"] - levels["p1_parking_pr_pkw"]
	if math.Abs(gotDelta-wantDelta) > 1e-4 {
		t.Errorf("P2 - P1 = %.6f dB, want %.6f dB from Eq. 10", gotDelta, wantDelta)
	}

	shielding := levels["p2_parking_lkw_omnibus"] - levels["p3_parking_shielded"]
	if shielding <= 0 {
		t.Errorf("the barrier must lower the Parkplatz contribution, got %.6f dB", shielding)
	}
}
