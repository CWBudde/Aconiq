package reporting

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/aconiq/backend/internal/atomicfile"
)

func writeHTML(path string, ctx reportContext) error {
	tmpl, err := template.New("report-html").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parse html template: %w", err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return fmt.Errorf("execute html template: %w", err)
	}

	err = atomicfile.WriteFile(path, buf.Bytes())
	if err != nil {
		return fmt.Errorf("write report html %s: %w", path, err)
	}

	return nil
}

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
    {{if .ParkingSourceCount}}<li>Parking source count: {{.ParkingSourceCount}}</li>{{end}}
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
    <li>Standard context: {{.StandardContext}}</li>
    <li>Standard version: {{.StandardVersion}}</li>
    <li>Standard profile: {{.StandardProfile}}</li>
    <li>Evidence tier: {{.EvidenceTier}}</li>
    <li>Standard data digest: <code>{{.StandardDataDigest}}</code></li>
  </ul>
  {{if .StandardDataTables}}
  <table>
    <thead><tr><th>Standard data table</th><th>SHA-256</th></tr></thead>
    <tbody>{{range .StandardDataTables}}<tr><td><code>{{.Name}}</code></td><td><code>{{.Digest}}</code></td></tr>{{end}}</tbody>
  </table>
  {{end}}
  <table>
    <thead><tr><th>Parameter</th><th>Value</th></tr></thead>
    <tbody>{{range .Parameters}}<tr><td><code>{{.Key}}</code></td><td><code>{{.Value}}</code></td></tr>{{end}}</tbody>
  </table>

  <h2>Maps/images</h2>
  {{if .Maps}}
  <table>
    <thead><tr><th>Metadata</th><th>Data</th><th>Width</th><th>Height</th><th>Bands</th><th>Band names</th></tr></thead>
    <tbody>
    {{range .Maps}}
      <tr>
        <td><code>{{.MetadataPath}}</code></td>
        <td><code>{{.DataPath}}</code></td>
        <td>{{.Width}}</td>
        <td>{{.Height}}</td>
        <td>{{.Bands}}</td>
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
  <table>
    <thead><tr><th>Indicator</th><th>Unit</th><th>Min</th><th>Mean</th><th>Max</th></tr></thead>
    <tbody>{{range .Indicators}}<tr><td>{{.Indicator}}</td><td>{{.Unit}}</td><td>{{printf "%.3f" .Min}}</td><td>{{printf "%.3f" .Mean}}</td><td>{{printf "%.3f" .Max}}</td></tr>{{end}}</tbody>
  </table>
  {{else}}
  <p>No receiver statistics were available.</p>
  {{end}}

  {{if .Assessment}}
  <h2>16. BImSchV assessment</h2>
  <ul>
    <li>Law: {{.Assessment.Law}}</li>
    <li>Source standard: {{.Assessment.SourceStandard}}</li>
    <li>Assessed receivers: {{.Assessment.AssessedCount}}</li>
    <li>Receivers with threshold exceedance: {{.Assessment.ExceedingCount}}</li>
    <li>Skipped receivers: {{.Assessment.SkippedCount}}</li>
  </ul>
  {{if .Assessment.Categories}}
  <table>
    <thead><tr><th>Area category</th><th>Count</th></tr></thead>
    <tbody>{{range .Assessment.Categories}}<tr><td>{{.Kind}}</td><td>{{.Count}}</td></tr>{{end}}</tbody>
  </table>
  {{end}}
  {{if .Assessment.ExamplesDE}}
  <ul>{{range .Assessment.ExamplesDE}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
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
