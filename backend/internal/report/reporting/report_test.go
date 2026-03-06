package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/report/results"
)

func TestBuildRunReportGeneratesRequiredSections(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	provenancePath := filepath.Join(bundleDir, "provenance.json")
	runSummaryPath := filepath.Join(bundleDir, "results", "run-summary.json")
	receiverPath := filepath.Join(bundleDir, "results", "receivers.json")
	rasterMetaPath := filepath.Join(bundleDir, "results", "lden.json")
	modelDumpPath := filepath.Join(bundleDir, "model", "model.dump.json")

	if err := os.MkdirAll(filepath.Dir(runSummaryPath), 0o755); err != nil {
		t.Fatalf("create results dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(modelDumpPath), 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}

	provenance := map[string]any{
		"standard": map[string]any{
			"id":      "cnossos-road",
			"version": "2021.1",
			"profile": "eu-default",
		},
		"parameters": map[string]string{
			"grid_resolution_m": "10",
			"min_distance_m":    "1",
		},
		"input_hashes": map[string]string{
			".noise/model/model.normalized.geojson": "abc123",
			"traffic/road.geojson":                  "def456",
		},
	}
	if err := writeJSONFile(provenancePath, provenance); err != nil {
		t.Fatalf("write provenance: %v", err)
	}

	runSummary := map[string]any{
		"source_count":   2,
		"receiver_count": 3,
		"grid_width":     3,
		"grid_height":    1,
		"output_hash":    "hash-001",
	}
	if err := writeJSONFile(runSummaryPath, runSummary); err != nil {
		t.Fatalf("write run summary: %v", err)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{"Lden", "Lnight"},
		Unit:           "dB",
		Records: []results.ReceiverRecord{
			{ID: "rx-1", X: 0, Y: 0, HeightM: 4, Values: map[string]float64{"Lden": 50, "Lnight": 40}},
			{ID: "rx-2", X: 1, Y: 0, HeightM: 4, Values: map[string]float64{"Lden": 55, "Lnight": 45}},
			{ID: "rx-3", X: 2, Y: 0, HeightM: 4, Values: map[string]float64{"Lden": 53, "Lnight": 43}},
		},
	}
	if err := results.SaveReceiverTableJSON(receiverPath, table); err != nil {
		t.Fatalf("write receiver table: %v", err)
	}

	if err := writeJSONFile(rasterMetaPath, map[string]any{
		"width":      3,
		"height":     1,
		"bands":      1,
		"unit":       "dB",
		"band_names": []string{"Lden"},
		"data_file":  "lden.bin",
	}); err != nil {
		t.Fatalf("write raster metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "results", "lden.bin"), []byte{1, 2, 3, 4}, 0o644); err != nil {
		t.Fatalf("write raster binary: %v", err)
	}

	if err := writeJSONFile(modelDumpPath, map[string]any{
		"source_path":   "traffic/road.geojson",
		"feature_count": 4,
		"counts_by_kind": map[string]int{
			"source":   2,
			"building": 1,
			"barrier":  1,
		},
	}); err != nil {
		t.Fatalf("write model dump: %v", err)
	}

	report, err := BuildRunReport(BuildOptions{
		BundleDir:         bundleDir,
		Project:           project.Project{ProjectID: "proj-1", Name: "Demo", CRS: "EPSG:25832"},
		Run:               project.Run{ID: "run-1", ScenarioID: "default", Status: "completed", StartedAt: time.Unix(100, 0), FinishedAt: time.Unix(200, 0)},
		ProvenancePath:    provenancePath,
		RunSummaryPath:    runSummaryPath,
		ReceiverTablePath: receiverPath,
		RasterMetaPaths:   []string{rasterMetaPath},
		ModelDumpPath:     modelDumpPath,
		QASuites: []QASuiteStatus{
			{Name: "golden", Status: "passed", Details: "phase8 fixture"},
		},
		GeneratedAt: time.Unix(300, 0),
	})
	if err != nil {
		t.Fatalf("build run report: %v", err)
	}

	markdown, err := os.ReadFile(report.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown report: %v", err)
	}
	markdownText := string(markdown)
	for _, section := range []string{
		"## Input overview",
		"## Standard ID + version/profile + parameters",
		"## Maps/images",
		"## Tables (receiver stats)",
		"## QA status (which suites passed)",
	} {
		if !strings.Contains(markdownText, section) {
			t.Fatalf("expected markdown to contain section %q", section)
		}
	}
	if !strings.Contains(markdownText, "Lden | 50.000 | 52.667 | 55.000") {
		t.Fatalf("expected receiver stats row in markdown: %s", markdownText)
	}

	html, err := os.ReadFile(report.HTMLPath)
	if err != nil {
		t.Fatalf("read html report: %v", err)
	}
	htmlText := string(html)
	if !strings.Contains(htmlText, "<h2>QA status (which suites passed)</h2>") {
		t.Fatalf("expected QA section in html report")
	}

	payload, err := os.ReadFile(report.ContextPath)
	if err != nil {
		t.Fatalf("read report context: %v", err)
	}
	var context map[string]any
	if err := json.Unmarshal(payload, &context); err != nil {
		t.Fatalf("decode context json: %v", err)
	}
	indicators, ok := context["indicators"].([]any)
	if !ok || len(indicators) != 2 {
		t.Fatalf("expected two indicator stats in context, got %#v", context["indicators"])
	}
}

func TestBuildRunReportUsesDefaultQABaseline(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	report, err := BuildRunReport(BuildOptions{
		BundleDir: bundleDir,
		Project:   project.Project{ProjectID: "proj-2", Name: "NoData"},
		Run:       project.Run{ID: "run-2", ScenarioID: "default", Status: "completed"},
	})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}

	markdown, err := os.ReadFile(report.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	text := string(markdown)
	if !strings.Contains(text, "phase20-baseline") {
		t.Fatalf("expected default QA suite row, got: %s", text)
	}
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}
