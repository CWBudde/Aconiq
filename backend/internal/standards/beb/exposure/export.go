package exposure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/report/results"
)

// ExportOutputs describes written files for BEB outputs.
type ExportOutputs struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
	SummaryPath      string
}

// ExportResultBundle exports building exposure tables plus a 1x1 totals raster and summary.
func ExportResultBundle(baseDir string, outputs []BuildingExposureOutput, summary Summary) (ExportOutputs, error) {
	if baseDir == "" {
		return ExportOutputs{}, errors.New("base dir is required")
	}

	if len(outputs) == 0 {
		return ExportOutputs{}, errors.New("at least one building exposure output is required")
	}

	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("create output directory: %w", err)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{
			IndicatorLden,
			IndicatorLnight,
			IndicatorEstimatedDwellings,
			IndicatorEstimatedPersons,
			IndicatorAffectedDwellingsLden,
			IndicatorAffectedPersonsLden,
			IndicatorAffectedDwellingsLnight,
			IndicatorAffectedPersonsLnight,
		},
		Unit:    "mixed",
		Records: make([]results.ReceiverRecord, 0, len(outputs)),
	}

	for _, output := range outputs {
		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      output.Building.ID,
			X:       output.RepresentativeReceiver.Point.X,
			Y:       output.RepresentativeReceiver.Point.Y,
			HeightM: output.Building.HeightM,
			Values: map[string]float64{
				IndicatorLden:                    output.Indicators.Lden,
				IndicatorLnight:                  output.Indicators.Lnight,
				IndicatorEstimatedDwellings:      output.Indicators.EstimatedDwellings,
				IndicatorEstimatedPersons:        output.Indicators.EstimatedPersons,
				IndicatorAffectedDwellingsLden:   output.Indicators.AffectedDwellingsLden,
				IndicatorAffectedPersonsLden:     output.Indicators.AffectedPersonsLden,
				IndicatorAffectedDwellingsLnight: output.Indicators.AffectedDwellingsLnight,
				IndicatorAffectedPersonsLnight:   output.Indicators.AffectedPersonsLnight,
			},
		})
	}

	receiverJSONPath := filepath.Join(baseDir, "buildings.json")
	receiverCSVPath := filepath.Join(baseDir, "buildings.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return ExportOutputs{}, err
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return ExportOutputs{}, err
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     1,
		Height:    1,
		Bands:     4,
		NoData:    -9999,
		Unit:      "count",
		BandNames: []string{IndicatorAffectedPersonsLden, IndicatorAffectedPersonsLnight, IndicatorAffectedDwellingsLden, IndicatorAffectedDwellingsLnight},
	})
	if err != nil {
		return ExportOutputs{}, err
	}

	err = raster.Set(0, 0, 0, summary.AffectedPersonsLden)
	if err != nil {
		return ExportOutputs{}, err
	}

	err = raster.Set(0, 0, 1, summary.AffectedPersonsLnight)
	if err != nil {
		return ExportOutputs{}, err
	}

	err = raster.Set(0, 0, 2, summary.AffectedDwellingsLden)
	if err != nil {
		return ExportOutputs{}, err
	}

	err = raster.Set(0, 0, 3, summary.AffectedDwellingsLnight)
	if err != nil {
		return ExportOutputs{}, err
	}

	persistence, err := results.SaveRaster(filepath.Join(baseDir, "beb-exposure"), raster)
	if err != nil {
		return ExportOutputs{}, err
	}

	summaryPath := filepath.Join(baseDir, "beb-summary.json")

	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return ExportOutputs{}, err
	}

	err = os.WriteFile(summaryPath, append(payload, '\n'), 0o600)
	if err != nil {
		return ExportOutputs{}, err
	}

	return ExportOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		RasterMetaPath:   persistence.MetadataPath,
		RasterDataPath:   persistence.DataPath,
		SummaryPath:      summaryPath,
	}, nil
}
