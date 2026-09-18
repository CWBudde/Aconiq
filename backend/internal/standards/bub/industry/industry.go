package industry

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
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

// ExportOutputs describes written files for receiver table and raster output.
type ExportOutputs = acoustics.ExportOutputs

// ExportResultBundle exports Lden/Lnight receiver table and raster outputs.
// The layout is the shared END one; only the raster is named after this
// standard, so two bundles from different modules stay comparable.
func ExportResultBundle(baseDir string, outputs []ReceiverOutput, grid results.GridLayout) (ExportOutputs, error) {
	bundle, err := acoustics.ExportENDBundle(baseDir, StandardID, outputs, grid)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("export %s result bundle: %w", StandardID, err)
	}

	return bundle, nil
}

func DefaultPropagationConfig() PropagationConfig {
	return cnossosindustry.DefaultPropagationConfig()
}

func ComputeEmission(source IndustrySource) (PeriodLevels, error) {
	emission, err := cnossosindustry.ComputeEmission(source)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute emission: %w", err)
	}

	return PeriodLevels(emission), nil
}

func ComputeLden(levels PeriodLevels) float64 {
	return cnossosindustry.ComputeLden(levels)
}

func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) (PeriodLevels, error) {
	levels, err := cnossosindustry.ComputeReceiverPeriodLevels(receiver, sources, cfg)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute receiver period levels: %w", err)
	}

	return levels, nil
}

func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := cnossosindustry.ComputeReceiverOutputs(receivers, sources, cfg)
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}

func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "Scaffold industry mapping module, an alias over the cnossos-industry scaffold: typed point and area sources with deterministic indicators only, with no BUB or CNOSSOS-EU coefficients and no octave bands; the intended target is the BUB mapping directive, which it does not implement.",
		EvidenceTier:   framework.EvidenceTierScaffold,
		DefaultVersion: "2021-preview",
		Versions: []framework.Version{{
			Name:           "2021-preview",
			DefaultProfile: "strategic-mapping",
			Profiles: []framework.Profile{{
				Name:                 "strategic-mapping",
				SupportedSourceTypes: []string{SourceTypePoint, SourceTypeArea},
				SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
				ParameterSchema: framework.ParameterSchema{Parameters: []framework.ParameterDefinition{
					{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "15", Min: &minPositive, Description: "Receiver grid spacing in meters"},
					{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "30", Min: &minZero, Description: "Padding around source extent in meters"},
					{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
					{Name: "industry_sound_power_level_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "96", Description: "Reference sound power level for imported industry sources"},
					{Name: "industry_source_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "5", Min: &minZero, Description: "Default source height for imported industry sources"},
					{Name: "industry_tonality_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Description: "Tonality correction added to source emission"},
					{Name: "industry_impulsivity_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Description: "Impulsivity correction added to source emission"},
					{Name: "operation_day_factor", Kind: framework.ParameterKindFloat, DefaultValue: "1", Min: &minZero, Description: "Normalized daytime operating factor"},
					{Name: "operation_evening_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Normalized evening operating factor"},
					{Name: "operation_night_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.4", Min: &minZero, Description: "Normalized night operating factor"},
					{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibelPerKilometer, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
					{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "1.0", Min: &minZero, Description: "Ground attenuation term"},
					{Name: "screening_attenuation_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Min: &minZero, Description: "Generic screening attenuation term"},
					{Name: "facade_reflection_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Min: &minZero, Description: "Generic facade reflection adjustment"},
					{Name: "min_distance_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
				}},
			}},
		}},
	}
}
