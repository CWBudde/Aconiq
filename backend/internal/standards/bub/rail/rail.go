package rail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	StandardID = "bub-rail"

	IndicatorLday     = cnossosrail.IndicatorLday
	IndicatorLevening = cnossosrail.IndicatorLevening
	IndicatorLnight   = cnossosrail.IndicatorLnight
	IndicatorLden     = cnossosrail.IndicatorLden

	TractionElectric = cnossosrail.TractionElectric
	TractionDiesel   = cnossosrail.TractionDiesel
	TractionMixed    = cnossosrail.TractionMixed

	RoughnessSmooth   = cnossosrail.RoughnessSmooth
	RoughnessStandard = cnossosrail.RoughnessStandard
	RoughnessRough    = cnossosrail.RoughnessRough

	TrackTypeBallasted = cnossosrail.TrackTypeBallasted
	TrackTypeSlab      = cnossosrail.TrackTypeSlab
)

type (
	TrafficPeriod      = cnossosrail.TrafficPeriod
	RailSource         = cnossosrail.RailSource
	PeriodLevels       = cnossosrail.PeriodLevels
	ReceiverIndicators = cnossosrail.ReceiverIndicators
	ReceiverOutput     = cnossosrail.ReceiverOutput
	PropagationConfig  = cnossosrail.PropagationConfig
)

type ExportOutputs struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
}

func DefaultPropagationConfig() PropagationConfig {
	return cnossosrail.DefaultPropagationConfig()
}

func ComputeEmission(source RailSource) (PeriodLevels, error) {
	emission, err := cnossosrail.ComputeEmission(source)
	if err != nil {
		return PeriodLevels{}, err
	}

	return PeriodLevels(emission), nil
}

func ComputeLden(levels PeriodLevels) float64 {
	return cnossosrail.ComputeLden(levels)
}

func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig) (PeriodLevels, error) {
	return cnossosrail.ComputeReceiverPeriodLevels(receiver, sources, cfg)
}

func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return cnossosrail.ComputeReceiverOutputs(receivers, sources, cfg)
}

func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	maxOne := 1.0

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "BUB rail mapping baseline with typed rail line sources and deterministic indicators.",
		DefaultVersion: "2021-preview",
		Versions: []framework.Version{{
			Name:           "2021-preview",
			DefaultProfile: "strategic-mapping",
			Profiles: []framework.Profile{{
				Name:                 "strategic-mapping",
				SupportedSourceTypes: []string{"line"},
				SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
				ParameterSchema: framework.ParameterSchema{Parameters: []framework.ParameterDefinition{
					{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "15", Min: &minPositive, Description: "Receiver grid spacing in meters"},
					{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Padding around source extent in meters"},
					{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
					{Name: "rail_traction_type", Kind: framework.ParameterKindString, DefaultValue: TractionElectric, Enum: []string{TractionElectric, TractionDiesel, TractionMixed}, Description: "Default traction type for imported rail sources"},
					{Name: "rail_track_roughness_class", Kind: framework.ParameterKindString, DefaultValue: RoughnessStandard, Enum: []string{RoughnessSmooth, RoughnessStandard, RoughnessRough}, Description: "Default roughness class for imported rail sources"},
					{Name: "rail_average_train_speed_kph", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minPositive, Description: "Default train speed for imported rail sources"},
					{Name: "rail_braking_share", Kind: framework.ParameterKindFloat, DefaultValue: "0.1", Min: &minZero, Max: &maxOne, Description: "Default braking share for imported rail sources"},
					{Name: "rail_curve_radius_m", Kind: framework.ParameterKindFloat, DefaultValue: "500", Min: &minZero, Description: "Default curve radius for imported rail sources"},
					{Name: "rail_on_bridge", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Default bridge flag for imported rail sources"},
					{Name: "traffic_day_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "12", Min: &minZero, Description: "Day trains per hour"},
					{Name: "traffic_evening_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "6", Min: &minZero, Description: "Evening trains per hour"},
					{Name: "traffic_night_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Night trains per hour"},
					{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
					{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.2", Min: &minZero, Description: "Ground attenuation term"},
					{Name: "bridge_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Bridge correction term"},
					{Name: "curve_squeal_db", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Curve squeal correction term"},
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

	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
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
