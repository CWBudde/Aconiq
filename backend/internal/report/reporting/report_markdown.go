package reporting

import (
	"bytes"
	"fmt"
	texttemplate "text/template"

	"github.com/aconiq/backend/internal/atomicfile"
)

func writeMarkdown(path string, ctx reportContext) error {
	tmpl, err := texttemplate.New("report-markdown").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("parse markdown template: %w", err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return fmt.Errorf("execute markdown template: %w", err)
	}

	err = atomicfile.WriteFile(path, buf.Bytes())
	if err != nil {
		return fmt.Errorf("write report markdown %s: %w", path, err)
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
{{if .ParkingSourceCount}}- Parking source count: {{.ParkingSourceCount}}{{end}}
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
- Standard context: {{.StandardContext}}
- Standard version: {{.StandardVersion}}
- Standard profile: {{.StandardProfile}}
- Evidence tier: {{.EvidenceTier}}
- Standard data digest: {{.StandardDataDigest}}
{{if .StandardDataTables}}
- Standard data tables:
{{range .StandardDataTables}}  - ` + "`" + `{{.Name}}` + "`" + ` (sha256={{.Digest}})
{{end}}{{end}}{{if .Parameters}}
- Parameters:
{{range .Parameters}}  - ` + "`" + `{{.Key}}={{.Value}}` + "`" + `
{{end}}{{else}}
- Parameters: none
{{end}}

## Maps/images

{{if .Maps}}
| Metadata | Data | Width | Height | Bands | Band names |
| --- | --- | ---: | ---: | ---: | --- |
{{range .Maps}}| ` + "`" + `{{.MetadataPath}}` + "`" + ` | ` + "`" + `{{.DataPath}}` + "`" + ` | {{.Width}} | {{.Height}} | {{.Bands}} | {{.BandNames}} |
{{end}}
{{else}}
No map/image artifacts were available for this run export.
{{end}}

## Tables (receiver stats)

{{if .Indicators}}
| Indicator | Unit | Min | Mean | Max |
| --- | --- | ---: | ---: | ---: |
{{range .Indicators}}| {{.Indicator}} | {{.Unit}} | {{printf "%.3f" .Min}} | {{printf "%.3f" .Mean}} | {{printf "%.3f" .Max}} |
{{end}}
{{else}}
No receiver statistics were available.
{{end}}

{{if .Assessment}}
## 16. BImSchV assessment

- Law: {{.Assessment.Law}}
- Source standard: {{.Assessment.SourceStandard}}
- Assessed receivers: {{.Assessment.AssessedCount}}
- Receivers with threshold exceedance: {{.Assessment.ExceedingCount}}
- Skipped receivers: {{.Assessment.SkippedCount}}
{{if .Assessment.Categories}}
- Area categories:
{{range .Assessment.Categories}}  - {{.Kind}}: {{.Count}}
{{end}}{{end}}
{{if .Assessment.ExamplesDE}}
- German assessment text blocks:
{{range .Assessment.ExamplesDE}}  - {{.}}
{{end}}{{end}}
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
