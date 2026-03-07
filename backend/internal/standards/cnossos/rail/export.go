package rail

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

// ExportResultBundle exports Lden/Lnight receiver table and raster outputs.
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

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return ExportOutputs{}, fmt.Errorf("create output directory: %w", err)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{IndicatorLden, IndicatorLnight, IndicatorLday, IndicatorLevening},
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
				IndicatorLden:     output.Indicators.Lden,
				IndicatorLnight:   output.Indicators.Lnight,
				IndicatorLday:     output.Indicators.Lday,
				IndicatorLevening: output.Indicators.Levening,
			},
		})
	}

	receiverJSONPath := filepath.Join(baseDir, "receivers.json")
	receiverCSVPath := filepath.Join(baseDir, "receivers.csv")

	if err := results.SaveReceiverTableJSON(receiverJSONPath, table); err != nil {
		return ExportOutputs{}, err
	}

	if err := results.SaveReceiverTableCSV(receiverCSVPath, table); err != nil {
		return ExportOutputs{}, err
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{IndicatorLden, IndicatorLnight},
	})
	if err != nil {
		return ExportOutputs{}, err
	}

	for index, output := range outputs {
		x := index % gridWidth

		y := index / gridWidth

		err := raster.Set(x, y, 0, output.Indicators.Lden)
		if err != nil {
			return ExportOutputs{}, err
		}

		err = raster.Set(x, y, 1, output.Indicators.Lnight)
		if err != nil {
			return ExportOutputs{}, err
		}
	}

	persistence, err := results.SaveRaster(filepath.Join(baseDir, "cnossos-rail"), raster)
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
