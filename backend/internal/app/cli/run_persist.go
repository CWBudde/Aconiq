package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/results"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func persistReceiverTableOnly(
	resultsDir string,
	table results.ReceiverTable,
	summary map[string]any,
) (persistedRunOutputs, error) {
	err := os.MkdirAll(resultsDir, 0o755)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "create results directory "+resultsDir, err)
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table csv", err)
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		SummaryPath:      summaryPath,
	}, nil
}

//nolint:funlen // The dummy export persists both table and raster outputs in one place for parity with the legacy flow.
func persistDummyRunOutputs(
	runDir string,
	runOutput engine.RunOutput,
	receivers []geo.PointReceiver,
	gridWidth int,
	gridHeight int,
	indicator string,
) (persistedRunOutputs, error) {
	resultsDir := filepath.Join(runDir, "results")

	err := os.MkdirAll(resultsDir, 0o755)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "create results directory "+resultsDir, err)
	}

	levelByReceiver := make(map[string]float64, len(runOutput.Results))
	for _, receiverResult := range runOutput.Results {
		levelByReceiver[receiverResult.ReceiverID] = receiverResult.LevelDB
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{indicator},
		Unit:           dummyResultUnit,
		Records:        make([]results.ReceiverRecord, 0, len(receivers)),
	}
	for _, receiver := range receivers {
		level, ok := levelByReceiver[receiver.ID]
		if !ok {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "missing result for receiver "+receiver.ID, nil)
		}

		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      receiver.ID,
			X:       receiver.Point.X,
			Y:       receiver.Point.Y,
			HeightM: receiver.HeightM,
			Values: map[string]float64{
				indicator: level,
			},
		})
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table csv", err)
	}

	summary := map[string]any{
		"run_id":             runOutput.RunID,
		"status":             runOutput.Status,
		"output_hash":        runOutput.OutputHash,
		"total_chunks":       runOutput.TotalChunks,
		"used_cached_chunks": runOutput.UsedCachedChunks,
		"source_count":       runOutput.Metadata["source_count"],
		"receiver_count":     len(receivers),
		"receiver_mode":      receiverModeAutoGrid,
	}

	if gridWidth <= 0 || gridHeight <= 0 {
		summary["receiver_mode"] = receiverModeCustom

		summaryPath := filepath.Join(resultsDir, "run-summary.json")

		err := writeJSONFile(summaryPath, summary)
		if err != nil {
			return persistedRunOutputs{}, err
		}

		return persistedRunOutputs{
			ReceiverJSONPath: receiverJSONPath,
			ReceiverCSVPath:  receiverCSVPath,
			SummaryPath:      summaryPath,
		}, nil
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     1,
		NoData:    -9999,
		Unit:      dummyResultUnit,
		BandNames: []string{indicator},
	})
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "build raster", err)
	}

	for receiverIndex, receiver := range receivers {
		level := levelByReceiver[receiver.ID]
		x := receiverIndex % gridWidth

		y := receiverIndex / gridWidth

		err := raster.Set(x, y, 0, level)
		if err != nil {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "set raster value", err)
		}
	}

	rasterBasePath := filepath.Join(resultsDir, strings.ToLower(indicator))

	rasterPersistence, err := results.SaveRaster(rasterBasePath, raster)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save raster", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   receiverJSONPath,
		ReceiverCSVPath:    receiverCSVPath,
		RasterMetadataPath: rasterPersistence.MetadataPath,
		RasterDataPath:     rasterPersistence.DataPath,
		SummaryPath:        summaryPath,
	}, nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistCnossosRoadRunOutputs(
	runDir string,
	outputs []cnossosroad.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosRoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "hash cnossos outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosindustry.BuiltinModelVersion,
		"reporting_precision_db": cnossosindustry.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosroad.IndicatorLden, cnossosroad.IndicatorLnight, cnossosroad.IndicatorLday, cnossosroad.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosroad.IndicatorLden: output.Indicators.Lden, cnossosroad.IndicatorLnight: output.Indicators.Lnight, cnossosroad.IndicatorLday: output.Indicators.Lday, cnossosroad.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosroad.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "export cnossos road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistBUBRoadRunOutputs(
	runDir string,
	outputs []bubroad.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashBUBRoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUBRoadRunOutputs", "hash BUB road outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          bubroad.BuiltinModelVersion,
		"reporting_precision_db": bubroad.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{bubroad.IndicatorLden, bubroad.IndicatorLnight, bubroad.IndicatorLday, bubroad.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{bubroad.IndicatorLden: output.Indicators.Lden, bubroad.IndicatorLnight: output.Indicators.Lnight, bubroad.IndicatorLday: output.Indicators.Lday, bubroad.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := bubroad.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUBRoadRunOutputs", "export BUB road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistRLS19RoadRunOutputs(
	runDir string,
	outputs []rls19road.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	sourceOverrideCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashRLS19RoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistRLS19RoadRunOutputs", "hash RLS-19 road outputs", err)
	}

	summary := map[string]any{
		"run_id":       filepath.Base(runDir),
		"status":       project.RunStatusCompleted,
		"output_hash":  outputHash,
		"source_count": sourceCount,
		"sources_with_feature_acoustics_overrides": sourceOverrideCount,
		"receiver_count":         len(outputs),
		"data_pack_version":      rls19road.BuiltinDataPackVersion,
		"reporting_precision_db": rls19road.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{rls19road.IndicatorLrDay: output.Indicators.LrDay, rls19road.IndicatorLrNight: output.Indicators.LrNight}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := rls19road.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistRLS19RoadRunOutputs", "export RLS-19 road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistSchall03RunOutputs(
	runDir string,
	outputs []schall03.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashSchall03Outputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistSchall03RunOutputs", "hash Schall 03 outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          schall03.BuiltinModelVersion,
		"data_pack_version":      schall03.BuiltinDataPackVersion,
		"reporting_precision_db": schall03.ReportingPrecisionDB,
		"band_model":             "octave-63Hz-8000Hz",
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{schall03.IndicatorLrDay: output.Indicators.LrDay, schall03.IndicatorLrNight: output.Indicators.LrNight}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := schall03.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistSchall03RunOutputs", "export Schall 03 results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistCnossosAircraftRunOutputs(
	runDir string,
	outputs []cnossosaircraft.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosAircraftOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosAircraftRunOutputs", "hash cnossos aircraft outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosaircraft.BuiltinModelVersion,
		"reporting_precision_db": cnossosaircraft.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosaircraft.IndicatorLden, cnossosaircraft.IndicatorLnight, cnossosaircraft.IndicatorLday, cnossosaircraft.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosaircraft.IndicatorLden: output.Indicators.Lden, cnossosaircraft.IndicatorLnight: output.Indicators.Lnight, cnossosaircraft.IndicatorLday: output.Indicators.Lday, cnossosaircraft.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosaircraft.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosAircraftRunOutputs", "export cnossos aircraft results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistCnossosRailRunOutputs(
	runDir string,
	outputs []cnossosrail.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosRailOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRailRunOutputs", "hash cnossos rail outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosrail.BuiltinModelVersion,
		"reporting_precision_db": cnossosrail.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosrail.IndicatorLden, cnossosrail.IndicatorLnight, cnossosrail.IndicatorLday, cnossosrail.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosrail.IndicatorLden: output.Indicators.Lden, cnossosrail.IndicatorLnight: output.Indicators.Lnight, cnossosrail.IndicatorLday: output.Indicators.Lday, cnossosrail.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosrail.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRailRunOutputs", "export cnossos rail results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistBUFAircraftRunOutputs(
	runDir string,
	outputs []bufaircraft.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashBUFAircraftOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUFAircraftRunOutputs", "hash buf aircraft outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
		"receiver_mode":  receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{bufaircraft.IndicatorLden, bufaircraft.IndicatorLnight, bufaircraft.IndicatorLday, bufaircraft.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{bufaircraft.IndicatorLden: output.Indicators.Lden, bufaircraft.IndicatorLnight: output.Indicators.Lnight, bufaircraft.IndicatorLday: output.Indicators.Lday, bufaircraft.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := bufaircraft.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUFAircraftRunOutputs", "export buf aircraft results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistBEBExposureRunOutputs(
	runDir string,
	outputs []bebexposure.BuildingExposureOutput,
	summary bebexposure.Summary,
	sourceCount int,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	exported, err := bebexposure.ExportResultBundle(resultsDir, outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "export BEB exposure results", err)
	}

	outputHash, err := hashBEBExposureOutputs(outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "hash BEB exposure outputs", err)
	}

	runSummary := map[string]any{
		"run_id":                    filepath.Base(runDir),
		"status":                    project.RunStatusCompleted,
		"output_hash":               outputHash,
		"source_count":              sourceCount,
		"building_count":            len(outputs),
		"estimated_dwellings":       summary.EstimatedDwellings,
		"estimated_persons":         summary.EstimatedPersons,
		"affected_dwellings_lden":   summary.AffectedDwellingsLden,
		"affected_persons_lden":     summary.AffectedPersonsLden,
		"affected_dwellings_lnight": summary.AffectedDwellingsLnight,
		"affected_persons_lnight":   summary.AffectedPersonsLnight,
		"model_version":             bebexposure.BuiltinModelVersion,
		"reporting_precision_db":    bebexposure.ReportingPrecisionCount,
		"occupancy_mode":            summary.OccupancyMode,
		"facade_evaluation_mode":    summary.FacadeEvaluationMode,
		"upstream_mapping_standard": summary.UpstreamMappingStandard,
		"lden_bands":                summary.LdenBands,
		"lnight_bands":              summary.LnightBands,
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, runSummary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   exported.ReceiverJSONPath,
		ReceiverCSVPath:    exported.ReceiverCSVPath,
		RasterMetadataPath: exported.RasterMetaPath,
		RasterDataPath:     exported.RasterDataPath,
		SummaryPath:        summaryPath,
	}, outputHash, nowUTC(), nil
}

//nolint:dupl // Standard-specific export shims intentionally keep each result bundle wiring explicit.
func persistCnossosIndustryRunOutputs(
	runDir string,
	outputs []cnossosindustry.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosIndustryOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosIndustryRunOutputs", "hash cnossos industry outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
		"receiver_mode":  receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosindustry.IndicatorLden, cnossosindustry.IndicatorLnight, cnossosindustry.IndicatorLday, cnossosindustry.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosindustry.IndicatorLden: output.Indicators.Lden, cnossosindustry.IndicatorLnight: output.Indicators.Lnight, cnossosindustry.IndicatorLday: output.Indicators.Lday, cnossosindustry.IndicatorLevening: output.Indicators.Levening}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosindustry.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosIndustryRunOutputs", "export cnossos industry results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistISO9613RunOutputs(
	runDir string,
	outputs []iso9613.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashISO9613Outputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistISO9613RunOutputs", "hash iso9613 outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
		"receiver_mode":  receiverMode,
		"indicator":      iso9613.IndicatorLpAeqDW,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{iso9613.IndicatorLpAeqDW, iso9613.IndicatorLpAeqLT}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{iso9613.IndicatorLpAeqDW: output.Indicators.LpAeqDW, iso9613.IndicatorLpAeqLT: output.Indicators.LpAeqLT}})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := iso9613.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistISO9613RunOutputs", "export iso9613 results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func hashCnossosRoadOutputs(outputs []cnossosroad.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators cnossosroad.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashISO9613Outputs(outputs []iso9613.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                     `json:"receiver_id"`
		Indicators iso9613.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosRailOutputs(outputs []cnossosrail.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators cnossosrail.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBUBRoadOutputs(outputs []bubroad.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                     `json:"receiver_id"`
		Indicators bubroad.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashRLS19RoadOutputs(outputs []rls19road.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                       `json:"receiver_id"`
		Indicators rls19road.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashSchall03Outputs(outputs []schall03.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                      `json:"receiver_id"`
		Indicators schall03.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosAircraftOutputs(outputs []cnossosaircraft.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                             `json:"receiver_id"`
		Indicators cnossosaircraft.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBUFAircraftOutputs(outputs []bufaircraft.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators bufaircraft.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBEBExposureOutputs(outputs []bebexposure.BuildingExposureOutput, summary bebexposure.Summary) (string, error) {
	type record struct {
		BuildingID string                         `json:"building_id"`
		Indicators bebexposure.BuildingIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			BuildingID: output.Building.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(struct {
		Buildings []record            `json:"buildings"`
		Summary   bebexposure.Summary `json:"summary"`
	}{
		Buildings: records,
		Summary:   summary,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosIndustryOutputs(outputs []cnossosindustry.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                             `json:"receiver_id"`
		Indicators cnossosindustry.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func buildRunArtifacts(projectRoot string, runID string, persisted persistedRunOutputs) []project.ArtifactRef {
	now := nowUTC()

	artifacts := make([]project.ArtifactRef, 0, 5)
	if persisted.ReceiverJSONPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-json", runID), RunID: runID, Kind: "run.result.receiver_table_json", Path: relativePath(projectRoot, persisted.ReceiverJSONPath), CreatedAt: now})
	}

	if persisted.ReceiverCSVPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-csv", runID), RunID: runID, Kind: "run.result.receiver_table_csv", Path: relativePath(projectRoot, persisted.ReceiverCSVPath), CreatedAt: now})
	}

	if persisted.RasterMetadataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-meta", runID), RunID: runID, Kind: "run.result.raster_metadata", Path: relativePath(projectRoot, persisted.RasterMetadataPath), CreatedAt: now})
	}

	if persisted.RasterDataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-data", runID), RunID: runID, Kind: "run.result.raster_binary", Path: relativePath(projectRoot, persisted.RasterDataPath), CreatedAt: now})
	}

	if persisted.SummaryPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-summary", runID), RunID: runID, Kind: "run.result.summary", Path: relativePath(projectRoot, persisted.SummaryPath), CreatedAt: now})
	}

	return artifacts
}

func finalizeRunFailure(store projectfs.Store, run project.Run, logLines []string, runErr error) error {
	finishedAt := nowUTC()

	logLines = append(logLines, finishedAt.Format(time.RFC3339)+" run failed")

	err := finalizeRun(store, run, project.RunStatusFailed, finishedAt, logLines, nil)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRunFailure", "finalize failed run", errors.Join(runErr, err))
	}

	return runErr
}

func finalizeRun(
	store projectfs.Store,
	run project.Run,
	status string,
	finishedAt time.Time,
	logLines []string,
	artifacts []project.ArtifactRef,
) error {
	if finishedAt.IsZero() {
		finishedAt = nowUTC()
	}

	proj, err := store.Load()
	if err != nil {
		return err
	}

	foundRun := false

	for i := range proj.Runs {
		if proj.Runs[i].ID != run.ID {
			continue
		}

		proj.Runs[i].Status = status
		proj.Runs[i].FinishedAt = finishedAt
		foundRun = true

		break
	}

	if !foundRun {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", fmt.Sprintf("run %s not found in project manifest", run.ID), nil)
	}

	for _, artifact := range artifacts {
		proj.Artifacts = upsertArtifact(proj.Artifacts, artifact)
	}

	err = store.Save(proj)
	if err != nil {
		return err
	}

	if len(logLines) == 0 {
		logLines = []string{fmt.Sprintf("%s run finalized with status=%s", finishedAt.Format(time.RFC3339), status)}
	}

	logContent := strings.Join(logLines, "\n") + "\n"

	logPath := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))

	err = os.WriteFile(logPath, []byte(logContent), 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", "write run log "+logPath, err)
	}

	return nil
}

// receiverGridCenter computes the centroid of a set of receivers.
func receiverGridCenter(receivers []geo.PointReceiver) (float64, float64) {
	if len(receivers) == 0 {
		return 0, 0
	}

	var sumX, sumY float64

	for _, r := range receivers {
		sumX += r.Point.X
		sumY += r.Point.Y
	}

	n := float64(len(receivers))

	return sumX / n, sumY / n
}

// findArtifactPath returns the path for the artifact with the given ID, or empty string if not found.
func findArtifactPath(proj project.Project, id string) string {
	for _, a := range proj.Artifacts {
		if a.ID == id {
			return a.Path
		}
	}

	return ""
}

// terrainElevationAt queries the terrain model for elevation at (x, y).
// Returns 0 if terrain is nil or the point is outside bounds.
func terrainElevationAt(tm terrain.Model, x, y float64) float64 {
	if tm == nil {
		return 0
	}

	elev, ok := tm.ElevationAt(x, y)
	if !ok {
		return 0
	}

	return elev
}
