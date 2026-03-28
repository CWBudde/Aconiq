package schall03runner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	schall03 "github.com/aconiq/backend/internal/standards/schall03"
)

const (
	// ModeCISafe runs the repo-authored synthetic suite that ships in testdata/.
	ModeCISafe = "ci-safe"

	statusPassed = "passed"
	statusFailed = "failed"
)

// Options controls a Run invocation.
type Options struct {
	Mode        string
	OutputDir   string
	GeneratedAt time.Time
}

// Report is the top-level run result returned by Run.
type Report struct {
	SuiteName        string                    `json:"suite_name"`
	StandardID       string                    `json:"standard_id"`
	Mode             string                    `json:"mode"`
	Status           string                    `json:"status"`
	SuiteVersion     string                    `json:"suite_version,omitempty"`
	EvidenceClass    string                    `json:"evidence_class,omitempty"`
	Provenance       string                    `json:"provenance,omitempty"`
	GeneratedAt      time.Time                 `json:"generated_at"`
	TaskCount        int                       `json:"task_count"`
	PassedCount      int                       `json:"passed_count"`
	FailedCount      int                       `json:"failed_count"`
	Tasks            []TaskResult              `json:"tasks"`
	CategoryCoverage map[string]CategoryStatus `json:"category_coverage,omitempty"`
	ReportPath       string                    `json:"report_path,omitempty"`
}

// TaskResult holds the outcome of a single conformance scenario.
type TaskResult struct {
	Name          string             `json:"name"`
	Category      string             `json:"category"`
	Status        string             `json:"status"`
	Description   string             `json:"description,omitempty"`
	ToleranceDB   float64            `json:"tolerance_db"`
	ScenarioPath  string             `json:"scenario_path"`
	ExpectedPath  string             `json:"expected_path,omitempty"`
	MaxAbsDeltaDB float64            `json:"max_abs_delta_db,omitempty"`
	ReceiverCount int                `json:"receiver_count"`
	Details       string             `json:"details,omitempty"`
	Expected      []ReceiverSnapshot `json:"expected,omitempty"`
	Actual        []ReceiverSnapshot `json:"actual,omitempty"`
}

// CategoryStatus summarizes pass/fail counts for one task category.
type CategoryStatus struct {
	TaskCount int `json:"task_count"`
	PassCount int `json:"pass_count"`
	FailCount int `json:"fail_count"`
}

// ReceiverSnapshot is one row in an expected or actual output file.
//
//nolint:tagliatelle // level indicator names (LpAeqDay etc.) follow Schall 03 notation, not generic snake_case
type ReceiverSnapshot struct {
	ID         string  `json:"id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	HeightM    float64 `json:"height_m"`
	LpAeqDay   float64 `json:"LpAeqDay"`
	LpAeqNight float64 `json:"LpAeqNight"`
	LrDay      float64 `json:"LrDay"`
	LrNight    float64 `json:"LrNight"`
}

// suiteManifest is the top-level structure of a ci_safe_suite.json file.
type suiteManifest struct {
	Name          string         `json:"name"`
	StandardID    string         `json:"standard_id"`
	SuiteVersion  string         `json:"suite_version"`
	EvidenceClass string         `json:"evidence_class"`
	Provenance    string         `json:"provenance"`
	Tasks         []taskManifest `json:"tasks"`
}

type taskManifest struct {
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	ScenarioPath string  `json:"scenario_path"`
	ExpectedPath string  `json:"expected_path,omitempty"`
	ToleranceDB  float64 `json:"tolerance_db"`
}

type scenarioFile struct {
	Segments  []schall03.TrackSegment    `json:"segments"`
	Receivers []schall03.ReceiverInput   `json:"receivers"`
	Walls     []schall03.ReflectingWall  `json:"walls,omitempty"`
	Barriers  []schall03.BarrierSegment  `json:"barriers,omitempty"`
}

type expectedFile struct {
	Receivers []ReceiverSnapshot `json:"receivers"`
}

// Run executes the conformance suite specified by opts and returns a Report.
func Run(opts Options) (Report, error) {
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeCISafe
	}

	generatedAt := opts.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	if mode != ModeCISafe {
		return Report{}, fmt.Errorf("unsupported mode %q", mode)
	}

	manifestPath := filepath.Join(packageDir(), "testdata", "ci_safe_suite.json")

	var suite suiteManifest

	err := decodeJSONFile(manifestPath, &suite)
	if err != nil {
		return Report{}, err
	}

	sort.SliceStable(suite.Tasks, func(i, j int) bool { return suite.Tasks[i].Name < suite.Tasks[j].Name })

	tasks, runErr := runAllTasks(suite.Tasks, filepath.Dir(manifestPath))
	if runErr != nil {
		return Report{}, runErr
	}

	report := buildReport(suite, mode, generatedAt, tasks)

	if opts.OutputDir != "" {
		reportPath, writeErr := writeReportArtifact(opts.OutputDir, report)
		if writeErr != nil {
			return Report{}, writeErr
		}

		report.ReportPath = reportPath
	}

	return report, nil
}

func runAllTasks(tasks []taskManifest, suiteDir string) ([]TaskResult, error) {
	results := make([]TaskResult, 0, len(tasks))

	for _, task := range tasks {
		result, err := runTask(task, suiteDir)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func buildReport(suite suiteManifest, mode string, generatedAt time.Time, tasks []TaskResult) Report {
	report := Report{
		SuiteName:     suite.Name,
		StandardID:    suite.StandardID,
		Mode:          mode,
		SuiteVersion:  suite.SuiteVersion,
		EvidenceClass: suite.EvidenceClass,
		Provenance:    suite.Provenance,
		GeneratedAt:   generatedAt,
		Tasks:         tasks,
		TaskCount:     len(tasks),
		Status:        statusPassed,
	}

	for _, task := range tasks {
		switch task.Status {
		case statusPassed:
			report.PassedCount++
		case statusFailed:
			report.FailedCount++
			report.Status = statusFailed
		}
	}

	report.CategoryCoverage = buildCategoryCoverage(tasks)

	return report
}

func buildCategoryCoverage(tasks []TaskResult) map[string]CategoryStatus {
	coverage := make(map[string]CategoryStatus)

	for _, task := range tasks {
		cs := coverage[task.Category]
		cs.TaskCount++

		switch task.Status {
		case statusPassed:
			cs.PassCount++
		case statusFailed:
			cs.FailCount++
		}

		coverage[task.Category] = cs
	}

	return coverage
}

func runTask(task taskManifest, suiteDir string) (TaskResult, error) {
	result := TaskResult{
		Name:         task.Name,
		Category:     task.Category,
		Description:  task.Description,
		ToleranceDB:  task.ToleranceDB,
		ScenarioPath: filepath.ToSlash(task.ScenarioPath),
		ExpectedPath: filepath.ToSlash(task.ExpectedPath),
	}

	var scenario scenarioFile

	err := decodeJSONFile(filepath.Join(suiteDir, filepath.FromSlash(task.ScenarioPath)), &scenario)
	if err != nil {
		return TaskResult{}, err
	}

	actual, computeErr := computeSnapshots(scenario)
	if computeErr != nil {
		return TaskResult{}, computeErr
	}

	result.Actual = actual
	result.ReceiverCount = len(actual)

	if task.ExpectedPath == "" {
		result.Status = "skipped"
		result.Details = "no expected snapshot configured"

		return result, nil
	}

	var expected expectedFile

	err = decodeJSONFile(filepath.Join(suiteDir, filepath.FromSlash(task.ExpectedPath)), &expected)
	if err != nil {
		return TaskResult{}, err
	}

	result.Expected = expected.Receivers

	maxDelta, compareErr := compareSnapshots(expected.Receivers, actual, task.ToleranceDB)
	result.MaxAbsDeltaDB = round6(maxDelta)

	if compareErr != nil {
		result.Status = statusFailed
		result.Details = compareErr.Error()

		return result, nil //nolint:nilerr // compareErr is intentionally encoded in result.Status, not surfaced as a Go error
	}

	result.Status = statusPassed

	return result, nil
}

func computeSnapshots(scenario scenarioFile) ([]ReceiverSnapshot, error) {
	out := make([]ReceiverSnapshot, 0, len(scenario.Receivers))
	hasScene := len(scenario.Walls) > 0 || len(scenario.Barriers) > 0

	for _, receiver := range scenario.Receivers {
		var levels schall03.NormativeReceiverLevels

		var err error

		if hasScene {
			levels, err = schall03.ComputeNormativeReceiverLevelsWithScene(
				receiver, scenario.Segments, scenario.Walls, scenario.Barriers,
			)
		} else {
			levels, err = schall03.ComputeNormativeReceiverLevels(receiver, scenario.Segments)
		}

		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		out = append(out, ReceiverSnapshot{
			ID:         receiver.ID,
			X:          round6(receiver.Point.X),
			Y:          round6(receiver.Point.Y),
			HeightM:    round6(receiver.HeightM),
			LpAeqDay:   round6(levels.LpAeqDay),
			LpAeqNight: round6(levels.LpAeqNight),
			LrDay:      round6(levels.LrDay),
			LrNight:    round6(levels.LrNight),
		})
	}

	return out, nil
}

func compareSnapshots(expected, actual []ReceiverSnapshot, tolerance float64) (float64, error) {
	if len(expected) != len(actual) {
		return 0, fmt.Errorf("receiver count mismatch: expected %d, got %d", len(expected), len(actual))
	}

	maxDelta := 0.0

	for i := range expected {
		if expected[i].ID != actual[i].ID {
			return maxDelta, fmt.Errorf("receiver[%d] id mismatch: expected %q, got %q", i, expected[i].ID, actual[i].ID)
		}

		for _, pair := range []struct {
			name     string
			expected float64
			actual   float64
		}{
			{"LpAeqDay", expected[i].LpAeqDay, actual[i].LpAeqDay},
			{"LpAeqNight", expected[i].LpAeqNight, actual[i].LpAeqNight},
			{"LrDay", expected[i].LrDay, actual[i].LrDay},
			{"LrNight", expected[i].LrNight, actual[i].LrNight},
		} {
			delta := math.Abs(pair.expected - pair.actual)
			if delta > maxDelta {
				maxDelta = delta
			}

			if delta > tolerance {
				return maxDelta, fmt.Errorf(
					"receiver %q %s exceeded tolerance: expected %.6f, got %.6f, delta %.6f > tolerance %.6f",
					expected[i].ID, pair.name, pair.expected, pair.actual, delta, tolerance,
				)
			}
		}
	}

	return maxDelta, nil
}

// WriteGoldenSnapshots computes and writes the expected snapshot files for all
// tasks in the CI-safe suite.  Call this manually when updating normative
// coefficients or calculation logic.
func WriteGoldenSnapshots() error {
	manifestPath := filepath.Join(packageDir(), "testdata", "ci_safe_suite.json")

	var suite suiteManifest

	err := decodeJSONFile(manifestPath, &suite)
	if err != nil {
		return err
	}

	suiteDir := filepath.Dir(manifestPath)

	for _, task := range suite.Tasks {
		if task.ExpectedPath == "" {
			continue
		}

		var scenario scenarioFile

		err = decodeJSONFile(filepath.Join(suiteDir, filepath.FromSlash(task.ScenarioPath)), &scenario)
		if err != nil {
			return err
		}

		snapshots, computeErr := computeSnapshots(scenario)
		if computeErr != nil {
			return fmt.Errorf("task %q: %w", task.Name, computeErr)
		}

		expectedPath := filepath.Join(suiteDir, filepath.FromSlash(task.ExpectedPath))
		payload := expectedFile{Receivers: snapshots}

		writeErr := writeJSONFile(expectedPath, payload)
		if writeErr != nil {
			return fmt.Errorf("task %q: write golden: %w", task.Name, writeErr)
		}
	}

	return nil
}

func writeReportArtifact(outputDir string, report Report) (string, error) {
	path := filepath.Join(outputDir, "schall03-ci-safe.json")

	err := writeJSONFile(path, report)
	if err != nil {
		return "", err
	}

	return path, nil
}

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Dir(file)
}

func decodeJSONFile(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	err = json.Unmarshal(payload, target)
	if err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}

	return nil
}

func writeJSONFile(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	payload = append(payload, '\n')

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	err = os.WriteFile(path, payload, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
