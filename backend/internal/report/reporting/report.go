package reporting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/jsonio"
)

const (
	defaultReportTitle = "Aconiq Run Report"

	// ProvenanceEvidenceTierKey is the provenance metadata key under which a run
	// records the evidence tier of the standards module it used.
	ProvenanceEvidenceTierKey = "evidence_tier"

	// UnknownEvidenceTier is reported wherever a run's provenance carries no
	// evidence tier, which is the case for runs written before the tier was
	// disclosed. It is deliberately not one of the tier values: a bundle must
	// never imply a tier it does not know.
	UnknownEvidenceTier = "(unknown)"

	// NoStandardData is reported for a run whose standards module carries no
	// coefficient data at all — dummy-freefield computes from its parameters
	// alone — and for runs written before the digest was recorded. Rendering it
	// keeps the provenance row honest instead of leaving it blank, which a
	// reader could mistake for a missing value rather than an absent one.
	NoStandardData = "(none)"
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
	AssessmentPath    string
	QASuites          []QASuiteStatus
	GeneratedAt       time.Time
	GeneratePDF       bool
	TypstExecutable   string
	PDFCompiler       PDFCompiler
}

type GeneratedReport struct {
	ContextPath  string
	MarkdownPath string
	HTMLPath     string
	TypstPath    string
	PDFPath      string
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

	err := os.MkdirAll(opts.BundleDir, 0o750)
	if err != nil {
		return GeneratedReport{}, fmt.Errorf("create report directory: %w", err)
	}

	ctx, err := buildContext(opts, generatedAt)
	if err != nil {
		return GeneratedReport{}, err
	}

	contextPath := filepath.Join(opts.BundleDir, "report-context.json")

	err = writeJSON(contextPath, ctx)
	if err != nil {
		return GeneratedReport{}, err
	}

	markdownPath := filepath.Join(opts.BundleDir, "report.md")

	err = writeMarkdown(markdownPath, ctx)
	if err != nil {
		return GeneratedReport{}, err
	}

	htmlPath := filepath.Join(opts.BundleDir, "report.html")

	err = writeHTML(htmlPath, ctx)
	if err != nil {
		return GeneratedReport{}, err
	}

	typstPath := filepath.Join(opts.BundleDir, "report.typ")

	err = writeTypst(typstPath, ctx)
	if err != nil {
		return GeneratedReport{}, err
	}

	var pdfPath string
	if opts.GeneratePDF {
		pdfPath = filepath.Join(opts.BundleDir, "report.pdf")

		err = writePDF(pdfPath, ctx, generatedAt, opts)
		if err != nil {
			return GeneratedReport{}, err
		}
	}

	return GeneratedReport{
		ContextPath:  contextPath,
		MarkdownPath: markdownPath,
		HTMLPath:     htmlPath,
		TypstPath:    typstPath,
		PDFPath:      pdfPath,
	}, nil
}

// applyRunSummary copies the run-summary counts into the report context. Every
// count is optional: a summary written before the field existed leaves it empty
// and the row is omitted rather than rendered as a zero.
func applyRunSummary(ctx *reportContext, summary runSummaryEnvelope) {
	ctx.SourceCount = optionalIntString(summary.SourceCount)
	ctx.ParkingSourceCount = optionalIntString(summary.ParkingSourceCount)
	ctx.ReceiverCount = optionalIntString(summary.ReceiverCount)
	ctx.GridWidth = optionalIntString(summary.GridWidth)
	ctx.GridHeight = optionalIntString(summary.GridHeight)
	ctx.OutputHash = summary.OutputHash
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
	encoded, err := jsonio.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode report json: %w", err)
	}

	err = atomicfile.WriteFile(path, encoded)
	if err != nil {
		return fmt.Errorf("write report json %s: %w", path, err)
	}

	return nil
}
