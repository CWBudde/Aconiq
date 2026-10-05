package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	exportfmt "github.com/aconiq/backend/internal/report/export"
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
func finishExportBundle(ctx context.Context, in finishExportInputs, summary *exportSummary) (string, error) {
	formatArtifacts, err := applyOptionalExportOutputs(
		ctx, summary, in.opts, in.bundleDir, in.proj.CRS, in.staged,
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

	summaryPath, err := finishExportBundle(cmd.Context(), finishExportInputs{
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

// applyOptionalExportOutputs handles the optional sample-result bundle and the
// additional export formats (GeoTIFF, GeoPackage, contours).
func applyOptionalExportOutputs(
	ctx context.Context,
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
		ctx, formats, bundleDir, projectCRS, resultsCRS,
		staged.runResults, opts.contourInterval,
		staged.modelGeoJSONPath,
	)
	if fmtErr != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.export", "format export", fmtErr)
	}

	summary.ExportedFormats = exportedPaths

	return formatExportArtifacts(exportedPaths, bundleDir, refs), nil
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
