package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	domainerrors "github.com/soundplan/soundplan/backend/internal/domain/errors"
	"github.com/soundplan/soundplan/backend/internal/domain/project"
	"github.com/soundplan/soundplan/backend/internal/io/projectfs"
	"github.com/soundplan/soundplan/backend/internal/report/results"
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
}

func newExportCommand() *cobra.Command {
	var runID string
	var outDir string
	var emitSampleResults bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export run artifacts into a portable bundle (Phase 6 skeleton)",
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
				return domainerrors.New(domainerrors.KindInternal, "cli.export", fmt.Sprintf("create export directory: %s", bundleDir), err)
			}

			copiedFiles := make([]string, 0, 2)
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
				}
			}

			summary := exportSummary{
				ExportID:        exportID,
				ProjectID:       proj.ProjectID,
				RunID:           run.ID,
				ExportedAt:      nowUTC(),
				OutputDirectory: bundleDir,
				CopiedFiles:     copiedFiles,
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
			if err := store.Save(proj); err != nil {
				return err
			}

			state.Logger.Info(
				"export completed",
				"run_id", run.ID,
				"bundle_dir", bundleDir,
				"copied_files", len(copiedFiles),
				"sample_files", len(summary.GeneratedSampleData),
			)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exported run %s to %s\n", run.ID, bundleDir)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Summary: %s\n", summaryPath)
			if emitSampleResults {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sample results generated: %d files\n", len(summary.GeneratedSampleData))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to export (defaults to latest run)")
	cmd.Flags().StringVar(&outDir, "out", filepath.Join(".noise", "exports"), "Output directory for export bundles")
	cmd.Flags().BoolVar(&emitSampleResults, "emit-sample-results", false, "Generate sample raster/table outputs in the export bundle")

	return cmd
}

func findRunForExport(runs []project.Run, runID string) (project.Run, error) {
	if len(runs) == 0 {
		return project.Run{}, fmt.Errorf("project has no runs to export")
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

	for y := 0; y < raster.Metadata().Height; y++ {
		for x := 0; x < raster.Metadata().Width; x++ {
			value := 45.0 + float64(x)/4.0 + float64(y)/5.0
			if err := raster.Set(x, y, 0, value); err != nil {
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
