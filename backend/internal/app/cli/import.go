package cli

import (
	"fmt"
	"os"
	"path/filepath"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

func newImportCommand() *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import GeoJSON model data into the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.import", "command state unavailable", nil)
			}
			if inputPath == "" {
				return domainerrors.New(domainerrors.KindUserInput, "cli.import", "--input is required", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			absoluteInput := resolvePath(store.Root(), inputPath)
			payload, err := os.ReadFile(absoluteInput)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.import", fmt.Sprintf("read input file: %s", absoluteInput), err)
			}

			relInput := relativePath(store.Root(), absoluteInput)
			model, err := modelgeojson.Normalize(payload, proj.CRS, relInput)
			if err != nil {
				return domainerrors.New(domainerrors.KindValidation, "cli.import", "invalid geojson input", err)
			}

			report := modelgeojson.Validate(model)
			if report.ErrorCount() > 0 {
				messages := make([]string, 0, len(report.Errors))
				for _, issue := range report.Errors {
					messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
				}
				return domainerrors.New(domainerrors.KindValidation, "cli.import", summarizeValidationErrors(messages, 3), nil)
			}

			modelDir := filepath.Join(store.Root(), ".noise", "model")
			normalizedPath := filepath.Join(modelDir, "model.normalized.geojson")
			dumpPath := filepath.Join(modelDir, "model.dump.json")
			reportPath := filepath.Join(modelDir, "validation-report.json")

			if err := writeJSONFile(normalizedPath, model.ToFeatureCollection()); err != nil {
				return err
			}
			if err := writeJSONFile(dumpPath, model.ToDump()); err != nil {
				return err
			}
			if err := writeJSONFile(reportPath, report); err != nil {
				return err
			}

			now := nowUTC()
			proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
				ID:        "artifact-model-normalized",
				Kind:      "model.normalized_geojson",
				Path:      relativePath(store.Root(), normalizedPath),
				CreatedAt: now,
			})
			proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
				ID:        "artifact-model-dump",
				Kind:      "model.dump_json",
				Path:      relativePath(store.Root(), dumpPath),
				CreatedAt: now,
			})
			proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
				ID:        "artifact-model-validation",
				Kind:      "model.validation_report",
				Path:      relativePath(store.Root(), reportPath),
				CreatedAt: now,
			})

			if err := store.Save(proj); err != nil {
				return err
			}

			state.Logger.Info(
				"import completed",
				"input", relInput,
				"feature_count", len(model.Features),
				"warnings", report.WarningCount(),
				"normalized", relativePath(store.Root(), normalizedPath),
			)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d features from %s\n", len(model.Features), relInput)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Normalized GeoJSON: %s\n", relativePath(store.Root(), normalizedPath))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model dump: %s\n", relativePath(store.Root(), dumpPath))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation report: %s\n", relativePath(store.Root(), reportPath))
			if report.WarningCount() > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation warnings: %d\n", report.WarningCount())
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "Path to GeoJSON input file (required)")

	return cmd
}
