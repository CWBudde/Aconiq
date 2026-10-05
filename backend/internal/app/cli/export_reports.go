package cli

import (
	"fmt"
	"sort"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/report/reporting"
)

// exportReportInputs bundles everything the report/assessment stage needs.
type exportReportInputs struct {
	storeRoot string
	bundleDir string
	exportID  string
	proj      project.Project
	run       project.Run
	staged    stagedExportBundle
	opts      exportOptions
}

func buildExportReports(in exportReportInputs, summary *exportSummary) ([]project.ArtifactRef, error) {
	reportArtifacts := make([]project.ArtifactRef, 0, 3)

	assessmentPath, builtAssessment, assessmentErr := maybeBuild16BImSchVAssessment(
		in.bundleDir,
		in.staged.modelGeoJSONPath,
		in.staged.runResults.ReceiverTableJSON,
		in.proj.CRS,
		in.run.Standard.ID,
		nowUTC(),
	)
	if assessmentErr != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.export", "build 16. BImSchV assessment", assessmentErr)
	}

	if builtAssessment {
		summary.GeneratedAssessments = []string{relativePath(in.bundleDir, assessmentPath)}
		reportArtifacts = append(reportArtifacts, project.ArtifactRef{
			ID:        fmt.Sprintf("artifact-export-%s-assessment-16bimschv", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportAssessment16BImSchV,
			Path:      relativePath(in.storeRoot, assessmentPath),
			CreatedAt: nowUTC(),
		})
	}

	if in.opts.skipReport {
		return reportArtifacts, nil
	}

	bundleArtifacts, err := buildReportBundleArtifacts(in, assessmentPath, summary)
	if err != nil {
		return nil, err
	}

	return append(reportArtifacts, bundleArtifacts...), nil
}

func buildReportBundleArtifacts(
	in exportReportInputs,
	assessmentPath string,
	summary *exportSummary,
) ([]project.ArtifactRef, error) {
	reportBundle, reportErr := reporting.BuildRunReport(reporting.BuildOptions{
		BundleDir:         in.bundleDir,
		Project:           in.proj,
		Run:               in.run,
		ProvenancePath:    in.staged.provenancePath,
		RunSummaryPath:    in.staged.runResults.RunSummary,
		ReceiverTablePath: in.staged.runResults.ReceiverTableJSON,
		RasterMetaPaths:   in.staged.runResults.RasterMetadataList,
		ModelDumpPath:     in.staged.runResults.ModelDump,
		AssessmentPath:    assessmentPath,
		QASuites:          collectQASuites(in.proj.Artifacts, in.run.ID),
		GeneratedAt:       nowUTC(),
		GeneratePDF:       in.opts.generatePDF,
	})
	if reportErr != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.export", "build report bundle", reportErr)
	}

	generatedReports := []string{
		relativePath(in.bundleDir, reportBundle.ContextPath),
		relativePath(in.bundleDir, reportBundle.MarkdownPath),
		relativePath(in.bundleDir, reportBundle.HTMLPath),
		relativePath(in.bundleDir, reportBundle.TypstPath),
	}
	if reportBundle.PDFPath != "" {
		generatedReports = append(generatedReports, relativePath(in.bundleDir, reportBundle.PDFPath))
	}

	summary.GeneratedReports = dedupeAndSort(generatedReports)

	artifacts := []project.ArtifactRef{
		{
			ID:        fmt.Sprintf("artifact-export-%s-report-context", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportReportContextJSON,
			Path:      relativePath(in.storeRoot, reportBundle.ContextPath),
			CreatedAt: nowUTC(),
		},
		{
			ID:        fmt.Sprintf("artifact-export-%s-report-markdown", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportReportMarkdown,
			Path:      relativePath(in.storeRoot, reportBundle.MarkdownPath),
			CreatedAt: nowUTC(),
		},
		{
			ID:        fmt.Sprintf("artifact-export-%s-report-html", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportReportHTML,
			Path:      relativePath(in.storeRoot, reportBundle.HTMLPath),
			CreatedAt: nowUTC(),
		},
		{
			ID:        fmt.Sprintf("artifact-export-%s-report-typst", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportReportTypst,
			Path:      relativePath(in.storeRoot, reportBundle.TypstPath),
			CreatedAt: nowUTC(),
		},
	}

	if reportBundle.PDFPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{
			ID:        fmt.Sprintf("artifact-export-%s-report-pdf", in.exportID),
			RunID:     in.run.ID,
			Kind:      project.ArtifactKindExportReportPDF,
			Path:      relativePath(in.storeRoot, reportBundle.PDFPath),
			CreatedAt: nowUTC(),
		})
	}

	return artifacts, nil
}

func collectQASuites(artifacts []project.ArtifactRef, runID string) []reporting.QASuiteStatus {
	suites := make([]reporting.QASuiteStatus, 0)

	for _, artifact := range artifacts {
		if artifact.RunID != runID {
			continue
		}

		if !strings.HasPrefix(artifact.Kind, "qa.") {
			continue
		}

		name := strings.TrimPrefix(artifact.Kind, "qa.")

		status := "unknown"
		if strings.Contains(name, ".passed") {
			status = "passed"
		}

		if strings.Contains(name, ".failed") {
			status = "failed"
		}

		suites = append(suites, reporting.QASuiteStatus{
			Name:    name,
			Status:  status,
			Details: "artifact=" + artifact.Path,
		})
	}

	sort.Slice(suites, func(i, j int) bool {
		return suites[i].Name < suites[j].Name
	})

	return suites
}
