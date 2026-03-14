package road

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

// ExportResultBundle exports LrDay/LrNight receiver table and raster outputs.
func ExportResultBundle(baseDir string, outputs []ReceiverOutput, gridWidth int, gridHeight int) (ExportOutputs, error) {
	if baseDir == "" {
		return ExportOutputs{}, errors.New("base dir is required")
	}

	if len(outputs) == 0 {
		return ExportOutputs{}, errors.New("at least one receiver output is required")
	}

	if gridWidth <= 0 || gridHeight <= 0 {
		return ExportOutputs{}, errors.New("grid dimensions must be > 0")
	}

	if gridWidth*gridHeight != len(outputs) {
		return ExportOutputs{}, fmt.Errorf("grid dimensions (%dx%d) do not match receiver output count (%d)", gridWidth, gridHeight, len(outputs))
	}

	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("create output directory: %w", err)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{IndicatorLrDay, IndicatorLrNight},
		Unit:           "dB",
		Records:        make([]results.ReceiverRecord, 0, len(outputs)),
	}
	for _, output := range outputs {
		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      output.Receiver.ID,
			X:       output.Receiver.Point.X,
			Y:       output.Receiver.Point.Y,
			HeightM: output.Receiver.HeightM,
			Values: map[string]float64{
				IndicatorLrDay:   output.Indicators.LrDay,
				IndicatorLrNight: output.Indicators.LrNight,
			},
		})
	}

	receiverJSONPath := filepath.Join(baseDir, "receivers.json")
	receiverCSVPath := filepath.Join(baseDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return ExportOutputs{}, err
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return ExportOutputs{}, err
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{IndicatorLrDay, IndicatorLrNight},
	})
	if err != nil {
		return ExportOutputs{}, err
	}

	for index, output := range outputs {
		x := index % gridWidth
		y := index / gridWidth

		err := raster.Set(x, y, 0, output.Indicators.LrDay)
		if err != nil {
			return ExportOutputs{}, err
		}

		err = raster.Set(x, y, 1, output.Indicators.LrNight)
		if err != nil {
			return ExportOutputs{}, err
		}
	}

	persistence, err := results.SaveRaster(filepath.Join(baseDir, "rls19-road"), raster)
	if err != nil {
		return ExportOutputs{}, err
	}

	return ExportOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		RasterMetaPath:   persistence.MetadataPath,
		RasterDataPath:   persistence.DataPath,
	}, nil
}
