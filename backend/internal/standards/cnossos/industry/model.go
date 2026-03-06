package industry

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the CNOSSOS-EU industry module entry in the standards registry.
	StandardID = "cnossos-industry"

	IndicatorLday     = "Lday"
	IndicatorLevening = "Levening"
	IndicatorLnight   = "Lnight"
	IndicatorLden     = "Lden"
)

const (
	SourceTypePoint = "point"
	SourceTypeArea  = "area"
)

const (
	CategoryProcess = "process"
	CategoryStack   = "stack"
	CategoryYard    = "yard"
)

const (
	EnclosureOpen     = "open"
	EnclosurePartial  = "partial"
	EnclosureEnclosed = "enclosed"
)

var allowedSourceCategories = map[string]struct{}{
	CategoryProcess: {},
	CategoryStack:   {},
	CategoryYard:    {},
}

var allowedEnclosureStates = map[string]struct{}{
	EnclosureOpen:     {},
	EnclosurePartial:  {},
	EnclosureEnclosed: {},
}

// OperationPeriod stores normalized source activity for one time period.
type OperationPeriod struct {
	OperatingFactor float64 `json:"operating_factor"`
}

// IndustrySource describes one industrial source.
type IndustrySource struct {
	ID                      string          `json:"id"`
	SourceType              string          `json:"source_type"`
	SourceCategory          string          `json:"source_category"`
	EnclosureState          string          `json:"enclosure_state"`
	Point                   geo.Point2D     `json:"point,omitempty"`
	AreaPolygon             [][]geo.Point2D `json:"area_polygon,omitempty"`
	SourceHeightM           float64         `json:"source_height_m"`
	SoundPowerLevelDB       float64         `json:"sound_power_level_db"`
	TonalityCorrectionDB    float64         `json:"tonality_correction_db,omitempty"`
	ImpulsivityCorrectionDB float64         `json:"impulsivity_correction_db,omitempty"`
	OperationDay            OperationPeriod `json:"operation_day"`
	OperationEvening        OperationPeriod `json:"operation_evening"`
	OperationNight          OperationPeriod `json:"operation_night"`
}

// Validate validates one industry source payload.
func (s IndustrySource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("industry source id is required")
	}

	switch strings.TrimSpace(s.SourceType) {
	case SourceTypePoint:
		if !s.Point.IsFinite() {
			return fmt.Errorf("industry source %q point is not finite", s.ID)
		}
	case SourceTypeArea:
		if len(s.AreaPolygon) == 0 {
			return fmt.Errorf("industry source %q area_polygon must contain at least one ring", s.ID)
		}

		for ringIndex, ring := range s.AreaPolygon {
			if len(ring) < 4 {
				return fmt.Errorf("industry source %q area_polygon ring[%d] must contain at least 4 points", s.ID, ringIndex)
			}

			for pointIndex, point := range ring {
				if !point.IsFinite() {
					return fmt.Errorf("industry source %q area_polygon ring[%d] point[%d] is not finite", s.ID, ringIndex, pointIndex)
				}
			}
		}
	default:
		return fmt.Errorf("industry source %q has unsupported source_type %q", s.ID, s.SourceType)
	}

	if _, ok := allowedSourceCategories[strings.TrimSpace(s.SourceCategory)]; !ok {
		return fmt.Errorf("industry source %q has unsupported source_category %q", s.ID, s.SourceCategory)
	}

	if _, ok := allowedEnclosureStates[strings.TrimSpace(s.EnclosureState)]; !ok {
		return fmt.Errorf("industry source %q has unsupported enclosure_state %q", s.ID, s.EnclosureState)
	}

	if math.IsNaN(s.SourceHeightM) || math.IsInf(s.SourceHeightM, 0) || s.SourceHeightM < 0 {
		return fmt.Errorf("industry source %q source_height_m must be finite and >= 0", s.ID)
	}

	if math.IsNaN(s.SoundPowerLevelDB) || math.IsInf(s.SoundPowerLevelDB, 0) {
		return fmt.Errorf("industry source %q sound_power_level_db must be finite", s.ID)
	}

	if math.IsNaN(s.TonalityCorrectionDB) || math.IsInf(s.TonalityCorrectionDB, 0) {
		return fmt.Errorf("industry source %q tonality_correction_db must be finite", s.ID)
	}

	if math.IsNaN(s.ImpulsivityCorrectionDB) || math.IsInf(s.ImpulsivityCorrectionDB, 0) {
		return fmt.Errorf("industry source %q impulsivity_correction_db must be finite", s.ID)
	}

	err := validateOperationPeriod(s.ID, "day", s.OperationDay)
	if err != nil {
		return err
	}

	err = validateOperationPeriod(s.ID, "evening", s.OperationEvening)
	if err != nil {
		return err
	}

	err = validateOperationPeriod(s.ID, "night", s.OperationNight)
	if err != nil {
		return err
	}

	return nil
}

func validateOperationPeriod(sourceID string, period string, operation OperationPeriod) error {
	if math.IsNaN(operation.OperatingFactor) || math.IsInf(operation.OperatingFactor, 0) || operation.OperatingFactor < 0 {
		return fmt.Errorf("industry source %q operation_%s operating_factor must be finite and >= 0", sourceID, period)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for CNOSSOS industry.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "CNOSSOS-EU industry preview module with typed point/area sources and deterministic indicators.",
		DefaultVersion: "2020-preview",
		Versions: []framework.Version{
			{
				Name:           "2020-preview",
				DefaultProfile: "default",
				Profiles: []framework.Profile{
					{
						Name:                 "default",
						SupportedSourceTypes: []string{SourceTypePoint, SourceTypeArea},
						SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "industry_source_category", Kind: framework.ParameterKindString, DefaultValue: CategoryProcess, Enum: []string{CategoryProcess, CategoryStack, CategoryYard}, Description: "Default source category for imported industry sources"},
								{Name: "industry_enclosure_state", Kind: framework.ParameterKindString, DefaultValue: EnclosureOpen, Enum: []string{EnclosureOpen, EnclosurePartial, EnclosureEnclosed}, Description: "Default enclosure state for imported industry sources"},
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
							},
						},
					},
				},
			},
		},
	}
}
