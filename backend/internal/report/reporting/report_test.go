package reporting

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	typst "github.com/Dadido3/go-typst"
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
	assessmentPath := filepath.Join(bundleDir, "assessment", "16bimschv-assessment.json")

	err := os.MkdirAll(filepath.Dir(runSummaryPath), 0o750)
	if err != nil {
		t.Fatalf("create results dir: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(modelDumpPath), 0o750)
	if err != nil {
		t.Fatalf("create model dir: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(assessmentPath), 0o750)
	if err != nil {
		t.Fatalf("create assessment dir: %v", err)
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
		"metadata": map[string]string{
			"evidence_tier": "scaffold",
		},
		"input_hashes": map[string]string{
			".noise/model/model.normalized.geojson": "abc123",
			"traffic/road.geojson":                  "def456",
		},
		"standard_data": map[string]any{
			"algorithm":     "sha256",
			"digest":        "d00d",
			"evidence_tier": "scaffold",
			"tables": []map[string]string{
				{"name": "preview-road/speed-corrections", "digest": "cafe"},
				{"name": "preview-road/base-levels", "digest": "beef"},
			},
		},
	}

	err = writeJSONFile(provenancePath, provenance)
	if err != nil {
		t.Fatalf("write provenance: %v", err)
	}

	runSummary := map[string]any{
		"source_count":   2,
		"receiver_count": 3,
		"grid_width":     3,
		"grid_height":    1,
		"output_hash":    "hash-001",
	}

	err = writeJSONFile(runSummaryPath, runSummary)
	if err != nil {
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

	err = results.SaveReceiverTableJSON(receiverPath, table)
	if err != nil {
		t.Fatalf("write receiver table: %v", err)
	}

	err = writeJSONFile(rasterMetaPath, map[string]any{
		"width":      3,
		"height":     1,
		"bands":      1,
		"unit":       "dB",
		"band_names": []string{"Lden"},
		"data_file":  "lden.bin",
	})
	if err != nil {
		t.Fatalf("write raster metadata: %v", err)
	}

	err = os.WriteFile(filepath.Join(bundleDir, "results", "lden.bin"), []byte{1, 2, 3, 4}, 0o600)
	if err != nil {
		t.Fatalf("write raster binary: %v", err)
	}

	err = writeJSONFile(modelDumpPath, map[string]any{
		"source_path":   "traffic/road.geojson",
		"feature_count": 4,
		"counts_by_kind": map[string]int{
			"source":   2,
			"building": 1,
			"barrier":  1,
		},
	})
	if err != nil {
		t.Fatalf("write model dump: %v", err)
	}

	err = writeJSONFile(assessmentPath, map[string]any{
		"law":                "16. BImSchV",
		"source_standard_id": "rls19-road",
		"assessed_count":     2,
		"exceeding_count":    1,
		"category_counts": map[string]int{
			"residential": 2,
		},
		"skipped": []map[string]any{
			{"receiver_id": "rx-3", "reason": "missing 16. BImSchV area category property"},
		},
		"results": []map[string]any{
			{"summary_de": "Empfänger rx-1: Die Immissionsgrenzwerte werden eingehalten."},
			{"summary_de": "Empfänger rx-2: Die Immissionsgrenzwerte werden tags um 2 dB überschritten."},
		},
	})
	if err != nil {
		t.Fatalf("write assessment: %v", err)
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
		AssessmentPath:    assessmentPath,
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
		"## 16. BImSchV assessment",
		"## QA status (which suites passed)",
	} {
		if !strings.Contains(markdownText, section) {
			t.Fatalf("expected markdown to contain section %q", section)
		}
	}

	for _, row := range []string{
		"- Standard profile: eu-default",
		"- Evidence tier: scaffold",
		"- Standard data digest: sha256:d00d",
		"`preview-road/base-levels` (sha256=beef)",
	} {
		if !strings.Contains(markdownText, row) {
			t.Fatalf("expected markdown to contain row %q: %s", row, markdownText)
		}
	}

	if !strings.Contains(markdownText, "scaffold-tier standards module") {
		t.Fatalf("expected the scaffold disclosure note in markdown: %s", markdownText)
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

	if !strings.Contains(htmlText, "<h2>16. BImSchV assessment</h2>") {
		t.Fatalf("expected assessment section in html report")
	}

	if !strings.Contains(htmlText, "<li>Evidence tier: scaffold</li>") {
		t.Fatalf("expected evidence tier row in html report: %s", htmlText)
	}

	if !strings.Contains(htmlText, "<li>Standard data digest: <code>sha256:d00d</code></li>") {
		t.Fatalf("expected standard data digest row in html report: %s", htmlText)
	}

	if !strings.Contains(htmlText, "<td><code>preview-road/speed-corrections</code></td><td><code>cafe</code></td>") {
		t.Fatalf("expected standard data table row in html report: %s", htmlText)
	}

	if !strings.Contains(htmlText, "scaffold-tier standards module") {
		t.Fatalf("expected the scaffold disclosure note in html report: %s", htmlText)
	}

	typstSource, err := os.ReadFile(report.TypstPath)
	if err != nil {
		t.Fatalf("read typst report: %v", err)
	}

	if !strings.Contains(string(typstSource), "#show: doc => template(report)") {
		t.Fatalf("expected typst template entrypoint")
	}

	if !strings.Contains(string(typstSource), "[== 16. BImSchV assessment]") {
		t.Fatalf("expected assessment section in typst source")
	}

	if !strings.Contains(string(typstSource), "[Evidence tier: #report.EvidenceTier]") {
		t.Fatalf("expected evidence tier row in typst source")
	}

	if !strings.Contains(string(typstSource), "\"EvidenceTier\": \"scaffold\"") {
		t.Fatalf("expected evidence tier value in typst source: %s", typstSource)
	}

	if !strings.Contains(string(typstSource), "[Standard data digest: #report.StandardDataDigest]") {
		t.Fatalf("expected standard data digest row in typst source")
	}

	if !strings.Contains(string(typstSource), "\"StandardDataDigest\": \"sha256:d00d\"") {
		t.Fatalf("expected standard data digest value in typst source: %s", typstSource)
	}

	payload, err := os.ReadFile(report.ContextPath)
	if err != nil {
		t.Fatalf("read report context: %v", err)
	}

	var context map[string]any

	err = json.Unmarshal(payload, &context)
	if err != nil {
		t.Fatalf("decode context json: %v", err)
	}

	indicators, ok := context["indicators"].([]any)
	if !ok || len(indicators) != 2 {
		t.Fatalf("expected two indicator stats in context, got %#v", context["indicators"])
	}

	assessment, ok := context["assessment"].(map[string]any)
	if !ok {
		t.Fatalf("expected assessment block in context, got %#v", context["assessment"])
	}

	if assessment["law"] != "16. BImSchV" {
		t.Fatalf("unexpected assessment law: %#v", assessment["law"])
	}

	if context["evidence_tier"] != "scaffold" {
		t.Fatalf("unexpected evidence tier in context: %#v", context["evidence_tier"])
	}

	if context["standard_data_digest"] != "sha256:d00d" {
		t.Fatalf("unexpected standard data digest in context: %#v", context["standard_data_digest"])
	}

	// The tables are rendered sorted by name regardless of the order the
	// manifest listed them in, so a digest mismatch is attributable.
	tables, ok := context["standard_data_tables"].([]any)
	if !ok || len(tables) != 2 {
		t.Fatalf("expected two standard data tables in context, got %#v", context["standard_data_tables"])
	}

	first, ok := tables[0].(map[string]any)
	if !ok || first["name"] != "preview-road/base-levels" {
		t.Fatalf("standard data tables are not sorted by name: %#v", tables)
	}
}

func TestBuildRunReportReportsUnknownEvidenceTierWithoutProvenance(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()

	report, err := BuildRunReport(BuildOptions{
		BundleDir: bundleDir,
		Project:   project.Project{ProjectID: "proj-4", Name: "NoProvenance"},
		Run:       project.Run{ID: "run-4", ScenarioID: "default", Status: "completed"},
	})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}

	markdown, err := os.ReadFile(report.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}

	markdownText := string(markdown)
	if !strings.Contains(markdownText, "- Evidence tier: "+UnknownEvidenceTier) {
		t.Fatalf("expected the unknown evidence tier row, got: %s", markdownText)
	}

	if strings.Contains(markdownText, "scaffold-tier standards module") {
		t.Fatalf("unexpected scaffold disclosure for an unknown tier: %s", markdownText)
	}

	payload, err := os.ReadFile(report.ContextPath)
	if err != nil {
		t.Fatalf("read report context: %v", err)
	}

	var context map[string]any

	err = json.Unmarshal(payload, &context)
	if err != nil {
		t.Fatalf("decode context json: %v", err)
	}

	if context["evidence_tier"] != UnknownEvidenceTier {
		t.Fatalf("unexpected evidence tier in context: %#v", context["evidence_tier"])
	}
}

// TestBuildRunReportRendersAbsentStandardDataCleanly covers a run against a
// module that carries no coefficient data at all, such as dummy-freefield. The
// row must say so rather than render blank, which a reader could mistake for a
// missing value rather than an absent one.
func TestBuildRunReportRendersAbsentStandardDataCleanly(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	provenancePath := filepath.Join(bundleDir, "provenance.json")

	err := writeJSONFile(provenancePath, map[string]any{
		"standard": map[string]any{
			"id":      "dummy-freefield",
			"version": "v0",
			"profile": "default",
		},
		"metadata":     map[string]string{"evidence_tier": "test-fixture"},
		"input_hashes": map[string]string{},
	})
	if err != nil {
		t.Fatalf("write provenance: %v", err)
	}

	report, err := BuildRunReport(BuildOptions{
		BundleDir:      bundleDir,
		Project:        project.Project{ProjectID: "proj-5", Name: "NoStandardData"},
		Run:            project.Run{ID: "run-5", ScenarioID: "default", Status: "completed"},
		ProvenancePath: provenancePath,
	})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}

	markdown, err := os.ReadFile(report.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}

	markdownText := string(markdown)
	if !strings.Contains(markdownText, "- Standard data digest: "+NoStandardData) {
		t.Fatalf("expected the absent standard data row, got: %s", markdownText)
	}

	if strings.Contains(markdownText, "- Standard data tables:") {
		t.Fatalf("unexpected standard data table list for a module carrying none: %s", markdownText)
	}

	html, err := os.ReadFile(report.HTMLPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}

	if !strings.Contains(string(html), "<li>Standard data digest: <code>"+NoStandardData+"</code></li>") {
		t.Fatalf("expected the absent standard data row in html: %s", html)
	}

	payload, err := os.ReadFile(report.ContextPath)
	if err != nil {
		t.Fatalf("read report context: %v", err)
	}

	var context map[string]any

	err = json.Unmarshal(payload, &context)
	if err != nil {
		t.Fatalf("decode context json: %v", err)
	}

	if context["standard_data_digest"] != NoStandardData {
		t.Fatalf("unexpected standard data digest in context: %#v", context["standard_data_digest"])
	}

	tables, ok := context["standard_data_tables"].([]any)
	if !ok || len(tables) != 0 {
		t.Fatalf("expected an empty standard data table list, got %#v", context["standard_data_tables"])
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
	if !strings.Contains(text, "acceptance-baseline") {
		t.Fatalf("expected default QA suite row, got: %s", text)
	}
}

func TestBuildRunReportCompilesPDFWhenRequested(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	compiler := &stubPDFCompiler{}
	generatedAt := time.Unix(300, 0)

	report, err := BuildRunReport(BuildOptions{
		BundleDir:   bundleDir,
		Project:     project.Project{ProjectID: "proj-3", Name: "PDFDemo", CRS: "EPSG:25832"},
		Run:         project.Run{ID: "run-3", ScenarioID: "default", Status: "completed"},
		GeneratedAt: generatedAt,
		GeneratePDF: true,
		PDFCompiler: compiler,
	})
	if err != nil {
		t.Fatalf("build pdf report: %v", err)
	}

	assertFileContents(t, report.PDFPath, "%PDF-stub")

	if compiler.compileCalls != 1 {
		t.Fatalf("expected one compile call, got %d", compiler.compileCalls)
	}

	if compiler.options == nil {
		t.Fatal("expected compile options to be captured")
	}

	if compiler.options.Format != typst.OutputFormatPDF {
		t.Fatalf("unexpected format: %v", compiler.options.Format)
	}

	if !compiler.options.IgnoreSystemFonts {
		t.Fatal("expected deterministic embedded-font mode")
	}

	if compiler.options.Jobs != 1 {
		t.Fatalf("expected single-job compilation, got %d", compiler.options.Jobs)
	}

	if !compiler.options.CreationTime.Equal(generatedAt.UTC()) {
		t.Fatalf("unexpected creation time: %s", compiler.options.CreationTime)
	}

	if !strings.Contains(compiler.input, "\"TemplateVersion\": \"report-pdf-v1\"") {
		t.Fatalf("expected template version in typst input: %s", compiler.input)
	}
}

type stubPDFCompiler struct {
	compileCalls int
	input        string
	options      *typst.OptionsCompile
}

func (s *stubPDFCompiler) Compile(input io.Reader, output io.Writer, options *typst.OptionsCompile) error {
	s.compileCalls++
	s.options = options

	payload, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	s.input = string(payload)

	_, err = bytes.NewBufferString("%PDF-stub").WriteTo(output)

	return err
}

func assertFileContents(t *testing.T, path string, expected string) {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}

	if string(payload) != expected {
		t.Fatalf("unexpected file contents for %s: %q", path, string(payload))
	}
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	encoded = append(encoded, '\n')

	return os.WriteFile(path, encoded, 0o600)
}
