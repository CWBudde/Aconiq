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

func newValidateCommand() *cobra.Command {
	var inputPath string
	var writeReport bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate GeoJSON model data for schema and geometry sanity",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.validate", "command state unavailable", nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			if inputPath == "" {
				inputPath = filepath.Join(".noise", "model", "model.normalized.geojson")
			}

			absoluteInput := resolvePath(store.Root(), inputPath)

			payload, err := os.ReadFile(absoluteInput)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.validate", "read input file: "+absoluteInput, err)
			}

			relInput := relativePath(store.Root(), absoluteInput)

			model, err := modelgeojson.Normalize(payload, proj.CRS, relInput)
			if err != nil {
				return domainerrors.New(domainerrors.KindValidation, "cli.validate", "invalid geojson input", err)
			}

			report := modelgeojson.Validate(model)

			if writeReport {
				reportPath := filepath.Join(store.Root(), ".noise", "model", "validation-report.json")

				err := writeJSONFile(reportPath, report)
				if err != nil {
					return err
				}

				proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
					ID:        "artifact-model-validation",
					Kind:      "model.validation_report",
					Path:      relativePath(store.Root(), reportPath),
					CreatedAt: nowUTC(),
				})

				err = store.Save(proj)
				if err != nil {
					return err
				}
			}

			state.Logger.Info(
				"validation completed",
				"input", relInput,
				"feature_count", len(model.Features),
				"errors", report.ErrorCount(),
				"warnings", report.WarningCount(),
			)

			if state.Config.JSONLogs {
				payload := map[string]any{
					"command":       "validate",
					"input":         relInput,
					"feature_count": len(model.Features),
					"errors":        report.ErrorCount(),
					"warnings":      report.WarningCount(),
				}
				if writeReport {
					reportPath := filepath.Join(store.Root(), ".noise", "model", "validation-report.json")
					payload["report_path"] = relativePath(store.Root(), reportPath)
				}

				writeErr := writeCommandOutput(cmd.OutOrStdout(), true, payload)
				if writeErr != nil {
					return writeErr
				}

				if report.ErrorCount() > 0 {
					messages := make([]string, 0, len(report.Errors))
					for _, issue := range report.Errors {
						messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
					}

					return domainerrors.New(domainerrors.KindValidation, "cli.validate", summarizeValidationErrors(messages, 5), nil)
				}

				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validated %d features from %s\n", len(model.Features), relInput)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Errors: %d\n", report.ErrorCount())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warnings: %d\n", report.WarningCount())

			if report.ErrorCount() > 0 {
				messages := make([]string, 0, len(report.Errors))
				for _, issue := range report.Errors {
					messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
				}

				return domainerrors.New(domainerrors.KindValidation, "cli.validate", summarizeValidationErrors(messages, 5), nil)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "Path to GeoJSON file to validate (defaults to .noise/model/model.normalized.geojson)")
	cmd.Flags().BoolVar(&writeReport, "write-report", true, "Write validation report to .noise/model/validation-report.json")

	return cmd
}
