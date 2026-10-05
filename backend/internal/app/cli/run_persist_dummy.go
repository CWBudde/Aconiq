package cli

import (
	"os"
	"path/filepath"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/framework"
)

// buildDummyReceiverTable maps engine results onto the receiver set. Every
// receiver must have a result; a missing one is an internal inconsistency.
func buildDummyReceiverTable(receivers []geo.PointReceiver, levelByReceiver map[string]float64, indicator string) (results.ReceiverTable, error) {
	indicators := []string{indicator}
	table := results.ReceiverTable{
		IndicatorOrder: indicators,
		Units:          results.UniformUnits(indicators, dummyResultUnit),
		Records:        make([]results.ReceiverRecord, 0, len(receivers)),
	}

	for _, receiver := range receivers {
		level, ok := levelByReceiver[receiver.ID]
		if !ok {
			return results.ReceiverTable{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "missing result for receiver "+receiver.ID, nil)
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

	return table, nil
}

// persistDummyRaster writes the grid raster. Receivers are laid out row-major
// in the order the grid produced them.
func persistDummyRaster(
	resultsDir string,
	receivers []geo.PointReceiver,
	levelByReceiver map[string]float64,
	layout results.GridLayout,
	indicator string,
) (results.RasterPersistence, error) {
	bands := []string{indicator}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     layout.Width,
		Height:    layout.Height,
		Bands:     1,
		NoData:    -9999,
		Units:     results.UniformUnits(bands, dummyResultUnit),
		BandNames: bands,
		CRS:       layout.CRS,
		Geo:       layout.Geo,
	})
	if err != nil {
		return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "build raster", err)
	}

	for receiverIndex, receiver := range receivers {
		err := raster.SetReceiver(layout, receiverIndex, levelByReceiver[receiver.ID])
		if err != nil {
			return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "set raster value", err)
		}
	}

	rasterBasePath := filepath.Join(resultsDir, strings.ToLower(indicator))

	rasterPersistence, err := results.SaveRaster(rasterBasePath, raster)
	if err != nil {
		return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save raster", err)
	}

	return rasterPersistence, nil
}

func persistDummyRunOutputs(
	runDir string,
	runOutput engine.RunOutput,
	receivers []geo.PointReceiver,
	layout results.GridLayout,
	indicator string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, error) {
	resultsDir := filepath.Join(runDir, "results")
	layout = withComputeCRS(layout, projection)

	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "create results directory "+resultsDir, err)
	}

	levelByReceiver := make(map[string]float64, len(runOutput.Results))
	for _, receiverResult := range runOutput.Results {
		levelByReceiver[receiverResult.ReceiverID] = receiverResult.LevelDB
	}

	table, err := buildDummyReceiverTable(receivers, levelByReceiver, indicator)
	if err != nil {
		return persistedRunOutputs{}, err
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

	// The engine reports the source count it actually chunked; the test fixture
	// versions no model of its own, so it contributes no model version.
	sourceCount, _ := runOutput.Metadata["source_count"].(int)

	summary := newRunSummary(runDir, runOutput.OutputHash, "", receiverModeAutoGrid, tier, projection, sourceCount, len(receivers))
	summary["total_chunks"] = runOutput.TotalChunks
	summary["used_cached_chunks"] = runOutput.UsedCachedChunks

	if layout.Width <= 0 || layout.Height <= 0 {
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

	rasterPersistence, err := persistDummyRaster(resultsDir, receivers, levelByReceiver, layout, indicator)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	summaryPath, err := writeGridRunSummary(resultsDir, summary, layout)
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
