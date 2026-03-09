package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/projectfs"
)

func TestExportGeneratesReportBundle(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase20", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(
		t,
		"--project", projectDir,
		"run",
		"--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10",
		"--param", "grid_padding_m=0",
		"--param", "source_emission_db=90",
		"--param", "receiver_height_m=4",
		"--param", "chunk_size=3",
		"--param", "workers=2",
	)
	mustRunCLI(t, "--project", projectDir, "export")

	bundleDir := latestExportBundleDir(t, projectDir)
	assertFileExists(t, filepath.Join(bundleDir, "report.md"))
	assertFileExists(t, filepath.Join(bundleDir, "report.html"))
	assertFileExists(t, filepath.Join(bundleDir, "report.typ"))
	assertFileExists(t, filepath.Join(bundleDir, "report-context.json"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "receivers.json"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "run-summary.json"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "ldummy.json"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "ldummy.bin"))

	reportMarkdown, err := os.ReadFile(filepath.Join(bundleDir, "report.md"))
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}

	reportText := string(reportMarkdown)
	for _, section := range []string{
		"## Input overview",
		"## Standard ID + version/profile + parameters",
		"## Maps/images",
		"## Tables (receiver stats)",
		"## QA status (which suites passed)",
	} {
		if !strings.Contains(reportText, section) {
			t.Fatalf("missing report section %q", section)
		}
	}

	summaryPayload, err := os.ReadFile(filepath.Join(bundleDir, "export-summary.json"))
	if err != nil {
		t.Fatalf("read export summary: %v", err)
	}

	var summary map[string]any

	err = json.Unmarshal(summaryPayload, &summary)
	if err != nil {
		t.Fatalf("decode export summary: %v", err)
	}

	generatedReports, ok := summary["generated_reports"].([]any)
	if !ok || len(generatedReports) < 3 {
		t.Fatalf("expected generated report files in summary, got %#v", summary["generated_reports"])
	}

	copiedFiles := anySliceToStrings(summary["copied_files"])
	for _, expected := range []string{"results/receivers.json", "results/run-summary.json", "provenance.json"} {
		if !slices.Contains(copiedFiles, expected) {
			t.Fatalf("expected copied file %q in export summary", expected)
		}
	}

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	foundHTMLArtifact := false
	foundTypstArtifact := false

	for _, artifact := range proj.Artifacts {
		if artifact.Kind == "export.report_html" {
			foundHTMLArtifact = true
		}

		if artifact.Kind == "export.report_typst" {
			foundTypstArtifact = true
		}
	}

	if !foundHTMLArtifact {
		t.Fatalf("expected export.report_html artifact in project manifest")
	}

	if !foundTypstArtifact {
		t.Fatalf("expected export.report_typst artifact in project manifest")
	}
}

func TestExportSkipReport(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase20Skip", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield")
	mustRunCLI(t, "--project", projectDir, "export", "--skip-report")

	bundleDir := latestExportBundleDir(t, projectDir)

	_, err := os.Stat(filepath.Join(bundleDir, "report.html"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected report.html to be skipped")
	}

	_, err = os.Stat(filepath.Join(bundleDir, "report.typ"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected report.typ to be skipped")
	}

	summaryPayload, err := os.ReadFile(filepath.Join(bundleDir, "export-summary.json"))
	if err != nil {
		t.Fatalf("read export summary: %v", err)
	}

	var summary map[string]any

	err = json.Unmarshal(summaryPayload, &summary)
	if err != nil {
		t.Fatalf("decode export summary: %v", err)
	}

	if generated, exists := summary["generated_reports"]; exists && len(anySliceToStrings(generated)) > 0 {
		t.Fatalf("expected generated_reports to be empty when --skip-report is set, got %#v", generated)
	}
}

func TestExportHandlesCustomReceiverRunsWithoutRasterArtifacts(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "rls19_custom_receivers.geojson")
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rls19-rd-1", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [20, 15]}
    }
  ]
}`)
	if err := os.WriteFile(modelPath, payload, 0o644); err != nil {
		t.Fatalf("write custom model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase30Export", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road", "--receiver-mode", "custom")
	mustRunCLI(t, "--project", projectDir, "export")

	bundleDir := latestExportBundleDir(t, projectDir)
	assertFileExists(t, filepath.Join(bundleDir, "report.md"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "receivers.json"))
	assertFileExists(t, filepath.Join(bundleDir, "results", "run-summary.json"))

	if _, err := os.Stat(filepath.Join(bundleDir, "results", "rls19-road.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no raster metadata in export bundle, got err=%v", err)
	}

	reportMarkdown, err := os.ReadFile(filepath.Join(bundleDir, "report.md"))
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}

	if !strings.Contains(string(reportMarkdown), "No map/image artifacts were available") {
		t.Fatalf("expected no-map note in report markdown")
	}
}

func TestExportRejectsPDFWithSkipReport(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase20PDFConflict", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield")

	err := runCLI("--project", projectDir, "export", "--skip-report", "--pdf")
	if err == nil {
		t.Fatal("expected export command error")
	}

	if !strings.Contains(err.Error(), "--pdf cannot be used together with --skip-report") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func latestExportBundleDir(t *testing.T, projectDir string) string {
	t.Helper()

	exportRoot := filepath.Join(projectDir, ".noise", "exports")

	entries, err := os.ReadDir(exportRoot)
	if err != nil {
		t.Fatalf("read exports directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one export bundle")
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		names = append(names, entry.Name())
	}

	if len(names) == 0 {
		t.Fatal("expected at least one export bundle directory")
	}

	slices.Sort(names)

	return filepath.Join(exportRoot, names[len(names)-1])
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func anySliceToStrings(value any) []string {
	rawSlice, ok := value.([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(rawSlice))
	for _, item := range rawSlice {
		text, ok := item.(string)
		if !ok {
			continue
		}

		out = append(out, text)
	}

	return out
}
