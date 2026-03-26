package rls19_test20

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/geo"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

const (
	ModeCISafe             = "ci-safe"
	ModeLocalSuite         = "local-suite"
	localSuiteManifestName = "suite.json"
	taskStatusFailed       = "failed"
)

type Options struct {
	Mode          string
	LocalSuiteDir string
	OutputDir     string
	GeneratedAt   time.Time
}

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
	SkippedCount     int                       `json:"skipped_count"`
	Tasks            []TaskResult              `json:"tasks"`
	CategoryCoverage map[string]CategoryStatus `json:"category_coverage,omitempty"`
	ReportPath       string                    `json:"report_path,omitempty"`
	SkipReason       string                    `json:"skip_reason,omitempty"`
}

type TaskResult struct {
	Name          string             `json:"name"`
	Category      string             `json:"category"`
	Setting       string             `json:"setting"`
	Status        string             `json:"status"`
	Description   string             `json:"description,omitempty"`
	Tolerance     Tolerance          `json:"tolerance"`
	ScenarioPath  string             `json:"scenario_path"`
	ExpectedPath  string             `json:"expected_path,omitempty"`
	MaxAbsDeltaDB float64            `json:"max_abs_delta_db,omitempty"`
	ReceiverCount int                `json:"receiver_count"`
	Expected      []ReceiverSnapshot `json:"expected,omitempty"`
	Actual        []ReceiverSnapshot `json:"actual,omitempty"`
	Details       string             `json:"details,omitempty"`
}

type Tolerance struct {
	AbsoluteDB float64 `json:"absolute_db"`
	Rule       string  `json:"rule"`
}

// CategoryStatus summarizes pass/fail/skip counts for one task category.
type CategoryStatus struct {
	TaskCount int `json:"task_count"`
	PassCount int `json:"pass_count"`
	FailCount int `json:"fail_count"`
	SkipCount int `json:"skip_count"`
}

type suiteManifest struct {
	Name          string         `json:"name"`
	StandardID    string         `json:"standard_id"`
	SuiteVersion  string         `json:"suite_version"`
	EvidenceClass string         `json:"evidence_class"`
	Provenance    string         `json:"provenance"`
	Tasks         []taskManifest `json:"tasks"`
}

type taskManifest struct {
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	Setting      string    `json:"setting"`
	Description  string    `json:"description"`
	ScenarioPath string    `json:"scenario_path"`
	ExpectedPath string    `json:"expected_path,omitempty"`
	Tolerance    Tolerance `json:"tolerance"`
}

type scenarioFile struct {
	Sources           []rls19road.RoadSource `json:"sources"`
	Barriers          []rls19road.Barrier    `json:"barriers,omitempty"`
	Buildings         []rls19road.Building   `json:"buildings,omitempty"`
	Receivers         []geo.PointReceiver    `json:"receivers"`
	PropagationConfig propagationConfigFile  `json:"propagation_config"`
}

type propagationConfigFile struct {
	SegmentLengthM   float64                    `json:"segment_length_m"`
	MinDistanceM     float64                    `json:"min_distance_m"`
	ReceiverHeightM  float64                    `json:"receiver_height_m"`
	ReceiverTerrainZ float64                    `json:"receiver_terrain_z,omitempty"`
	Terrain          []rls19road.TerrainProfile `json:"terrain,omitempty"`
	Reflectors       []rls19road.Reflector      `json:"reflectors,omitempty"`
}

type expectedSnapshotFile struct {
	Receivers []ReceiverSnapshot `json:"receivers"`
}

type ReceiverSnapshot struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	HeightM float64 `json:"height_m"`
	LrDay   float64 `json:"LrDay"`
	LrNight float64 `json:"LrNight"`
}

func Run(opts Options) (Report, error) {
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeCISafe
	}

	generatedAt := opts.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	manifestPath, skipReason, err := resolveManifestPath(mode, opts.LocalSuiteDir)
	if err != nil {
		return Report{}, err
	}

	if skipReason != "" {
		report := Report{
			SuiteName:   "rls19-test20",
			StandardID:  rls19road.StandardID,
			Mode:        mode,
			Status:      "skipped",
			GeneratedAt: generatedAt,
			SkipReason:  skipReason,
		}
		if opts.OutputDir != "" {
			reportPath, err := writeReportArtifact(opts.OutputDir, mode, report)
			if err != nil {
				return Report{}, err
			}

			report.ReportPath = reportPath
		}

		return report, nil
	}

	suite, suiteDir, err := loadSuiteManifest(manifestPath)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		SuiteName:     suite.Name,
		StandardID:    suite.StandardID,
		Mode:          mode,
		SuiteVersion:  suite.SuiteVersion,
		EvidenceClass: suite.EvidenceClass,
		Provenance:    suite.Provenance,
		GeneratedAt:   generatedAt,
		Tasks:         make([]TaskResult, 0, len(suite.Tasks)),
	}

	for _, task := range suite.Tasks {
		result, err := runTask(task, suiteDir)
		if err != nil {
			return Report{}, err
		}

		report.Tasks = append(report.Tasks, result)
	}

	report.TaskCount = len(report.Tasks)

	report.Status = "passed"
	for _, task := range report.Tasks {
		switch task.Status {
		case "passed":
			report.PassedCount++
		case taskStatusFailed:
			report.FailedCount++
			report.Status = taskStatusFailed
		case "skipped":
			report.SkippedCount++
			if report.Status == "passed" {
				report.Status = "skipped"
			}
		}
	}

	coverage := make(map[string]CategoryStatus)
	for _, task := range report.Tasks {
		cs := coverage[task.Category]
		cs.TaskCount++

		switch task.Status {
		case "passed":
			cs.PassCount++
		case taskStatusFailed:
			cs.FailCount++
		case "skipped":
			cs.SkipCount++
		}

		coverage[task.Category] = cs
	}

	report.CategoryCoverage = coverage

	if opts.OutputDir != "" {
		reportPath, err := writeReportArtifact(opts.OutputDir, mode, report)
		if err != nil {
			return Report{}, err
		}

		report.ReportPath = reportPath
	}

	return report, nil
}

func resolveManifestPath(mode string, localSuiteDir string) (string, string, error) {
	switch mode {
	case ModeCISafe:
		return filepath.Join(packageDir(), "testdata", "ci_safe_suite.json"), "", nil
	case ModeLocalSuite:
		trimmed := strings.TrimSpace(localSuiteDir)
		if trimmed == "" {
			return "", "local suite mode requested but no suite directory was provided", nil
		}

		manifestPath := filepath.Join(trimmed, localSuiteManifestName)

		_, err := os.Stat(manifestPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", "local suite mode requested but suite manifest was not found: " + manifestPath, nil
			}

			return "", "", fmt.Errorf("stat local suite manifest: %w", err)
		}

		return manifestPath, "", nil
	default:
		return "", "", fmt.Errorf("unsupported mode %q", mode)
	}
}

func loadSuiteManifest(path string) (suiteManifest, string, error) {
	var suite suiteManifest

	err := decodeJSONFile(path, &suite)
	if err != nil {
		return suiteManifest{}, "", err
	}

	sort.SliceStable(suite.Tasks, func(i, j int) bool { return suite.Tasks[i].Name < suite.Tasks[j].Name })

	return suite, filepath.Dir(path), nil
}

func runTask(task taskManifest, suiteDir string) (TaskResult, error) {
	result := TaskResult{
		Name:         task.Name,
		Category:     task.Category,
		Setting:      task.Setting,
		Description:  task.Description,
		Tolerance:    task.Tolerance,
		ScenarioPath: filepath.ToSlash(task.ScenarioPath),
		ExpectedPath: filepath.ToSlash(task.ExpectedPath),
	}

	scenarioPath := filepath.Join(suiteDir, filepath.FromSlash(task.ScenarioPath))

	var scenario scenarioFile

	err := decodeJSONFile(scenarioPath, &scenario)
	if err != nil {
		return TaskResult{}, err
	}

	outputs, err := rls19road.ComputeReceiverOutputs(
		scenario.Receivers,
		scenario.Sources,
		scenario.Barriers,
		scenario.PropagationConfig.toPropagationConfig(scenario.Buildings),
	)
	if err != nil {
		return TaskResult{}, err
	}

	result.Actual = snapshotsFromOutputs(outputs)
	result.ReceiverCount = len(result.Actual)

	if task.ExpectedPath == "" {
		result.Status = "skipped"
		result.Details = "no expected snapshot configured"

		return result, nil
	}

	expectedPath := filepath.Join(suiteDir, filepath.FromSlash(task.ExpectedPath))

	var expected expectedSnapshotFile

	err = decodeJSONFile(expectedPath, &expected)
	if err != nil {
		return TaskResult{}, err
	}

	result.Expected = expected.Receivers
	maxDelta, compareErr := compareSnapshots(expected.Receivers, result.Actual, task.Tolerance.AbsoluteDB)

	result.MaxAbsDeltaDB = round6(maxDelta)
	if compareErr != nil {
		// A failed comparison is reported as task failure, not as a hard execution error.
		result.Status = taskStatusFailed
		result.Details = compareErr.Error()

		return result, nil //nolint:nilerr
	}

	result.Status = "passed"

	return result, nil
}

func (cfg propagationConfigFile) toPropagationConfig(buildings []rls19road.Building) rls19road.PropagationConfig {
	return rls19road.PropagationConfig{
		SegmentLengthM:   cfg.SegmentLengthM,
		MinDistanceM:     cfg.MinDistanceM,
		ReceiverHeightM:  cfg.ReceiverHeightM,
		ReceiverTerrainZ: cfg.ReceiverTerrainZ,
		Terrain:          cfg.Terrain,
		Reflectors:       cfg.Reflectors,
		Buildings:        buildings,
	}
}

func snapshotsFromOutputs(outputs []rls19road.ReceiverOutput) []ReceiverSnapshot {
	out := make([]ReceiverSnapshot, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, ReceiverSnapshot{
			ID:      output.Receiver.ID,
			X:       round6(output.Receiver.Point.X),
			Y:       round6(output.Receiver.Point.Y),
			HeightM: round6(output.Receiver.HeightM),
			LrDay:   round6(output.Indicators.LrDay),
			LrNight: round6(output.Indicators.LrNight),
		})
	}

	return out
}

func compareSnapshots(expected []ReceiverSnapshot, actual []ReceiverSnapshot, tolerance float64) (float64, error) {
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
			{"x", expected[i].X, actual[i].X},
			{"y", expected[i].Y, actual[i].Y},
			{"height_m", expected[i].HeightM, actual[i].HeightM},
			{"LrDay", expected[i].LrDay, actual[i].LrDay},
			{"LrNight", expected[i].LrNight, actual[i].LrNight},
		} {
			delta := math.Abs(pair.expected - pair.actual)
			if delta > maxDelta {
				maxDelta = delta
			}

			if delta > tolerance {
				return maxDelta, fmt.Errorf("receiver %q %s exceeded tolerance: expected %.6f, got %.6f, tolerance %.6f", expected[i].ID, pair.name, pair.expected, pair.actual, tolerance)
			}
		}
	}

	return maxDelta, nil
}

func writeReportArtifact(outputDir string, mode string, report Report) (string, error) {
	path := filepath.Join(outputDir, "rls19-test20-"+mode+".json")

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
		return fmt.Errorf("marshal %s: %w", path, err)
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

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}
