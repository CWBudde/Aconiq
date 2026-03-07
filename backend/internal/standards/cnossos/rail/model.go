package rail

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the CNOSSOS-EU rail module entry in the standards registry.
	StandardID = "cnossos-rail"

	IndicatorLday     = "Lday"
	IndicatorLevening = "Levening"
	IndicatorLnight   = "Lnight"
	IndicatorLden     = "Lden"
)

const (
	TractionElectric = "electric"
	TractionDiesel   = "diesel"
	TractionMixed    = "mixed"
)

const (
	RoughnessSmooth   = "smooth"
	RoughnessStandard = "standard"
	RoughnessRough    = "rough"
)

var allowedTractionTypes = map[string]struct{}{
	TractionElectric: {},
	TractionDiesel:   {},
	TractionMixed:    {},
}

var allowedRoughnessClasses = map[string]struct{}{
	RoughnessSmooth:   {},
	RoughnessStandard: {},
	RoughnessRough:    {},
}

// TrafficPeriod stores train count information for one period.
type TrafficPeriod struct {
	TrainsPerHour float64 `json:"trains_per_hour"`
}

// RailSource describes one railway source segment.
type RailSource struct {
	ID                   string        `json:"id"`
	TrackCenterline      []geo.Point2D `json:"track_centerline"`
	TractionType         string        `json:"traction_type"`
	TrackRoughnessClass  string        `json:"track_roughness_class"`
	AverageTrainSpeedKPH float64       `json:"average_train_speed_kph"`
	BrakingShare         float64       `json:"braking_share"`
	CurveRadiusM         float64       `json:"curve_radius_m,omitempty"`
	OnBridge             bool          `json:"on_bridge,omitempty"`
	TrafficDay           TrafficPeriod `json:"traffic_day"`
	TrafficEvening       TrafficPeriod `json:"traffic_evening"`
	TrafficNight         TrafficPeriod `json:"traffic_night"`
}

// Validate validates one rail source payload.
func (s RailSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("rail source id is required")
	}

	if len(s.TrackCenterline) < 2 {
		return fmt.Errorf("rail source %q track_centerline must contain at least 2 points", s.ID)
	}

	for i, point := range s.TrackCenterline {
		if !point.IsFinite() {
			return fmt.Errorf("rail source %q track_centerline point[%d] is not finite", s.ID, i)
		}
	}

	if _, ok := allowedTractionTypes[strings.TrimSpace(s.TractionType)]; !ok {
		return fmt.Errorf("rail source %q has unsupported traction_type %q", s.ID, s.TractionType)
	}

	if _, ok := allowedRoughnessClasses[strings.TrimSpace(s.TrackRoughnessClass)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_roughness_class %q", s.ID, s.TrackRoughnessClass)
	}

	if math.IsNaN(s.AverageTrainSpeedKPH) || math.IsInf(s.AverageTrainSpeedKPH, 0) || s.AverageTrainSpeedKPH <= 0 {
		return fmt.Errorf("rail source %q average_train_speed_kph must be finite and > 0", s.ID)
	}

	if math.IsNaN(s.BrakingShare) || math.IsInf(s.BrakingShare, 0) || s.BrakingShare < 0 || s.BrakingShare > 1 {
		return fmt.Errorf("rail source %q braking_share must be within [0,1]", s.ID)
	}

	if math.IsNaN(s.CurveRadiusM) || math.IsInf(s.CurveRadiusM, 0) {
		return fmt.Errorf("rail source %q curve_radius_m must be finite", s.ID)
	}

	err := validateTrafficPeriod(s.ID, "day", s.TrafficDay)
	if err != nil {
		return err
	}

	err = validateTrafficPeriod(s.ID, "evening", s.TrafficEvening)
	if err != nil {
		return err
	}

	err = validateTrafficPeriod(s.ID, "night", s.TrafficNight)
	if err != nil {
		return err
	}

	return nil
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if math.IsNaN(traffic.TrainsPerHour) || math.IsInf(traffic.TrainsPerHour, 0) || traffic.TrainsPerHour < 0 {
		return fmt.Errorf("rail source %q traffic_%s trains_per_hour must be finite and >= 0", sourceID, period)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for CNOSSOS rail.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	maxOne := 1.0

	return framework.StandardDescriptor{
		ID:             StandardID,
		Description:    "CNOSSOS-EU rail preview module with typed source schema and deterministic indicators.",
		DefaultVersion: "2020-preview",
		Versions: []framework.Version{
			{
				Name:           "2020-preview",
				DefaultProfile: "default",
				Profiles: []framework.Profile{
					{
						Name:                 "default",
						SupportedSourceTypes: []string{"line"},
						SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{
									Name:         "rail_traction_type",
									Kind:         framework.ParameterKindString,
									DefaultValue: TractionElectric,
									Enum:         []string{TractionElectric, TractionDiesel, TractionMixed},
									Description:  "Default traction type for imported rail line sources",
								},
								{
									Name:         "rail_track_roughness_class",
									Kind:         framework.ParameterKindString,
									DefaultValue: RoughnessStandard,
									Enum:         []string{RoughnessSmooth, RoughnessStandard, RoughnessRough},
									Description:  "Default track roughness class for imported rail line sources",
								},
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
							},
						},
					},
				},
			},
		},
	}
}
