package industry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	StandardID = "bub-industry"

	IndicatorLday     = cnossosindustry.IndicatorLday
	IndicatorLevening = cnossosindustry.IndicatorLevening
	IndicatorLnight   = cnossosindustry.IndicatorLnight
	IndicatorLden     = cnossosindustry.IndicatorLden

	SourceTypePoint = cnossosindustry.SourceTypePoint
	SourceTypeArea  = cnossosindustry.SourceTypeArea

	CategoryProcess = cnossosindustry.CategoryProcess
	CategoryStack   = cnossosindustry.CategoryStack
	CategoryYard    = cnossosindustry.CategoryYard

	EnclosureOpen     = cnossosindustry.EnclosureOpen
	EnclosurePartial  = cnossosindustry.EnclosurePartial
	EnclosureEnclosed = cnossosindustry.EnclosureEnclosed
)

type (
	OperationPeriod    = cnossosindustry.OperationPeriod
	IndustrySource     = cnossosindustry.IndustrySource
	PeriodLevels       = cnossosindustry.PeriodLevels
	ReceiverIndicators = cnossosindustry.ReceiverIndicators
	ReceiverOutput     = cnossosindustry.ReceiverOutput
	PropagationConfig  = cnossosindustry.PropagationConfig
)

type ExportOutputs struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
}

func DefaultPropagationConfig() PropagationConfig {
	return cnossosindustry.DefaultPropagationConfig()
}

func ComputeEmission(source IndustrySource) (PeriodLevels, error) {
	emission, err := cnossosindustry.ComputeEmission(source)
	if err != nil {
		return PeriodLevels{}, err
	}

	return PeriodLevels(emission), nil
}

func ComputeLden(levels PeriodLevels) float64 {
	return cnossosindustry.ComputeLden(levels)
}

func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) (PeriodLevels, error) {
	return cnossosindustry.ComputeReceiverPeriodLevels(receiver, sources, cfg)
}

func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return cnossosindustry.ComputeReceiverOutputs(receivers, sources, cfg)
}

func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "BUB industry mapping baseline with typed point and area sources and deterministic indicators.",
		DefaultVersion: "2021-preview",
		Versions: []framework.Version{{
			Name:           "2021-preview",
			DefaultProfile: "strategic-mapping",
			Profiles: []framework.Profile{{
				Name:                 "strategic-mapping",
				SupportedSourceTypes: []string{SourceTypePoint, SourceTypeArea},
				SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
				ParameterSchema: framework.ParameterSchema{Parameters: []framework.ParameterDefinition{
					{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "15", Min: &minPositive, Description: "Receiver grid spacing in meters"},
					{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Padding around source extent in meters"},
					{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
					{Name: "industry_sound_power_level_db", Kind: framework.ParameterKindFloat, DefaultValue: "96", Description: "Reference sound power level for imported industry sources"},
					{Name: "industry_source_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Default source height for imported industry sources"},
					{Name: "industry_tonality_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Tonality correction added to source emission"},
					{Name: "industry_impulsivity_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Impulsivity correction added to source emission"},
					{Name: "operation_day_factor", Kind: framework.ParameterKindFloat, DefaultValue: "1", Min: &minZero, Description: "Normalized daytime operating factor"},
					{Name: "operation_evening_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Normalized evening operating factor"},
					{Name: "operation_night_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.4", Min: &minZero, Description: "Normalized night operating factor"},
					{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
					{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.0", Min: &minZero, Description: "Ground attenuation term"},
					{Name: "screening_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Generic screening attenuation term"},
					{Name: "facade_reflection_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Generic facade reflection adjustment"},
					{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
				}},
			}},
		}},
	}
}

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

	persistence, err := results.SaveRaster(filepath.Join(baseDir, StandardID), raster)
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
