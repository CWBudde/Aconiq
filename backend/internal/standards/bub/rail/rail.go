package rail

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
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

// ExportOutputs describes written files for receiver table and raster output.
type ExportOutputs = acoustics.ExportOutputs

// ExportResultBundle exports Lden/Lnight receiver table and raster outputs.
// The layout is the shared END one; only the raster is named after this
// standard, so two bundles from different modules stay comparable.
func ExportResultBundle(baseDir string, outputs []ReceiverOutput, gridWidth int, gridHeight int) (ExportOutputs, error) {
	bundle, err := acoustics.ExportENDBundle(baseDir, StandardID, outputs, gridWidth, gridHeight)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("export %s result bundle: %w", StandardID, err)
	}

	return bundle, nil
}

func DefaultPropagationConfig() PropagationConfig {
	return cnossosrail.DefaultPropagationConfig()
}

func ComputeEmission(source RailSource) (PeriodLevels, error) {
	emission, err := cnossosrail.ComputeEmission(source)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute emission: %w", err)
	}

	return PeriodLevels(emission), nil
}

func ComputeLden(levels PeriodLevels) float64 {
	return cnossosrail.ComputeLden(levels)
}

func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig) (PeriodLevels, error) {
	levels, err := cnossosrail.ComputeReceiverPeriodLevels(receiver, sources, cfg)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute receiver period levels: %w", err)
	}

	return levels, nil
}

func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := cnossosrail.ComputeReceiverOutputs(receivers, sources, cfg)
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}

func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	maxOne := 1.0

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "Scaffold rail mapping module, an alias over the cnossos-rail scaffold: typed rail line sources and deterministic indicators only, with no BUB or CNOSSOS-EU coefficients, spectra or octave bands; the intended target is the BUB mapping directive, which it does not implement.",
		EvidenceTier:   framework.EvidenceTierScaffold,
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
