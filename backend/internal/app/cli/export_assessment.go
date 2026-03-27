package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/assessment/bimschv16"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func maybeBuild16BImSchVAssessment(bundleDir, modelGeoJSONPath, receiverTablePath, projectCRS, standardID string, generatedAt time.Time) (string, bool, error) {
	if strings.TrimSpace(modelGeoJSONPath) == "" || strings.TrimSpace(receiverTablePath) == "" {
		return "", false, nil
	}
	if !supports16BImSchVAssessment(standardID) {
		return "", false, nil
	}

	modelPayload, err := os.ReadFile(modelGeoJSONPath)
	if err != nil {
		return "", false, fmt.Errorf("read model geojson: %w", err)
	}

	model, err := modelgeojson.Normalize(modelPayload, projectCRS, filepath.ToSlash(filepath.Base(modelGeoJSONPath)))
	if err != nil {
		return "", false, fmt.Errorf("normalize model geojson: %w", err)
	}

	table, err := results.LoadReceiverTableJSON(receiverTablePath)
	if err != nil {
		return "", false, fmt.Errorf("load receiver table: %w", err)
	}

	envelope, err := bimschv16.BuildExportEnvelope(model, table, standardID, generatedAt)
	if err != nil {
		return "", false, err
	}
	if envelope.AssessedCount == 0 && len(envelope.Skipped) == 0 {
		return "", false, nil
	}

	assessmentDir := filepath.Join(bundleDir, "assessment")
	if err := os.MkdirAll(assessmentDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create assessment directory: %w", err)
	}

	outPath := filepath.Join(assessmentDir, "16bimschv-assessment.json")
	payload, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return "", false, fmt.Errorf("encode 16. BImSchV assessment: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(outPath, payload, 0o600); err != nil {
		return "", false, fmt.Errorf("write 16. BImSchV assessment: %w", err)
	}

	return outPath, true, nil
}

func supports16BImSchVAssessment(standardID string) bool {
	switch standardID {
	case rls19road.StandardID, schall03.StandardID:
		return true
	default:
		return false
	}
}
