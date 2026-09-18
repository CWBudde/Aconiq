package acoustics

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/report/results"
)

// ExportOutputs describes written files for receiver table and raster output.
type ExportOutputs struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
}

// ExportENDBundle writes the receiver table and the two-band raster an END run
// publishes: receivers.json, receivers.csv, and an Lden/Lnight raster named
// after the standard that produced it.
//
// standardID names the raster files, which is the only thing that differed
// between the eight copies this replaces. Everything else — the indicator
// order, the band layout, the no-data value and the validation — is the same
// for every module reporting the END set, and has to be: a consumer reading two
// bundles must be able to compare them.
func ExportENDBundle(baseDir string, standardID string, outputs []ReceiverOutput, grid results.GridLayout) (ExportOutputs, error) {
	if baseDir == "" {
		return ExportOutputs{}, errors.New("base dir is required")
	}

	if standardID == "" {
		return ExportOutputs{}, errors.New("standard id is required")
	}

	if len(outputs) == 0 {
		return ExportOutputs{}, errors.New("at least one receiver output is required")
	}

	if grid.Width <= 0 || grid.Height <= 0 {
		return ExportOutputs{}, errors.New("grid dimensions must be > 0")
	}

	if grid.Width*grid.Height != len(outputs) {
		return ExportOutputs{}, fmt.Errorf("grid dimensions (%dx%d) do not match receiver output count (%d)", grid.Width, grid.Height, len(outputs))
	}

	err := os.MkdirAll(baseDir, 0o750)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("create output directory: %w", err)
	}

	receiverJSONPath, receiverCSVPath, err := writeReceiverTable(baseDir, outputs)
	if err != nil {
		return ExportOutputs{}, err
	}

	metaPath, dataPath, err := writeIndicatorRaster(baseDir, standardID, outputs, grid)
	if err != nil {
		return ExportOutputs{}, err
	}

	return ExportOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		RasterMetaPath:   metaPath,
		RasterDataPath:   dataPath,
	}, nil
}

func writeReceiverTable(baseDir string, outputs []ReceiverOutput) (string, string, error) {
	table := results.ReceiverTable{
		IndicatorOrder: IndicatorOrder(),
		Unit:           "dB",
		Records:        make([]results.ReceiverRecord, 0, len(outputs)),
	}

	for _, output := range outputs {
		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      output.Receiver.ID,
			X:       output.Receiver.Point.X,
			Y:       output.Receiver.Point.Y,
			HeightM: output.Receiver.HeightM,
			Values:  output.Indicators.Values(),
		})
	}

	receiverJSONPath := filepath.Join(baseDir, "receivers.json")
	receiverCSVPath := filepath.Join(baseDir, "receivers.csv")

	err := results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return "", "", fmt.Errorf("save receiver table json %s: %w", receiverJSONPath, err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return "", "", fmt.Errorf("save receiver table csv %s: %w", receiverCSVPath, err)
	}

	return receiverJSONPath, receiverCSVPath, nil
}

// writeIndicatorRaster writes the Lden/Lnight grid. Receiver order is the grid
// order — row-major from the origin — which is what lets the index arithmetic
// below stand in for coordinates.
func writeIndicatorRaster(baseDir string, standardID string, outputs []ReceiverOutput, grid results.GridLayout) (string, string, error) {
	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     grid.Width,
		Height:    grid.Height,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{IndicatorLden, IndicatorLnight},
		CRS:       grid.CRS,
		Geo:       grid.Geo,
	})
	if err != nil {
		return "", "", fmt.Errorf("create raster: %w", err)
	}

	for index, output := range outputs {
		x := index % grid.Width

		y := index / grid.Width

		err := raster.Set(x, y, 0, output.Indicators.Lden)
		if err != nil {
			return "", "", fmt.Errorf("set raster band %s: %w", IndicatorLden, err)
		}

		err = raster.Set(x, y, 1, output.Indicators.Lnight)
		if err != nil {
			return "", "", fmt.Errorf("set raster band %s: %w", IndicatorLnight, err)
		}
	}

	persistence, err := results.SaveRaster(filepath.Join(baseDir, standardID), raster)
	if err != nil {
		return "", "", fmt.Errorf("save raster %s: %w", standardID, err)
	}

	return persistence.MetadataPath, persistence.DataPath, nil
}
