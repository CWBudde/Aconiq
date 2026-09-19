package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/jsonio"
	exportfmt "github.com/aconiq/backend/internal/report/export"
	"github.com/aconiq/backend/internal/report/reporting"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/spf13/cobra"
)

type exportSummary struct {
	ExportID             string              `json:"export_id"`
	ProjectID            string              `json:"project_id"`
	ProjectCRS           string              `json:"project_crs,omitempty"`
	RunID                string              `json:"run_id"`
	EvidenceTier         string              `json:"evidence_tier"`
	ExportedAt           time.Time           `json:"exported_at"`
	OutputDirectory      string              `json:"output_directory"`
	CopiedFiles          []string            `json:"copied_files"`
	GeneratedSampleData  []string            `json:"generated_sample_data,omitempty"`
	GeneratedReports     []string            `json:"generated_reports,omitempty"`
	GeneratedAssessments []string            `json:"generated_assessments,omitempty"`
	ExportedFormats      map[string][]string `json:"exported_formats,omitempty"`
}

type copiedRunResults struct {
	CopiedFiles        []string
	ReceiverTableJSON  string
	RunSummary         string
	RasterMetadataList []string
	ModelDump          string
}

// exportOptions carries the parsed `export` command flags.
type exportOptions struct {
	runID             string
	outDir            string
	targetCRS         string
	emitSampleResults bool
	skipReport        bool
	generatePDF       bool
	formatList        string
	contourInterval   float64
}

// stagedExportBundle describes the files copied into a freshly created bundle.
type stagedExportBundle struct {
	copiedFiles      []string
	provenancePath   string
	runResults       copiedRunResults
	modelGeoJSONPath string
}

func newExportCommand() *cobra.Command {
	var opts exportOptions

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export run artifacts into a portable bundle with offline report files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportCommand(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.runID, "run-id", "", "Run ID to export (defaults to latest run)")
	cmd.Flags().StringVar(&opts.outDir, "out", filepath.Join(".noise", "exports"), "Output directory for export bundles")
	cmd.Flags().StringVar(&opts.targetCRS, "target-crs", "", "Re-project exported model GeoJSON to target CRS (e.g. EPSG:4326)")
	cmd.Flags().BoolVar(&opts.emitSampleResults, "emit-sample-results", false, "Generate sample raster/table outputs in the export bundle")
	cmd.Flags().BoolVar(&opts.skipReport, "skip-report", false, "Skip report generation (by default report.md/report.html/report.typ are generated)")
	cmd.Flags().BoolVar(&opts.generatePDF, "pdf", false, "Compile report.pdf with Typst in addition to the offline report bundle")
	cmd.Flags().StringVar(&opts.formatList, "format", "", "Comma-separated export formats: geotiff, cog, gpkg, contour-geojson, contour-gpkg")
	cmd.Flags().Float64Var(&opts.contourInterval, "contour-interval", exportfmt.DefaultContourInterval, "Contour line interval in dB (default 5)")

	return cmd
}

// finishExportInputs is what the tail of an export needs: the optional format
// outputs, and then the one manifest write that records everything.
type finishExportInputs struct {
	store           projectfs.Store
	proj            project.Project
	run             project.Run
	bundleDir       string
	exportID        string
	opts            exportOptions
	staged          stagedExportBundle
	reportArtifacts []project.ArtifactRef
}

// finishExportBundle runs the --format and --emit-sample-results outputs and
// persists the bundle. The format artifacts join the report ones in a single
// manifest write, so a failed save leaves no half-registered bundle.
func finishExportBundle(in finishExportInputs, summary *exportSummary) (string, error) {
	formatArtifacts, err := applyOptionalExportOutputs(
		summary, in.opts, in.bundleDir, in.proj.CRS, in.staged,
		formatArtifactInputs{
			storeRoot: in.store.Root(),
			runID:     in.run.ID,
			exportID:  in.exportID,
			createdAt: nowUTC(),
		},
	)
	if err != nil {
		return "", err
	}

	return persistExportBundle(
		in.store, in.proj, in.run, in.bundleDir, *summary,
		append(in.reportArtifacts, formatArtifacts...),
	)
}

// openExportTarget resolves the project and the run an export is about.
//
// Selecting the run is a user-input failure — the id names a run that is not
// there — while opening the project is not, and the two exit differently.
func openExportTarget(projectPath string, runID string) (projectfs.Store, project.Project, project.Run, error) {
	store, err := projectfs.New(projectPath)
	if err != nil {
		return projectfs.Store{}, project.Project{}, project.Run{}, fmt.Errorf("open project %s: %w", projectPath, err)
	}

	proj, err := store.Load()
	if err != nil {
		return projectfs.Store{}, project.Project{}, project.Run{}, fmt.Errorf("load project manifest: %w", err)
	}

	run, err := findRunForExport(proj.Runs, runID)
	if err != nil {
		return projectfs.Store{}, project.Project{}, project.Run{},
			domainerrors.New(domainerrors.KindUserInput, "cli.export", err.Error(), nil)
	}

	return store, proj, run, nil
}

func runExportCommand(cmd *cobra.Command, opts exportOptions) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.export", "command state unavailable", nil)
	}

	if opts.skipReport && opts.generatePDF {
		return domainerrors.New(domainerrors.KindUserInput, "cli.export", "--pdf cannot be used together with --skip-report", nil)
	}

	store, proj, run, err := openExportTarget(state.Config.ProjectPath, opts.runID)
	if err != nil {
		return err
	}

	if opts.outDir == "" {
		opts.outDir = filepath.Join(".noise", "exports")
	}

	outRoot := resolvePath(store.Root(), opts.outDir)
	exportID := fmt.Sprintf("%s-%s", run.ID, time.Now().UTC().Format("20060102T150405Z"))

	bundleDir := filepath.Join(outRoot, exportID)

	err = os.MkdirAll(bundleDir, 0o750)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.export", "create export directory: "+bundleDir, err)
	}

	staged, err := stageExportBundle(store.Root(), bundleDir, proj, run, opts.targetCRS)
	if err != nil {
		return err
	}

	summary := exportSummary{
		ExportID:        exportID,
		ProjectID:       proj.ProjectID,
		ProjectCRS:      proj.CRS,
		RunID:           run.ID,
		EvidenceTier:    evidenceTierFromProvenance(staged.provenancePath),
		ExportedAt:      nowUTC(),
		OutputDirectory: bundleDir,
		CopiedFiles:     dedupeAndSort(staged.copiedFiles),
	}

	reportArtifacts, err := buildExportReports(exportReportInputs{
		storeRoot: store.Root(),
		bundleDir: bundleDir,
		exportID:  exportID,
		proj:      proj,
		run:       run,
		staged:    staged,
		opts:      opts,
	}, &summary)
	if err != nil {
		return err
	}

	summaryPath, err := finishExportBundle(finishExportInputs{
		store:           store,
		proj:            proj,
		run:             run,
		bundleDir:       bundleDir,
		exportID:        exportID,
		opts:            opts,
		staged:          staged,
		reportArtifacts: reportArtifacts,
	}, &summary)
	if err != nil {
		return err
	}

	state.Logger.Info(
		"export completed",
		"run_id", run.ID,
		"bundle_dir", bundleDir,
		"copied_files", len(summary.CopiedFiles),
		"report_files", len(summary.GeneratedReports),
		"sample_files", len(summary.GeneratedSampleData),
	)

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":      "export",
			"run_id":       run.ID,
			"bundle_dir":   bundleDir,
			"summary_path": summaryPath,
			"summary":      summary,
		})
	}

	writeExportSummaryText(cmd, run.ID, bundleDir, summaryPath, summary, opts.emitSampleResults)

	return nil
}

// evidenceTierFromProvenance reads the evidence tier the run recorded in its own
// provenance metadata, under the same key the generated report reads, so the
// bundle summary can never disagree with the report inside the bundle. The
// bundle summary must state the tier the run actually
// ran at, so it is read from provenance rather than resolved against the current
// registry: a standard that has since been retired, renamed or re-tiered would
// otherwise be reported under a tier the run never used. Every failure path —
// no provenance copied into the bundle, an unreadable or malformed file, or
// provenance predating the disclosure — yields the unknown marker rather than an
// error, because a missing tier must not stop an export.
func evidenceTierFromProvenance(path string) string {
	if strings.TrimSpace(path) == "" {
		return reporting.UnknownEvidenceTier
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return reporting.UnknownEvidenceTier
	}

	var parsed project.ProvenanceManifest

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return reporting.UnknownEvidenceTier
	}

	tier := strings.TrimSpace(parsed.Metadata[reporting.ProvenanceEvidenceTierKey])
	if tier == "" {
		return reporting.UnknownEvidenceTier
	}

	return tier
}

// computeCRSFromProvenance returns the CRS a run's results are expressed in.
//
// The empty string means one thing only: a manifest that decoded and carries
// no compute_crs, which is a run recorded before the key existed and whose
// results are therefore in the project CRS. newFormatExportContext applies
// that fallback.
//
// A manifest that cannot be read or decoded is an error rather than that same
// empty string. Conflating the two would label a geographic run's UTM results
// EPSG:4326 — putting every exported result off the coast of Africa — on the
// strength of an unreadable file, which is the one case where guessing is
// least defensible.
//
// A run with no provenance path at all is not an error: `aconiq export`
// stages one only when the run has one.
func computeCRSFromProvenance(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read provenance %s: %w", path, err)
	}

	var parsed project.ProvenanceManifest

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return "", fmt.Errorf("decode provenance %s: %w", path, err)
	}

	return strings.TrimSpace(parsed.Metadata[provenanceComputeCRSKey]), nil
}

// persistExportBundle writes the export summary and records the bundle plus the
// generated report artifacts in the project manifest.
func persistExportBundle(
	store projectfs.Store,
	proj project.Project,
	run project.Run,
	bundleDir string,
	summary exportSummary,
	reportArtifacts []project.ArtifactRef,
) (string, error) {
	summaryPath := filepath.Join(bundleDir, "export-summary.json")

	err := writeJSONFile(summaryPath, summary)
	if err != nil {
		return "", err
	}

	proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
		ID:        fmt.Sprintf("artifact-export-%s-%d", run.ID, time.Now().UTC().UnixNano()),
		RunID:     run.ID,
		Kind:      project.ArtifactKindExportBundle,
		Path:      relativePath(store.Root(), summaryPath),
		CreatedAt: nowUTC(),
	})

	proj.Artifacts = append(proj.Artifacts, reportArtifacts...)

	err = store.Save(proj)
	if err != nil {
		return "", fmt.Errorf("save project manifest: %w", err)
	}

	return summaryPath, nil
}

// stageExportBundle copies the run log, provenance, result artifacts and model
// exports of a run into the freshly created bundle directory.
func stageExportBundle(
	storeRoot string,
	bundleDir string,
	proj project.Project,
	run project.Run,
	targetCRS string,
) (stagedExportBundle, error) {
	staged := stagedExportBundle{copiedFiles: make([]string, 0, 12)}

	if run.LogPath != "" {
		src := filepath.Join(storeRoot, filepath.FromSlash(run.LogPath))
		dst := filepath.Join(bundleDir, "run.log")

		copied, err := copyFileIfExists(src, dst)
		if err != nil {
			return stagedExportBundle{}, domainerrors.New(domainerrors.KindInternal, "cli.export", "copy run log", err)
		}

		if copied {
			staged.copiedFiles = append(staged.copiedFiles, filepath.ToSlash("run.log"))
		}
	}

	if run.ProvenancePath != "" {
		src := filepath.Join(storeRoot, filepath.FromSlash(run.ProvenancePath))
		dst := filepath.Join(bundleDir, "provenance.json")

		copied, err := copyFileIfExists(src, dst)
		if err != nil {
			return stagedExportBundle{}, domainerrors.New(domainerrors.KindInternal, "cli.export", "copy provenance", err)
		}

		if copied {
			staged.copiedFiles = append(staged.copiedFiles, filepath.ToSlash("provenance.json"))
			staged.provenancePath = dst
		}
	}

	copiedResults, err := copyRunResultArtifactsToBundle(storeRoot, bundleDir, proj.Artifacts, run.ID)
	if err != nil {
		return stagedExportBundle{}, domainerrors.New(domainerrors.KindInternal, "cli.export", "copy run result artifacts", err)
	}

	staged.copiedFiles = append(staged.copiedFiles, copiedResults.CopiedFiles...)

	modelDumpPath, modelDumpRel, err := copyModelDumpToBundle(storeRoot, bundleDir, proj.Artifacts)
	if err != nil {
		return stagedExportBundle{}, domainerrors.New(domainerrors.KindInternal, "cli.export", "copy model dump artifact", err)
	}

	if modelDumpPath != "" {
		staged.copiedFiles = append(staged.copiedFiles, modelDumpRel)
		copiedResults.ModelDump = modelDumpPath
	}

	staged.runResults = copiedResults

	modelGeoJSONPath, modelGeoJSONRel, err := copyModelGeoJSONToBundle(storeRoot, bundleDir, proj.Artifacts)
	if err != nil {
		return stagedExportBundle{}, domainerrors.New(domainerrors.KindInternal, "cli.export", "copy model geojson artifact", err)
	}

	if modelGeoJSONPath != "" {
		staged.copiedFiles = append(staged.copiedFiles, modelGeoJSONRel)
	}

	staged.modelGeoJSONPath = modelGeoJSONPath

	if targetCRS != "" && modelGeoJSONPath != "" {
		err = reprojectModelGeoJSON(modelGeoJSONPath, proj.CRS, targetCRS)
		if err != nil {
			return stagedExportBundle{}, domainerrors.New(domainerrors.KindUserInput, "cli.export", "re-project model GeoJSON", err)
		}
	}

	return staged, nil
}

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

// applyOptionalExportOutputs handles the optional sample-result bundle and the
// additional export formats (GeoTIFF, GeoPackage, contours).
func applyOptionalExportOutputs(
	summary *exportSummary,
	opts exportOptions,
	bundleDir string,
	projectCRS string,
	staged stagedExportBundle,
	refs formatArtifactInputs,
) ([]project.ArtifactRef, error) {
	if opts.emitSampleResults {
		generated, err := emitSampleResultBundle(bundleDir)
		if err != nil {
			return nil, err
		}

		summary.GeneratedSampleData = generated
	}

	if opts.formatList == "" {
		return nil, nil
	}

	formats, parseErr := exportfmt.ParseFormats(opts.formatList)
	if parseErr != nil {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.export", parseErr.Error(), nil)
	}

	resultsCRS, crsErr := computeCRSFromProvenance(staged.provenancePath)
	if crsErr != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.export",
			"read the CRS the run's results are in", crsErr)
	}

	exportedPaths, fmtErr := executeFormatExports(
		formats, bundleDir, projectCRS, resultsCRS,
		staged.runResults, opts.contourInterval,
		staged.modelGeoJSONPath,
	)
	if fmtErr != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.export", "format export", fmtErr)
	}

	summary.ExportedFormats = exportedPaths

	return formatExportArtifacts(exportedPaths, bundleDir, refs), nil
}

// formatArtifactInputs is what an artifact ref needs beyond the file itself.
type formatArtifactInputs struct {
	storeRoot string
	runID     string
	exportID  string
	createdAt time.Time
}

// formatArtifactKinds maps a format name to the artifact kind its files get.
//
// A format missing from this map produces no ref rather than a generic one: a
// consumer switching on `Kind` would render an unlabelled row, and the export
// page's kind table is explicitly required to carry a row per kind.
var formatArtifactKinds = map[string]string{
	string(exportfmt.FormatGeoTIFF):           project.ArtifactKindExportFormatGeoTIFF,
	string(exportfmt.FormatCOG):               project.ArtifactKindExportFormatCOG,
	string(exportfmt.FormatGeoPackage):        project.ArtifactKindExportFormatGeoPackage,
	string(exportfmt.FormatContourGeoJSON):    project.ArtifactKindExportFormatContourGeoJSON,
	string(exportfmt.FormatContourGeoPackage): project.ArtifactKindExportFormatContourGPKG,
}

// formatExportArtifacts turns the written format files into manifest entries.
//
// Until now these files existed only in `export-summary.json`'s
// `exported_formats` — reachable by reading that one file and by nothing else.
// No `ArtifactRef` meant no id, so `GET /api/v1/artifacts/{id}/content` could
// not serve them and the frontend had no URL for a GeoTIFF or a contour set.
//
// The paths arrive bundle-relative, because that is what the summary records;
// an `ArtifactRef.Path` is project-root-relative, so each is rejoined to the
// bundle and re-relativised. One ref per file, not per format: GeoTIFF and COG
// write one file per raster band.
func formatExportArtifacts(
	exportedPaths map[string][]string,
	bundleDir string,
	in formatArtifactInputs,
) []project.ArtifactRef {
	// Map iteration order must not reach the manifest: the artifact list is
	// part of a file this project hashes and diffs.
	formatNames := slices.Sorted(maps.Keys(exportedPaths))

	refs := make([]project.ArtifactRef, 0, len(exportedPaths))

	for _, formatName := range formatNames {
		kind, ok := formatArtifactKinds[formatName]
		if !ok {
			continue
		}

		for index, bundleRelative := range exportedPaths[formatName] {
			absolute := filepath.Join(bundleDir, filepath.FromSlash(bundleRelative))

			refs = append(refs, project.ArtifactRef{
				// The export id makes the ref unique per bundle and the index
				// unique per file within it, so re-exporting one run replaces
				// nothing and collides with nothing.
				ID:        fmt.Sprintf("artifact-export-%s-%s-%d", in.exportID, formatName, index),
				RunID:     in.runID,
				Kind:      kind,
				Path:      relativePath(in.storeRoot, absolute),
				CreatedAt: in.createdAt,
			})
		}
	}

	return refs
}

func writeExportSummaryText(
	cmd *cobra.Command,
	runID string,
	bundleDir string,
	summaryPath string,
	summary exportSummary,
	emitSampleResults bool,
) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Exported run %s to %s\n", runID, bundleDir)

	_, _ = fmt.Fprintf(out, "Summary: %s\n", summaryPath)
	if emitSampleResults {
		_, _ = fmt.Fprintf(out, "Sample results generated: %d files\n", len(summary.GeneratedSampleData))
	}

	if len(summary.GeneratedReports) > 0 {
		_, _ = fmt.Fprintf(out, "Report files generated: %d\n", len(summary.GeneratedReports))
	}

	if len(summary.ExportedFormats) > 0 {
		for fmtName, paths := range summary.ExportedFormats {
			_, _ = fmt.Fprintf(out, "Format %s: %d files\n", fmtName, len(paths))
		}
	}
}

func findRunForExport(runs []project.Run, runID string) (project.Run, error) {
	if len(runs) == 0 {
		return project.Run{}, errors.New("project has no runs to export")
	}

	if runID == "" {
		return runs[len(runs)-1], nil
	}

	for _, run := range runs {
		if run.ID == runID {
			return run, nil
		}
	}

	return project.Run{}, fmt.Errorf("run %q not found", runID)
}

func copyFileIfExists(srcPath string, dstPath string) (copied bool, err error) {
	_, err = os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat export source %s: %w", srcPath, err)
	}

	err = os.MkdirAll(filepath.Dir(dstPath), 0o750)
	if err != nil {
		return false, fmt.Errorf("create export bundle directory %s: %w", filepath.Dir(dstPath), err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return false, fmt.Errorf("open export source %s: %w", srcPath, err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(dstPath)
	if err != nil {
		return false, fmt.Errorf("create export bundle file %s: %w", dstPath, err)
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close export bundle file %s: %w", dstPath, cerr)
		}
	}()

	_, err = io.Copy(dst, src)
	if err != nil {
		return false, fmt.Errorf("copy %s to export bundle: %w", srcPath, err)
	}

	return true, nil
}

func copyRunResultArtifactsToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef, runID string) (copiedRunResults, error) {
	filtered := make([]project.ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.RunID != runID {
			continue
		}

		if !strings.HasPrefix(artifact.Kind, project.ArtifactKindRunResultPrefix) {
			continue
		}

		filtered = append(filtered, artifact)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Kind == filtered[j].Kind {
			return filtered[i].Path < filtered[j].Path
		}

		return filtered[i].Kind < filtered[j].Kind
	})

	out := copiedRunResults{
		CopiedFiles:        make([]string, 0, len(filtered)),
		RasterMetadataList: make([]string, 0, len(filtered)),
	}

	usedTargets := make(map[string]struct{}, len(filtered))
	for _, artifact := range filtered {
		destRel := destinationPathForRunArtifact(artifact)
		destRel = ensureUniqueDestination(destRel, usedTargets)
		usedTargets[destRel] = struct{}{}

		srcPath := filepath.Join(projectRoot, filepath.FromSlash(artifact.Path))
		dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

		copied, err := copyFileIfExists(srcPath, dstPath)
		if err != nil {
			return copiedRunResults{}, err
		}

		if !copied {
			continue
		}

		out.CopiedFiles = append(out.CopiedFiles, filepath.ToSlash(destRel))

		switch artifact.Kind {
		case project.ArtifactKindRunResultReceiverTableJSON:
			out.ReceiverTableJSON = dstPath
		case project.ArtifactKindRunResultSummary:
			out.RunSummary = dstPath
		case project.ArtifactKindRunResultRasterMetadata:
			out.RasterMetadataList = append(out.RasterMetadataList, dstPath)
		}
	}

	out.CopiedFiles = dedupeAndSort(out.CopiedFiles)
	sort.Strings(out.RasterMetadataList)

	return out, nil
}

func copyModelDumpToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef) (string, string, error) {
	modelDumpPath := ""

	var latestAt time.Time

	for _, artifact := range artifacts {
		if artifact.Kind != project.ArtifactKindModelDumpJSON {
			continue
		}

		if modelDumpPath == "" || artifact.CreatedAt.After(latestAt) {
			modelDumpPath = artifact.Path
			latestAt = artifact.CreatedAt
		}
	}

	if modelDumpPath == "" {
		return "", "", nil
	}

	srcPath := filepath.Join(projectRoot, filepath.FromSlash(modelDumpPath))
	destRel := filepath.ToSlash(filepath.Join("model", "model.dump.json"))
	dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

	copied, err := copyFileIfExists(srcPath, dstPath)
	if err != nil {
		return "", "", err
	}

	if !copied {
		return "", "", nil
	}

	return dstPath, destRel, nil
}

// reprojectModelGeoJSON reads a normalized GeoJSON file, re-normalizes it from
// the project CRS into a target CRS, and overwrites the file in place.
func reprojectModelGeoJSON(geojsonPath string, projectCRS string, targetCRS string) error {
	data, err := os.ReadFile(geojsonPath)
	if err != nil {
		return fmt.Errorf("read model GeoJSON: %w", err)
	}

	// Re-normalize: the file is in projectCRS, and we want targetCRS.
	// NormalizeWithCRS(data, targetCRS, projectCRS, ...) will transform from projectCRS → targetCRS.
	model, err := modelgeojson.NormalizeWithCRS(data, targetCRS, projectCRS, "export")
	if err != nil {
		return fmt.Errorf("re-project %s -> %s: %w", projectCRS, targetCRS, err)
	}

	fc := model.ToFeatureCollection()

	out, err := jsonio.Marshal(fc)
	if err != nil {
		return fmt.Errorf("marshal re-projected GeoJSON: %w", err)
	}

	// G703: geojsonPath is a file this function has just read from inside the
	// export bundle the CLI itself laid out; only the bundle root comes from
	// --out, which is the destination the user asked for.
	//nolint:gosec // in-place rewrite of a bundle file the exporter created
	if err := os.WriteFile(geojsonPath, out, 0o600); err != nil {
		return fmt.Errorf("write re-projected GeoJSON %s: %w", geojsonPath, err)
	}

	return nil
}

func copyModelGeoJSONToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef) (string, string, error) {
	modelGeoJSONPath := ""

	var latestAt time.Time

	for _, artifact := range artifacts {
		if artifact.Kind != project.ArtifactKindModelNormalizedGeoJSON {
			continue
		}

		if modelGeoJSONPath == "" || artifact.CreatedAt.After(latestAt) {
			modelGeoJSONPath = artifact.Path
			latestAt = artifact.CreatedAt
		}
	}

	if modelGeoJSONPath == "" {
		return "", "", nil
	}

	srcPath := filepath.Join(projectRoot, filepath.FromSlash(modelGeoJSONPath))
	destRel := filepath.ToSlash(filepath.Join("model", "model.normalized.geojson"))
	dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

	copied, err := copyFileIfExists(srcPath, dstPath)
	if err != nil {
		return "", "", err
	}

	if !copied {
		return "", "", nil
	}

	return dstPath, destRel, nil
}

func destinationPathForRunArtifact(artifact project.ArtifactRef) string {
	switch artifact.Kind {
	case project.ArtifactKindRunResultReceiverTableJSON:
		return filepath.ToSlash(filepath.Join("results", "receivers.json"))
	case project.ArtifactKindRunResultReceiverTableCSV:
		return filepath.ToSlash(filepath.Join("results", "receivers.csv"))
	case project.ArtifactKindRunResultSummary:
		return filepath.ToSlash(filepath.Join("results", "run-summary.json"))
	default:
		return filepath.ToSlash(filepath.Join("results", filepath.Base(artifact.Path)))
	}
}

func ensureUniqueDestination(destRel string, used map[string]struct{}) string {
	if _, exists := used[destRel]; !exists {
		return destRel
	}

	ext := filepath.Ext(destRel)

	base := strings.TrimSuffix(destRel, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))

	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		normalized := filepath.ToSlash(trimmed)
		if _, ok := seen[normalized]; ok {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	sort.Strings(out)

	return out
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

// Indicator names used by the sample result bundle emitted with
// `aconiq export --with-sample`. They must match the indicator names the
// standards modules produce.
const (
	sampleIndicatorLden   = "Lden"
	sampleIndicatorLnight = "Lnight"
)

func emitSampleResultBundle(bundleDir string) ([]string, error) {
	resultsDir := filepath.Join(bundleDir, "sample-results")

	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "create sample results directory", err)
	}

	rasterBands := []string{sampleIndicatorLden}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     32,
		Height:    24,
		Bands:     1,
		NoData:    -9999,
		Units:     results.UniformUnits(rasterBands, results.UnitDecibel),
		BandNames: rasterBands,
	})
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "build sample raster", err)
	}

	for y := range raster.Metadata().Height {
		for x := range raster.Metadata().Width {
			value := 45.0 + float64(x)/4.0 + float64(y)/5.0

			err := raster.Set(x, y, 0, value)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "set sample raster cell", err)
			}
		}
	}

	rasterPaths, err := results.SaveRaster(filepath.Join(resultsDir, "lden-raster"), raster)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample raster", err)
	}

	sampleIndicators := []string{sampleIndicatorLden, sampleIndicatorLnight}
	table := results.ReceiverTable{
		IndicatorOrder: sampleIndicators,
		Units:          results.UniformUnits(sampleIndicators, results.UnitDecibel),
		Records: []results.ReceiverRecord{
			{ID: "rx-001", X: 100, Y: 200, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 56.3, sampleIndicatorLnight: 47.8}},
			{ID: "rx-002", X: 110, Y: 200, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 58.1, sampleIndicatorLnight: 49.2}},
			{ID: "rx-003", X: 120, Y: 205, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 55.4, sampleIndicatorLnight: 46.6}},
		},
	}

	jsonPath := filepath.Join(resultsDir, "receivers.json")
	csvPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(jsonPath, table)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver json", err)
	}

	err = results.SaveReceiverTableCSV(csvPath, table)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver csv", err)
	}

	return []string{
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.MetadataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.DataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(jsonPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(csvPath))),
	}, nil
}

// loadModelFeaturesFromGeoJSON reads a normalized model GeoJSON file and converts
// its features into export.ModelFeature values for GeoPackage export.
func loadModelFeaturesFromGeoJSON(geojsonPath string) ([]exportfmt.ModelFeature, error) {
	data, err := os.ReadFile(geojsonPath)
	if err != nil {
		return nil, fmt.Errorf("read model geojson: %w", err)
	}

	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
			Geometry   struct {
				Type        string `json:"type"`
				Coordinates any    `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}

	err = json.Unmarshal(data, &fc)
	if err != nil {
		return nil, fmt.Errorf("parse model geojson: %w", err)
	}

	out := make([]exportfmt.ModelFeature, 0, len(fc.Features))

	for _, f := range fc.Features {
		mf := exportfmt.ModelFeature{
			GeometryType: f.Geometry.Type,
			Coordinates:  f.Geometry.Coordinates,
		}

		if id, ok := f.Properties["id"].(string); ok {
			mf.ID = id
		}

		if kind, ok := f.Properties["kind"].(string); ok {
			mf.Kind = kind
		}

		if st, ok := f.Properties["source_type"].(string); ok {
			mf.SourceType = st
		}

		if h, ok := f.Properties["height_m"].(float64); ok {
			mf.HeightM = h
		}

		out = append(out, mf)
	}

	return out, nil
}
