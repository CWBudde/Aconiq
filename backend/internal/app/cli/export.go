package cli

import (
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
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/reporting"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/spf13/cobra"
)

type exportSummary struct {
	ExportID            string    `json:"export_id"`
	ProjectID           string    `json:"project_id"`
	RunID               string    `json:"run_id"`
	ExportedAt          time.Time `json:"exported_at"`
	OutputDirectory     string    `json:"output_directory"`
	CopiedFiles         []string  `json:"copied_files"`
	GeneratedSampleData []string  `json:"generated_sample_data,omitempty"`
	GeneratedReports    []string  `json:"generated_reports,omitempty"`
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
	var emitSampleResults bool
	var skipReport bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export run artifacts into a portable bundle with offline report files",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.export", "command state unavailable", nil)
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
			if err := os.MkdirAll(bundleDir, 0o755); err != nil {
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

			summary := exportSummary{
				ExportID:        exportID,
				ProjectID:       proj.ProjectID,
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
				})
				if reportErr != nil {
					return domainerrors.New(domainerrors.KindInternal, "cli.export", "build report bundle", reportErr)
				}

				summary.GeneratedReports = dedupeAndSort([]string{
					relativePath(bundleDir, reportBundle.ContextPath),
					relativePath(bundleDir, reportBundle.MarkdownPath),
					relativePath(bundleDir, reportBundle.HTMLPath),
				})
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
				)
			}

			if emitSampleResults {
				generated, err := emitSampleResultBundle(bundleDir)
				if err != nil {
					return err
				}

				summary.GeneratedSampleData = generated
			}

			summaryPath := filepath.Join(bundleDir, "export-summary.json")
			if err := writeJSONFile(summaryPath, summary); err != nil {
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
			if err := store.Save(proj); err != nil {
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

			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to export (defaults to latest run)")
	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".noise", "exports"), "Output directory for export bundles")
	cmd.Flags().BoolVar(&emitSampleResults, "emit-sample-results", false, "Generate sample raster/table outputs in the export bundle")
	cmd.Flags().BoolVar(&skipReport, "skip-report", false, "Skip report generation (by default report.md/report.html are generated)")

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
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
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

	if _, err := io.Copy(dst, src); err != nil {
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

func emitSampleResultBundle(bundleDir string) ([]string, error) {
	resultsDir := filepath.Join(bundleDir, "sample-results")
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
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

	if err := results.SaveReceiverTableJSON(jsonPath, table); err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver json", err)
	}

	if err := results.SaveReceiverTableCSV(csvPath, table); err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver csv", err)
	}

	return []string{
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.MetadataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.DataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(jsonPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(csvPath))),
	}, nil
}
