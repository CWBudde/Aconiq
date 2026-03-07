package reporting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/report/results"
)

const (
	defaultReportTitle = "Aconiq Run Report"
)

type QASuiteStatus struct {
	Name    string
	Status  string
	Details string
}

type BuildOptions struct {
	BundleDir         string
	Project           project.Project
	Run               project.Run
	ProvenancePath    string
	RunSummaryPath    string
	ReceiverTablePath string
	RasterMetaPaths   []string
	ModelDumpPath     string
	QASuites          []QASuiteStatus
	GeneratedAt       time.Time
}

type GeneratedReport struct {
	ContextPath  string
	MarkdownPath string
	HTMLPath     string
}

type reportContext struct {
	Title string `json:"title"`

	GeneratedAt string `json:"generated_at"`

	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	ProjectCRS  string `json:"project_crs"`

	RunID      string `json:"run_id"`
	RunStatus  string `json:"run_status"`
	ScenarioID string `json:"scenario_id"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`

	SourceCount   string `json:"source_count,omitempty"`
	ReceiverCount string `json:"receiver_count,omitempty"`
	GridWidth     string `json:"grid_width,omitempty"`
	GridHeight    string `json:"grid_height,omitempty"`
	OutputHash    string `json:"output_hash,omitempty"`

	InputFiles      []inputFileView `json:"input_files"`
	ModelSourcePath string          `json:"model_source_path,omitempty"`
	ModelFeatureCnt string          `json:"model_feature_count,omitempty"`
	CountsByKind    []kindCountView `json:"counts_by_kind,omitempty"`

	StandardID      string          `json:"standard_id"`
	StandardVersion string          `json:"standard_version"`
	StandardProfile string          `json:"standard_profile,omitempty"`
	Parameters      []kvPairView    `json:"parameters"`
	Maps            []rasterMapView `json:"maps"`
	ReceiverUnit    string          `json:"receiver_unit,omitempty"`
	Indicators      []indicatorView `json:"indicators"`
	QASuites        []qaSuiteView   `json:"qa_suites"`
	Notes           []string        `json:"notes,omitempty"`
}

type inputFileView struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type kindCountView struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type kvPairView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type rasterMapView struct {
	MetadataPath string `json:"metadata_path"`
	DataPath     string `json:"data_path"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Bands        int    `json:"bands"`
	Unit         string `json:"unit,omitempty"`
	BandNames    string `json:"band_names,omitempty"`
}

type indicatorView struct {
	Indicator string  `json:"indicator"`
	Min       float64 `json:"min"`
	Mean      float64 `json:"mean"`
	Max       float64 `json:"max"`
}

type qaSuiteView struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type provenanceEnvelope struct {
	Standard    project.StandardRef `json:"standard"`
	Parameters  map[string]string   `json:"parameters"`
	InputHashes map[string]string   `json:"input_hashes"`
}

type runSummaryEnvelope struct {
	SourceCount   *int
	ReceiverCount *int
	GridWidth     *int
	GridHeight    *int
	OutputHash    string
}

type modelDumpEnvelope struct {
	SourcePath   string         `json:"source_path"`
	FeatureCount int            `json:"feature_count"`
	CountsByKind map[string]int `json:"counts_by_kind"`
}

type rasterMetaEnvelope struct {
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Bands     int      `json:"bands"`
	Unit      string   `json:"unit"`
	BandNames []string `json:"band_names"`
	DataFile  string   `json:"data_file"`
}

func BuildRunReport(opts BuildOptions) (GeneratedReport, error) {
	if strings.TrimSpace(opts.BundleDir) == "" {
		return GeneratedReport{}, errors.New("bundle directory is required")
	}

	if strings.TrimSpace(opts.Run.ID) == "" {
		return GeneratedReport{}, errors.New("run id is required")
	}

	generatedAt := opts.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	if err := os.MkdirAll(opts.BundleDir, 0o755); err != nil {
		return GeneratedReport{}, fmt.Errorf("create report directory: %w", err)
	}

	ctx, err := buildContext(opts, generatedAt)
	if err != nil {
		return GeneratedReport{}, err
	}

	contextPath := filepath.Join(opts.BundleDir, "report-context.json")
	if err := writeJSON(contextPath, ctx); err != nil {
		return GeneratedReport{}, err
	}

	markdownPath := filepath.Join(opts.BundleDir, "report.md")
	if err := writeMarkdown(markdownPath, ctx); err != nil {
		return GeneratedReport{}, err
	}

	htmlPath := filepath.Join(opts.BundleDir, "report.html")
	if err := writeHTML(htmlPath, ctx); err != nil {
		return GeneratedReport{}, err
	}

	return GeneratedReport{
		ContextPath:  contextPath,
		MarkdownPath: markdownPath,
		HTMLPath:     htmlPath,
	}, nil
}

func buildContext(opts BuildOptions, generatedAt time.Time) (reportContext, error) {
	ctx := reportContext{
		Title:           defaultReportTitle,
		GeneratedAt:     generatedAt.Format(time.RFC3339),
		ProjectID:       opts.Project.ProjectID,
		ProjectName:     opts.Project.Name,
		ProjectCRS:      opts.Project.CRS,
		RunID:           opts.Run.ID,
		RunStatus:       opts.Run.Status,
		ScenarioID:      opts.Run.ScenarioID,
		StartedAt:       formatTime(opts.Run.StartedAt),
		FinishedAt:      formatTime(opts.Run.FinishedAt),
		StandardID:      opts.Run.Standard.ID,
		StandardVersion: opts.Run.Standard.Version,
		StandardProfile: opts.Run.Standard.Profile,
		InputFiles:      make([]inputFileView, 0),
		Parameters:      make([]kvPairView, 0),
		Maps:            make([]rasterMapView, 0),
		Indicators:      make([]indicatorView, 0),
		Notes:           make([]string, 0),
	}

	provenance, hasProvenance, err := loadProvenance(opts.ProvenancePath)
	if err != nil {
		return reportContext{}, err
	}

	if hasProvenance {
		if provenance.Standard.ID != "" {
			ctx.StandardID = provenance.Standard.ID
			ctx.StandardVersion = provenance.Standard.Version
			ctx.StandardProfile = provenance.Standard.Profile
		}

		ctx.InputFiles = inputFilesFromHashes(provenance.InputHashes)
		ctx.Parameters = kvPairsFromMap(provenance.Parameters)
	}

	if len(ctx.Parameters) == 0 {
		ctx.Parameters = []kvPairView{{Key: "(none)", Value: ""}}
	}

	summary, hasSummary, err := loadRunSummary(opts.RunSummaryPath)
	if err != nil {
		return reportContext{}, err
	}

	if hasSummary {
		ctx.SourceCount = optionalIntString(summary.SourceCount)
		ctx.ReceiverCount = optionalIntString(summary.ReceiverCount)
		ctx.GridWidth = optionalIntString(summary.GridWidth)
		ctx.GridHeight = optionalIntString(summary.GridHeight)
		ctx.OutputHash = summary.OutputHash
	}

	modelDump, hasModelDump, err := loadModelDump(opts.ModelDumpPath)
	if err != nil {
		return reportContext{}, err
	}

	if hasModelDump {
		ctx.ModelSourcePath = modelDump.SourcePath
		ctx.ModelFeatureCnt = strconv.Itoa(modelDump.FeatureCount)
		ctx.CountsByKind = kindCountsFromMap(modelDump.CountsByKind)
	}

	table, hasTable, err := loadReceiverTable(opts.ReceiverTablePath)
	if err != nil {
		return reportContext{}, err
	}

	if hasTable {
		ctx.ReceiverUnit = table.Unit

		ctx.Indicators = buildIndicatorStats(table)
		if ctx.ReceiverCount == "" {
			ctx.ReceiverCount = strconv.Itoa(len(table.Records))
		}
	}

	maps, err := loadRasterMaps(opts.BundleDir, opts.RasterMetaPaths)
	if err != nil {
		return reportContext{}, err
	}

	ctx.Maps = maps

	ctx.QASuites = normalizeQASuites(opts.QASuites)
	if len(ctx.Maps) == 0 {
		ctx.Notes = append(ctx.Notes, "No raster map artifacts were found in the export bundle.")
	}

	if len(ctx.Indicators) == 0 {
		ctx.Notes = append(ctx.Notes, "No receiver table statistics were available.")
	}

	if len(ctx.InputFiles) == 0 {
		ctx.Notes = append(ctx.Notes, "No input hashes were found in provenance.")
	}

	return ctx, nil
}

func loadProvenance(path string) (provenanceEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return provenanceEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return provenanceEnvelope{}, false, nil
		}

		return provenanceEnvelope{}, false, fmt.Errorf("read provenance %s: %w", path, err)
	}

	var parsed provenanceEnvelope
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return provenanceEnvelope{}, false, fmt.Errorf("decode provenance %s: %w", path, err)
	}

	return parsed, true, nil
}

func loadRunSummary(path string) (runSummaryEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return runSummaryEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runSummaryEnvelope{}, false, nil
		}

		return runSummaryEnvelope{}, false, fmt.Errorf("read run summary %s: %w", path, err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return runSummaryEnvelope{}, false, fmt.Errorf("decode run summary %s: %w", path, err)
	}

	out := runSummaryEnvelope{
		SourceCount:   optionalInt(parsed["source_count"]),
		ReceiverCount: optionalInt(parsed["receiver_count"]),
		GridWidth:     optionalInt(parsed["grid_width"]),
		GridHeight:    optionalInt(parsed["grid_height"]),
	}
	if hashText, ok := parsed["output_hash"].(string); ok {
		out.OutputHash = strings.TrimSpace(hashText)
	}

	return out, true, nil
}

func loadModelDump(path string) (modelDumpEnvelope, bool, error) {
	if strings.TrimSpace(path) == "" {
		return modelDumpEnvelope{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return modelDumpEnvelope{}, false, nil
		}

		return modelDumpEnvelope{}, false, fmt.Errorf("read model dump %s: %w", path, err)
	}

	var parsed modelDumpEnvelope
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return modelDumpEnvelope{}, false, fmt.Errorf("decode model dump %s: %w", path, err)
	}

	return parsed, true, nil
}

func loadReceiverTable(path string) (results.ReceiverTable, bool, error) {
	if strings.TrimSpace(path) == "" {
		return results.ReceiverTable{}, false, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return results.ReceiverTable{}, false, nil
		}

		return results.ReceiverTable{}, false, fmt.Errorf("read receiver table %s: %w", path, err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		return results.ReceiverTable{}, false, fmt.Errorf("decode receiver table %s: %w", path, err)
	}

	if err := table.Validate(); err != nil {
		return results.ReceiverTable{}, false, fmt.Errorf("validate receiver table %s: %w", path, err)
	}

	return table, true, nil
}

func loadRasterMaps(bundleDir string, metaPaths []string) ([]rasterMapView, error) {
	paths := make([]string, 0, len(metaPaths))
	for _, rawPath := range metaPaths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}

		paths = append(paths, trimmed)
	}

	sort.Strings(paths)

	views := make([]rasterMapView, 0, len(paths))
	for _, metaPath := range paths {
		payload, err := os.ReadFile(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("read raster metadata %s: %w", metaPath, err)
		}

		var meta rasterMetaEnvelope
		if err := json.Unmarshal(payload, &meta); err != nil {
			return nil, fmt.Errorf("decode raster metadata %s: %w", metaPath, err)
		}

		dataPath := meta.DataFile
		if !filepath.IsAbs(dataPath) {
			dataPath = filepath.Join(filepath.Dir(metaPath), meta.DataFile)
		}

		views = append(views, rasterMapView{
			MetadataPath: filepath.ToSlash(relativeFrom(bundleDir, metaPath)),
			DataPath:     filepath.ToSlash(relativeFrom(bundleDir, dataPath)),
			Width:        meta.Width,
			Height:       meta.Height,
			Bands:        meta.Bands,
			Unit:         meta.Unit,
			BandNames:    strings.Join(meta.BandNames, ", "),
		})
	}

	return views, nil
}

func normalizeQASuites(in []QASuiteStatus) []qaSuiteView {
	if len(in) == 0 {
		return []qaSuiteView{
			{
				Name:    "phase20-baseline",
				Status:  "not_configured",
				Details: "No QA suite artifacts were found for this run.",
			},
		}
	}

	out := make([]qaSuiteView, 0, len(in))
	for _, suite := range in {
		name := strings.TrimSpace(suite.Name)
		status := strings.TrimSpace(suite.Status)

		if name == "" {
			continue
		}

		if status == "" {
			status = "unknown"
		}

		out = append(out, qaSuiteView{
			Name:    name,
			Status:  status,
			Details: strings.TrimSpace(suite.Details),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	if len(out) == 0 {
		return []qaSuiteView{
			{
				Name:    "phase20-baseline",
				Status:  "not_configured",
				Details: "No QA suite artifacts were found for this run.",
			},
		}
	}

	return out
}

func inputFilesFromHashes(hashes map[string]string) []inputFileView {
	if len(hashes) == 0 {
		return []inputFileView{}
	}

	keys := make([]string, 0, len(hashes))
	for key := range hashes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]inputFileView, 0, len(keys))
	for _, key := range keys {
		out = append(out, inputFileView{
			Path:   key,
			SHA256: hashes[key],
		})
	}

	return out
}

func kvPairsFromMap(values map[string]string) []kvPairView {
	if len(values) == 0 {
		return []kvPairView{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]kvPairView, 0, len(keys))
	for _, key := range keys {
		out = append(out, kvPairView{Key: key, Value: values[key]})
	}

	return out
}

func kindCountsFromMap(values map[string]int) []kindCountView {
	if len(values) == 0 {
		return []kindCountView{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]kindCountView, 0, len(keys))
	for _, key := range keys {
		out = append(out, kindCountView{Kind: key, Count: values[key]})
	}

	return out
}

func buildIndicatorStats(table results.ReceiverTable) []indicatorView {
	stats := make([]indicatorView, 0, len(table.IndicatorOrder))
	for _, indicator := range table.IndicatorOrder {
		minValue := math.Inf(1)
		maxValue := math.Inf(-1)
		sum := 0.0
		count := 0

		for _, record := range table.Records {
			value := record.Values[indicator]
			if value < minValue {
				minValue = value
			}

			if value > maxValue {
				maxValue = value
			}

			sum += value
			count++
		}

		if count == 0 {
			continue
		}

		stats = append(stats, indicatorView{
			Indicator: indicator,
			Min:       minValue,
			Mean:      sum / float64(count),
			Max:       maxValue,
		})
	}

	return stats
}

func optionalInt(value any) *int {
	switch typed := value.(type) {
	case float64:
		v := int(math.Round(typed))
		return &v
	case int:
		v := typed
		return &v
	case int64:
		v := int(typed)
		return &v
	default:
		return nil
	}
}

func optionalIntString(value *int) string {
	if value == nil {
		return ""
	}

	return strconv.Itoa(*value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "n/a"
	}

	return value.UTC().Format(time.RFC3339)
}

func relativeFrom(baseDir string, fullPath string) string {
	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return fullPath
	}

	return rel
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report json: %w", err)
	}

	encoded = append(encoded, '\n')

	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write report json %s: %w", path, err)
	}

	return nil
}

func writeMarkdown(path string, ctx reportContext) error {
	tmpl, err := texttemplate.New("report-markdown").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("parse markdown template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute markdown template: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write report markdown %s: %w", path, err)
	}

	return nil
}

func writeHTML(path string, ctx reportContext) error {
	tmpl, err := template.New("report-html").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parse html template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute html template: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write report html %s: %w", path, err)
	}

	return nil
}

const markdownTemplate = `# {{.Title}}

Generated: {{.GeneratedAt}}

## Input overview

- Project: {{.ProjectName}} ({{.ProjectID}})
- CRS: {{.ProjectCRS}}
- Run: {{.RunID}} (status={{.RunStatus}})
- Scenario: {{.ScenarioID}}
- Started: {{.StartedAt}}
- Finished: {{.FinishedAt}}
{{if .SourceCount}}- Source count: {{.SourceCount}}{{end}}
{{if .ReceiverCount}}- Receiver count: {{.ReceiverCount}}{{end}}
{{if .GridWidth}}- Grid width: {{.GridWidth}}{{end}}
{{if .GridHeight}}- Grid height: {{.GridHeight}}{{end}}
{{if .OutputHash}}- Output hash: {{.OutputHash}}{{end}}
{{if .ModelFeatureCnt}}- Model features: {{.ModelFeatureCnt}}{{end}}
{{if .ModelSourcePath}}- Model source path: {{.ModelSourcePath}}{{end}}
{{if .CountsByKind}}
- Model counts by kind:
{{range .CountsByKind}}  - {{.Kind}}: {{.Count}}
{{end}}{{end}}
{{if .InputFiles}}
- Input files:
{{range .InputFiles}}  - ` + "`" + `{{.Path}}` + "`" + ` (sha256={{.SHA256}})
{{end}}{{else}}
- Input files: none
{{end}}

## Standard ID + version/profile + parameters

- Standard ID: {{.StandardID}}
- Standard version: {{.StandardVersion}}
- Standard profile: {{.StandardProfile}}
{{if .Parameters}}
- Parameters:
{{range .Parameters}}  - ` + "`" + `{{.Key}}={{.Value}}` + "`" + `
{{end}}{{else}}
- Parameters: none
{{end}}

## Maps/images

{{if .Maps}}
| Metadata | Data | Width | Height | Bands | Unit | Band names |
| --- | --- | ---: | ---: | ---: | --- | --- |
{{range .Maps}}| ` + "`" + `{{.MetadataPath}}` + "`" + ` | ` + "`" + `{{.DataPath}}` + "`" + ` | {{.Width}} | {{.Height}} | {{.Bands}} | {{.Unit}} | {{.BandNames}} |
{{end}}
{{else}}
No map/image artifacts were available for this run export.
{{end}}

## Tables (receiver stats)

{{if .Indicators}}
Unit: {{.ReceiverUnit}}

| Indicator | Min | Mean | Max |
| --- | ---: | ---: | ---: |
{{range .Indicators}}| {{.Indicator}} | {{printf "%.3f" .Min}} | {{printf "%.3f" .Mean}} | {{printf "%.3f" .Max}} |
{{end}}
{{else}}
No receiver statistics were available.
{{end}}

## QA status (which suites passed)

| Suite | Status | Details |
| --- | --- | --- |
{{range .QASuites}}| {{.Name}} | {{.Status}} | {{.Details}} |
{{end}}
{{if .Notes}}
## Notes
{{range .Notes}}- {{.}}
{{end}}{{end}}
`

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light; }
    body {
      font-family: "Source Sans 3", "Segoe UI", sans-serif;
      margin: 2rem auto;
      padding: 0 1rem 3rem;
      max-width: 980px;
      color: #1f2933;
      background: linear-gradient(180deg, #f9fbfd, #ffffff);
    }
    h1, h2 { color: #102a43; }
    .meta { color: #486581; margin-bottom: 2rem; }
    table {
      border-collapse: collapse;
      width: 100%;
      margin: 0.75rem 0 1.5rem;
      font-size: 0.95rem;
    }
    th, td {
      border: 1px solid #d9e2ec;
      text-align: left;
      padding: 0.45rem 0.6rem;
      vertical-align: top;
    }
    th { background: #f0f4f8; }
    code {
      font-family: "JetBrains Mono", "Cascadia Mono", monospace;
      font-size: 0.9em;
    }
    ul { margin-top: 0.5rem; }
  </style>
</head>
<body>
  <h1>{{.Title}}</h1>
  <p class="meta">Generated: {{.GeneratedAt}}</p>

  <h2>Input overview</h2>
  <ul>
    <li>Project: {{.ProjectName}} ({{.ProjectID}})</li>
    <li>CRS: {{.ProjectCRS}}</li>
    <li>Run: {{.RunID}} (status={{.RunStatus}})</li>
    <li>Scenario: {{.ScenarioID}}</li>
    <li>Started: {{.StartedAt}}</li>
    <li>Finished: {{.FinishedAt}}</li>
    {{if .SourceCount}}<li>Source count: {{.SourceCount}}</li>{{end}}
    {{if .ReceiverCount}}<li>Receiver count: {{.ReceiverCount}}</li>{{end}}
    {{if .GridWidth}}<li>Grid width: {{.GridWidth}}</li>{{end}}
    {{if .GridHeight}}<li>Grid height: {{.GridHeight}}</li>{{end}}
    {{if .OutputHash}}<li>Output hash: {{.OutputHash}}</li>{{end}}
    {{if .ModelFeatureCnt}}<li>Model features: {{.ModelFeatureCnt}}</li>{{end}}
    {{if .ModelSourcePath}}<li>Model source path: {{.ModelSourcePath}}</li>{{end}}
  </ul>
  {{if .CountsByKind}}
  <table>
    <thead><tr><th>Model kind</th><th>Count</th></tr></thead>
    <tbody>{{range .CountsByKind}}<tr><td>{{.Kind}}</td><td>{{.Count}}</td></tr>{{end}}</tbody>
  </table>
  {{end}}
  {{if .InputFiles}}
  <table>
    <thead><tr><th>Input path</th><th>SHA-256</th></tr></thead>
    <tbody>{{range .InputFiles}}<tr><td><code>{{.Path}}</code></td><td><code>{{.SHA256}}</code></td></tr>{{end}}</tbody>
  </table>
  {{else}}
  <p>No input hashes were available.</p>
  {{end}}

  <h2>Standard ID + version/profile + parameters</h2>
  <ul>
    <li>Standard ID: {{.StandardID}}</li>
    <li>Standard version: {{.StandardVersion}}</li>
    <li>Standard profile: {{.StandardProfile}}</li>
  </ul>
  <table>
    <thead><tr><th>Parameter</th><th>Value</th></tr></thead>
    <tbody>{{range .Parameters}}<tr><td><code>{{.Key}}</code></td><td><code>{{.Value}}</code></td></tr>{{end}}</tbody>
  </table>

  <h2>Maps/images</h2>
  {{if .Maps}}
  <table>
    <thead><tr><th>Metadata</th><th>Data</th><th>Width</th><th>Height</th><th>Bands</th><th>Unit</th><th>Band names</th></tr></thead>
    <tbody>
    {{range .Maps}}
      <tr>
        <td><code>{{.MetadataPath}}</code></td>
        <td><code>{{.DataPath}}</code></td>
        <td>{{.Width}}</td>
        <td>{{.Height}}</td>
        <td>{{.Bands}}</td>
        <td>{{.Unit}}</td>
        <td>{{.BandNames}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}
  <p>No map/image artifacts were available for this run export.</p>
  {{end}}

  <h2>Tables (receiver stats)</h2>
  {{if .Indicators}}
  <p>Unit: {{.ReceiverUnit}}</p>
  <table>
    <thead><tr><th>Indicator</th><th>Min</th><th>Mean</th><th>Max</th></tr></thead>
    <tbody>{{range .Indicators}}<tr><td>{{.Indicator}}</td><td>{{printf "%.3f" .Min}}</td><td>{{printf "%.3f" .Mean}}</td><td>{{printf "%.3f" .Max}}</td></tr>{{end}}</tbody>
  </table>
  {{else}}
  <p>No receiver statistics were available.</p>
  {{end}}

  <h2>QA status (which suites passed)</h2>
  <table>
    <thead><tr><th>Suite</th><th>Status</th><th>Details</th></tr></thead>
    <tbody>{{range .QASuites}}<tr><td>{{.Name}}</td><td>{{.Status}}</td><td>{{.Details}}</td></tr>{{end}}</tbody>
  </table>

  {{if .Notes}}
  <h2>Notes</h2>
  <ul>{{range .Notes}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
</body>
</html>
`
