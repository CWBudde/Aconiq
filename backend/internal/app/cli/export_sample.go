package cli

import (
	"os"
	"path/filepath"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/results"
)

// Indicator names used by the sample result bundle emitted with
// `aconiq export --with-sample`. They must match the indicator names the
// standards modules produce.
const (
	sampleIndicatorLden   = "Lden"
	sampleIndicatorLnight = "Lnight"
)

func emitSampleResultBundle(bundleDir string) ([]string, error) {
	resultsDir := filepath.Join(bundleDir, "sample-results")

	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "create sample results directory", err)
	}

	rasterBands := []string{sampleIndicatorLden}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     32,
		Height:    24,
		Bands:     1,
		NoData:    -9999,
		Units:     results.UniformUnits(rasterBands, results.UnitDecibel),
		BandNames: rasterBands,
	})
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "build sample raster", err)
	}

	for y := range raster.Metadata().Height {
		for x := range raster.Metadata().Width {
			value := 45.0 + float64(x)/4.0 + float64(y)/5.0

			err := raster.Set(x, y, 0, value)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "set sample raster cell", err)
			}
		}
	}

	rasterPaths, err := results.SaveRaster(filepath.Join(resultsDir, "lden-raster"), raster)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample raster", err)
	}

	sampleIndicators := []string{sampleIndicatorLden, sampleIndicatorLnight}
	table := results.ReceiverTable{
		IndicatorOrder: sampleIndicators,
		Units:          results.UniformUnits(sampleIndicators, results.UnitDecibel),
		Records: []results.ReceiverRecord{
			{ID: "rx-001", X: 100, Y: 200, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 56.3, sampleIndicatorLnight: 47.8}},
			{ID: "rx-002", X: 110, Y: 200, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 58.1, sampleIndicatorLnight: 49.2}},
			{ID: "rx-003", X: 120, Y: 205, HeightM: 4, Values: map[string]float64{sampleIndicatorLden: 55.4, sampleIndicatorLnight: 46.6}},
		},
	}

	jsonPath := filepath.Join(resultsDir, "receivers.json")
	csvPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(jsonPath, table)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver json", err)
	}

	err = results.SaveReceiverTableCSV(csvPath, table)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.emitSampleResultBundle", "save sample receiver csv", err)
	}

	return []string{
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.MetadataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(rasterPaths.DataPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(jsonPath))),
		filepath.ToSlash(filepath.Join("sample-results", filepath.Base(csvPath))),
	}, nil
}
