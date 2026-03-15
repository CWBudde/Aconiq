package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	exportfmt "github.com/aconiq/backend/internal/report/export"
	"github.com/aconiq/backend/internal/report/reporting"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/spf13/cobra"
)

type exportSummary struct {
	ExportID            string              `json:"export_id"`
	ProjectID           string              `json:"project_id"`
	ProjectCRS          string              `json:"project_crs,omitempty"`
	RunID               string              `json:"run_id"`
	ExportedAt          time.Time           `json:"exported_at"`
	OutputDirectory     string              `json:"output_directory"`
	CopiedFiles         []string            `json:"copied_files"`
	GeneratedSampleData []string            `json:"generated_sample_data,omitempty"`
	GeneratedReports    []string            `json:"generated_reports,omitempty"`
	ExportedFormats     map[string][]string `json:"exported_formats,omitempty"`
}

type copiedRunResults struct {
	CopiedFiles        []string
	ReceiverTableJSON  string
	RunSummary         string
	RasterMetadataList []string
	ModelDump          string
}

func newExportCommand() *cobra.Command {
	var runID string
	var outDir string
	var targetCRS string
	var emitSampleResults bool
	var skipReport bool
	var generatePDF bool
	var formatList string
	var contourInterval float64

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export run artifacts into a portable bundle with offline report files",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "command state unavailable", nil)
			}

			if skipReport && generatePDF {
				return domainerrors.New(domainerrors.KindUserInput, "cli.export", "--pdf cannot be used together with --skip-report", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			run, err := findRunForExport(proj.Runs, runID)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.export", err.Error(), nil)
			}

			if outDir == "" {
				outDir = filepath.Join(".noise", "exports")
			}

			outRoot := resolvePath(store.Root(), outDir)
			exportID := fmt.Sprintf("%s-%s", run.ID, time.Now().UTC().Format("20060102T150405Z"))

			bundleDir := filepath.Join(outRoot, exportID)

			err = os.MkdirAll(bundleDir, 0o755)
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "create export directory: "+bundleDir, err)
			}

			copiedFiles := make([]string, 0, 12)
			var provenancePath string

			if run.LogPath != "" {
				src := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))
				dst := filepath.Join(bundleDir, "run.log")

				copied, err := copyFileIfExists(src, dst)
				if err != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.export", "copy run log", err)
				}

				if copied {
					copiedFiles = append(copiedFiles, filepath.ToSlash("run.log"))
				}
			}

			if run.ProvenancePath != "" {
				src := filepath.Join(store.Root(), filepath.FromSlash(run.ProvenancePath))
				dst := filepath.Join(bundleDir, "provenance.json")

				copied, err := copyFileIfExists(src, dst)
				if err != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.export", "copy provenance", err)
				}

				if copied {
					copiedFiles = append(copiedFiles, filepath.ToSlash("provenance.json"))
					provenancePath = dst
				}
			}

			copiedResults, err := copyRunResultArtifactsToBundle(store.Root(), bundleDir, proj.Artifacts, run.ID)
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "copy run result artifacts", err)
			}

			copiedFiles = append(copiedFiles, copiedResults.CopiedFiles...)

			modelDumpPath, modelDumpRel, err := copyModelDumpToBundle(store.Root(), bundleDir, proj.Artifacts)
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "copy model dump artifact", err)
			}

			if modelDumpPath != "" {
				copiedFiles = append(copiedFiles, modelDumpRel)
				copiedResults.ModelDump = modelDumpPath
			}

			modelGeoJSONPath, modelGeoJSONRel, err := copyModelGeoJSONToBundle(store.Root(), bundleDir, proj.Artifacts)
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "copy model geojson artifact", err)
			}

			if modelGeoJSONPath != "" {
				copiedFiles = append(copiedFiles, modelGeoJSONRel)
			}

			if targetCRS != "" && modelGeoJSONPath != "" {
				err = reprojectModelGeoJSON(modelGeoJSONPath, proj.CRS, targetCRS)
				if err != nil {
					return domainerrors.New(domainerrors.KindUserInput, "cli.export", "re-project model GeoJSON", err)
				}
			}

			summary := exportSummary{
				ExportID:        exportID,
				ProjectID:       proj.ProjectID,
				ProjectCRS:      proj.CRS,
				RunID:           run.ID,
				ExportedAt:      nowUTC(),
				OutputDirectory: bundleDir,
				CopiedFiles:     dedupeAndSort(copiedFiles),
			}

			reportArtifacts := make([]project.ArtifactRef, 0, 3)

			if !skipReport {
				reportBundle, reportErr := reporting.BuildRunReport(reporting.BuildOptions{
					BundleDir:         bundleDir,
					Project:           proj,
					Run:               run,
					ProvenancePath:    provenancePath,
					RunSummaryPath:    copiedResults.RunSummary,
					ReceiverTablePath: copiedResults.ReceiverTableJSON,
					RasterMetaPaths:   copiedResults.RasterMetadataList,
					ModelDumpPath:     copiedResults.ModelDump,
					QASuites:          collectQASuites(proj.Artifacts, run.ID),
					GeneratedAt:       nowUTC(),
					GeneratePDF:       generatePDF,
				})
				if reportErr != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.export", "build report bundle", reportErr)
				}

				generatedReports := []string{
					relativePath(bundleDir, reportBundle.ContextPath),
					relativePath(bundleDir, reportBundle.MarkdownPath),
					relativePath(bundleDir, reportBundle.HTMLPath),
					relativePath(bundleDir, reportBundle.TypstPath),
				}
				if reportBundle.PDFPath != "" {
					generatedReports = append(generatedReports, relativePath(bundleDir, reportBundle.PDFPath))
				}

				summary.GeneratedReports = dedupeAndSort(generatedReports)

				reportArtifacts = append(
					reportArtifacts,
					project.ArtifactRef{
						ID:        fmt.Sprintf("artifact-export-%s-report-context", exportID),
						RunID:     run.ID,
						Kind:      "export.report_context_json",
						Path:      relativePath(store.Root(), reportBundle.ContextPath),
						CreatedAt: nowUTC(),
					},
					project.ArtifactRef{
						ID:        fmt.Sprintf("artifact-export-%s-report-markdown", exportID),
						RunID:     run.ID,
						Kind:      "export.report_markdown",
						Path:      relativePath(store.Root(), reportBundle.MarkdownPath),
						CreatedAt: nowUTC(),
					},
					project.ArtifactRef{
						ID:        fmt.Sprintf("artifact-export-%s-report-html", exportID),
						RunID:     run.ID,
						Kind:      "export.report_html",
						Path:      relativePath(store.Root(), reportBundle.HTMLPath),
						CreatedAt: nowUTC(),
					},
					project.ArtifactRef{
						ID:        fmt.Sprintf("artifact-export-%s-report-typst", exportID),
						RunID:     run.ID,
						Kind:      "export.report_typst",
						Path:      relativePath(store.Root(), reportBundle.TypstPath),
						CreatedAt: nowUTC(),
					},
				)
				if reportBundle.PDFPath != "" {
					reportArtifacts = append(reportArtifacts, project.ArtifactRef{
						ID:        fmt.Sprintf("artifact-export-%s-report-pdf", exportID),
						RunID:     run.ID,
						Kind:      "export.report_pdf",
						Path:      relativePath(store.Root(), reportBundle.PDFPath),
						CreatedAt: nowUTC(),
					})
				}
			}

			if emitSampleResults {
				generated, err := emitSampleResultBundle(bundleDir)
				if err != nil {
					return err
				}

				summary.GeneratedSampleData = generated
			}

			// Process additional export formats (GeoTIFF, GeoPackage, contours).
			if formatList != "" {
				formats, parseErr := exportfmt.ParseFormats(formatList)
				if parseErr != nil {
					return domainerrors.New(domainerrors.KindUserInput, "cli.export", parseErr.Error(), nil)
				}

				exportedPaths, fmtErr := executeFormatExports(
					formats, bundleDir, proj.CRS,
					copiedResults, contourInterval,
				)
				if fmtErr != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.export", "format export", fmtErr)
				}

				summary.ExportedFormats = exportedPaths
			}

			summaryPath := filepath.Join(bundleDir, "export-summary.json")

			err = writeJSONFile(summaryPath, summary)
			if err != nil {
				return err
			}

			proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
				ID:        fmt.Sprintf("artifact-export-%s-%d", run.ID, time.Now().UTC().UnixNano()),
				RunID:     run.ID,
				Kind:      "export.bundle",
				Path:      relativePath(store.Root(), summaryPath),
				CreatedAt: nowUTC(),
			})

			proj.Artifacts = append(proj.Artifacts, reportArtifacts...)

			err = store.Save(proj)
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

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported run %s to %s\n", run.ID, bundleDir)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Summary: %s\n", summaryPath)
			if emitSampleResults {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sample results generated: %d files\n", len(summary.GeneratedSampleData))
			}

			if len(summary.GeneratedReports) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Report files generated: %d\n", len(summary.GeneratedReports))
			}

			if len(summary.ExportedFormats) > 0 {
				for fmtName, paths := range summary.ExportedFormats {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Format %s: %d files\n", fmtName, len(paths))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to export (defaults to latest run)")
	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".noise", "exports"), "Output directory for export bundles")
	cmd.Flags().StringVar(&targetCRS, "target-crs", "", "Re-project exported model GeoJSON to target CRS (e.g. EPSG:4326)")
	cmd.Flags().BoolVar(&emitSampleResults, "emit-sample-results", false, "Generate sample raster/table outputs in the export bundle")
	cmd.Flags().BoolVar(&skipReport, "skip-report", false, "Skip report generation (by default report.md/report.html/report.typ are generated)")
	cmd.Flags().BoolVar(&generatePDF, "pdf", false, "Compile report.pdf with Typst in addition to the offline report bundle")
	cmd.Flags().StringVar(&formatList, "format", "", "Comma-separated export formats: geotiff, gpkg, contour-geojson, contour-gpkg")
	cmd.Flags().Float64Var(&contourInterval, "contour-interval", exportfmt.DefaultContourInterval, "Contour line interval in dB (default 5)")

	return cmd
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

func copyFileIfExists(srcPath string, dstPath string) (bool, error) {
	_, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	err = os.MkdirAll(filepath.Dir(dstPath), 0o755)
	if err != nil {
		return false, err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return false, err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return false, err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return false, err
	}

	return true, nil
}

func copyRunResultArtifactsToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef, runID string) (copiedRunResults, error) {
	filtered := make([]project.ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.RunID != runID {
			continue
		}

		if !strings.HasPrefix(artifact.Kind, "run.result.") {
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
		case "run.result.receiver_table_json":
			out.ReceiverTableJSON = dstPath
		case "run.result.summary":
			out.RunSummary = dstPath
		case "run.result.raster_metadata":
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
		if artifact.Kind != "model.dump_json" {
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

	out, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal re-projected GeoJSON: %w", err)
	}

	out = append(out, '\n')

	return os.WriteFile(geojsonPath, out, 0o644)
}

func copyModelGeoJSONToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef) (string, string, error) {
	modelGeoJSONPath := ""
	var latestAt time.Time

	for _, artifact := range artifacts {
		if artifact.Kind != "model.normalized_geojson" {
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
	case "run.result.receiver_table_json":
		return filepath.ToSlash(filepath.Join("results", "receivers.json"))
	case "run.result.receiver_table_csv":
		return filepath.ToSlash(filepath.Join("results", "receivers.csv"))
	case "run.result.summary":
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

func executeFormatExports(
	formats []exportfmt.Format,
	bundleDir string,
	projectCRS string,
	copiedResults copiedRunResults,
	contourInterval float64,
) (map[string][]string, error) {
	out := make(map[string][]string)
	epsgCode := 0

	_, _ = fmt.Sscanf(projectCRS, "EPSG:%d", &epsgCode)

	// Load receiver table if available (needed for GeoPackage + geo-transform inference).
	var receiverTable *results.ReceiverTable

	if copiedResults.ReceiverTableJSON != "" {
		table, err := results.LoadReceiverTableJSON(copiedResults.ReceiverTableJSON)
		if err == nil {
			receiverTable = &table
		}
	}

	// Load raster if available (needed for GeoTIFF + contours).
	var raster *results.Raster

	if len(copiedResults.RasterMetadataList) > 0 {
		r, err := results.LoadRaster(copiedResults.RasterMetadataList[0])
		if err == nil {
			raster = r
		}
	}

	// Infer geo-transform from receiver coordinates if we have both receiver table and raster.
	var gt exportfmt.GeoTransform
	var hasGT bool

	if receiverTable != nil && raster != nil {
		meta := raster.Metadata()
		xs := make([]float64, 0, len(receiverTable.Records))
		ys := make([]float64, 0, len(receiverTable.Records))

		for _, r := range receiverTable.Records {
			xs = append(xs, r.X)
			ys = append(ys, r.Y)
		}

		if len(xs) == meta.Width*meta.Height {
			inferred, err := exportfmt.InferGeoTransformFromReceivers(xs, ys, meta.Width, meta.Height)
			if err == nil {
				gt = inferred
				hasGT = true
			}
		}
	}

	formatsDir := filepath.Join(bundleDir, "formats")

	for _, f := range formats {
		switch f {
		case exportfmt.FormatGeoTIFF:
			if raster == nil {
				continue // skip if no raster available
			}

			if !hasGT {
				// Use a default identity transform if we can't infer.
				gt = exportfmt.GeoTransform{
					OriginX: 0, OriginY: float64(raster.Metadata().Height),
					PixelSizeX: 1, PixelSizeY: -1,
				}
			}

			basePath := filepath.Join(formatsDir, "raster")
			paths, err := exportfmt.ExportGeoTIFF(basePath, raster, gt, projectCRS)
			if err != nil {
				return nil, fmt.Errorf("geotiff export: %w", err)
			}

			relPaths := make([]string, len(paths))
			for i, p := range paths {
				relPaths[i] = relativePath(bundleDir, p)
			}

			out[string(exportfmt.FormatGeoTIFF)] = relPaths

		case exportfmt.FormatGeoPackage:
			if receiverTable == nil {
				continue
			}

			gpkgPath := filepath.Join(formatsDir, "receivers.gpkg")

			err := exportfmt.ExportReceiverGeoPackage(gpkgPath, *receiverTable, projectCRS, epsgCode)
			if err != nil {
				return nil, fmt.Errorf("geopackage export: %w", err)
			}

			out[string(exportfmt.FormatGeoPackage)] = []string{relativePath(bundleDir, gpkgPath)}

		case exportfmt.FormatContourGeoJSON:
			if raster == nil {
				continue
			}

			if !hasGT {
				gt = exportfmt.GeoTransform{
					OriginX: 0, OriginY: float64(raster.Metadata().Height),
					PixelSizeX: 1, PixelSizeY: -1,
				}
			}

			contours, err := exportfmt.GenerateContours(raster, gt, exportfmt.ContourOptions{
				Interval: contourInterval,
			})
			if err != nil {
				return nil, fmt.Errorf("contour generation: %w", err)
			}

			contourPath := filepath.Join(formatsDir, "contours.geojson")

			err = exportfmt.ExportContourGeoJSON(contourPath, contours)
			if err != nil {
				return nil, fmt.Errorf("contour geojson export: %w", err)
			}

			out[string(exportfmt.FormatContourGeoJSON)] = []string{relativePath(bundleDir, contourPath)}

		case exportfmt.FormatContourGeoPackage:
			if raster == nil {
				continue
			}

			if !hasGT {
				gt = exportfmt.GeoTransform{
					OriginX: 0, OriginY: float64(raster.Metadata().Height),
					PixelSizeX: 1, PixelSizeY: -1,
				}
			}

			contours, err := exportfmt.GenerateContours(raster, gt, exportfmt.ContourOptions{
				Interval: contourInterval,
			})
			if err != nil {
				return nil, fmt.Errorf("contour generation: %w", err)
			}

			contourGpkgPath := filepath.Join(formatsDir, "contours.gpkg")

			err = exportfmt.ExportContourGeoPackage(contourGpkgPath, contours, projectCRS, epsgCode)
			if err != nil {
				return nil, fmt.Errorf("contour geopackage export: %w", err)
			}

			out[string(exportfmt.FormatContourGeoPackage)] = []string{relativePath(bundleDir, contourGpkgPath)}
		}
	}

	return out, nil
}

func emitSampleResultBundle(bundleDir string) ([]string, error) {
	resultsDir := filepath.Join(bundleDir, "sample-results")

	err := os.MkdirAll(resultsDir, 0o755)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "create sample results directory", err)
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     32,
		Height:    24,
		Bands:     1,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden"},
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

	table := results.ReceiverTable{
		IndicatorOrder: []string{"Lden", "Lnight"},
		Unit:           "dB",
		Records: []results.ReceiverRecord{
			{ID: "rx-001", X: 100, Y: 200, HeightM: 4, Values: map[string]float64{"Lden": 56.3, "Lnight": 47.8}},
			{ID: "rx-002", X: 110, Y: 200, HeightM: 4, Values: map[string]float64{"Lden": 58.1, "Lnight": 49.2}},
			{ID: "rx-003", X: 120, Y: 205, HeightM: 4, Values: map[string]float64{"Lden": 55.4, "Lnight": 46.6}},
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
