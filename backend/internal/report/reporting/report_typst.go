package reporting

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	typst "github.com/Dadido3/go-typst"
)

const reportTypstTemplateVersion = "report-pdf-v1"

type PDFCompiler interface {
	Compile(input io.Reader, output io.Writer, options *typst.OptionsCompile) error
}

func writeTypst(path string, ctx reportContext) error {
	source, err := renderTypstSource(ctx)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, source, 0o644)
	if err != nil {
		return fmt.Errorf("write report typst %s: %w", path, err)
	}

	return nil
}

func writePDF(path string, ctx reportContext, generatedAt time.Time, opts BuildOptions) error {
	source, err := renderTypstSource(ctx)
	if err != nil {
		return err
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report pdf %s: %w", path, err)
	}
	defer out.Close()

	compiler := opts.PDFCompiler
	if compiler == nil {
		compiler = typst.CLI{
			ExecutablePath:   opts.TypstExecutable,
			WorkingDirectory: opts.BundleDir,
		}
	}

	err = compiler.Compile(bytes.NewReader(source), out, &typst.OptionsCompile{
		Format:            typst.OutputFormatPDF,
		CreationTime:      generatedAt.UTC(),
		IgnoreSystemFonts: true,
		Jobs:              1,
	})
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("compile report pdf %s: typst executable not found in PATH", path)
		}

		return fmt.Errorf("compile report pdf %s: %w", path, err)
	}

	return nil
}

func renderTypstSource(ctx reportContext) ([]byte, error) {
	var markup bytes.Buffer

	err := typst.InjectValues(&markup, map[string]any{
		"report": ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("inject typst report context: %w", err)
	}

	markup.WriteString(reportTypstTemplate)

	return markup.Bytes(), nil
}

const reportTypstTemplate = `

#let report-title(report) = if report.ProjectName != "" {
  report.Title + " - " + report.ProjectName
} else {
  report.Title
}

#let labeled-list(items) = list(..items)

#let kv-table(rows, first, second) = if rows.len() > 0 {
  table(
    columns: 2,
    table.header([#first], [#second]),
    ..for row in rows {
      ([#row.at(0)], [#row.at(1)])
    },
  )
} else {
  [No entries were available.]
}

#let stat-row(indicator) = [
  #indicator.Indicator: min #indicator.Min, mean #indicator.Mean, max #indicator.Max
]

#let qa-row(suite) = [
  #suite.Name: #suite.Status#if suite.Details != "" { " (" + suite.Details + ")" }
]

#let map-row(item) = [
  #item.MetadataPath -> #item.DataPath, #item.Width x #item.Height, #item.Bands band(s)#if item.Unit != "" { ", unit " + item.Unit }#if item.BandNames != "" { ", names " + item.BandNames }
]

#let template(report) = {
  set page(
    paper: "a4",
    margin: (top: 18mm, bottom: 18mm, left: 18mm, right: 18mm),
  )
  set text(size: 10pt)
  set par(leading: 0.65em)
  set heading(numbering: "1.")
  show heading.where(level: 1): set text(size: 18pt)
  show heading.where(level: 2): set text(size: 12pt)
  show table.cell.where(y: 0): strong
  set table(
    inset: 5pt,
    stroke: (x, y) => if y == 0 {
      (bottom: 0.8pt + black)
    } else {
      (bottom: 0.25pt + luma(220))
    },
  )

  [= #report-title(report)]
  [Generated: #report.GeneratedAt]
  [Template version: #report.TemplateVersion]

  [== Input overview]
  #labeled-list((
    [Project: #report.ProjectName],
    [Project ID: #report.ProjectID],
    [CRS: #report.ProjectCRS],
    [Run: #report.RunID],
    [Run status: #report.RunStatus],
    [Scenario: #report.ScenarioID],
    [Started: #report.StartedAt],
    [Finished: #report.FinishedAt],
    ..if report.SourceCount != "" { ([Source count: #report.SourceCount],) } else { () },
    ..if report.ReceiverCount != "" { ([Receiver count: #report.ReceiverCount],) } else { () },
    ..if report.GridWidth != "" { ([Grid width: #report.GridWidth],) } else { () },
    ..if report.GridHeight != "" { ([Grid height: #report.GridHeight],) } else { () },
    ..if report.OutputHash != "" { ([Output hash: #report.OutputHash],) } else { () },
    ..if report.ModelFeatureCnt != "" { ([Model features: #report.ModelFeatureCnt],) } else { () },
    ..if report.ModelSourcePath != "" { ([Model source path: #report.ModelSourcePath],) } else { () },
  ))

  #if report.CountsByKind.len() > 0 [
    #table(
      columns: 2,
      table.header([Model kind], [Count]),
      ..for entry in report.CountsByKind {
        ([#entry.Kind], [#entry.Count])
      },
    )
  ]

  #if report.InputFiles.len() > 0 [
    #table(
      columns: 2,
      table.header([Input path], [SHA-256]),
      ..for entry in report.InputFiles {
        ([#entry.Path], [#entry.SHA256])
      },
    )
  ] else [
    No input hashes were available.
  ]

  [== Standard ID + version/profile + parameters]
  #labeled-list((
    [Standard ID: #report.StandardID],
    [Standard context: #report.StandardContext],
    [Standard version: #report.StandardVersion],
    [Standard profile: #report.StandardProfile],
  ))

  #kv-table(
    (
      ..for entry in report.Parameters {
        ((entry.Key, entry.Value),)
      },
    ),
    "Parameter",
    "Value",
  )

  [== Maps/images]
  #if report.Maps.len() > 0 [
    #labeled-list((
      ..for item in report.Maps {
        (map-row(item),)
      },
    ))
  ] else [
    No map/image artifacts were available for this run export.
  ]

  [== Tables (receiver stats)]
  #if report.Indicators.len() > 0 [
    #if report.ReceiverUnit != "" [
      Unit: #report.ReceiverUnit
    ]
    #labeled-list((
      ..for indicator in report.Indicators {
        (stat-row(indicator),)
      },
    ))
  ] else [
    No receiver statistics were available.
  ]

  [== QA status (which suites passed)]
  #labeled-list((
    ..for suite in report.QASuites {
      (qa-row(suite),)
    },
  ))

  #if report.Notes.len() > 0 [
    [== Notes]
    #labeled-list((
      ..for note in report.Notes {
        ([#note],)
      },
    ))
  ]
}

#show: doc => template(report)
`
