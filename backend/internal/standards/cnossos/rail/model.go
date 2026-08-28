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
	TrackTypeBallasted = "ballasted"
	TrackTypeSlab      = "slab"
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

var allowedTrackTypes = map[string]struct{}{
	TrackTypeBallasted: {},
	TrackTypeSlab:      {},
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
	TrackType            string        `json:"track_type"`
	TrackRoughnessClass  string        `json:"track_roughness_class"`
	AverageTrainSpeedKPH float64       `json:"average_train_speed_kph"`
	BrakingShare         float64       `json:"braking_share"`
	CurveRadiusM         float64       `json:"curve_radius_m,omitempty"`
	OnBridge             bool          `json:"on_bridge,omitempty"`
	TrafficDay           TrafficPeriod `json:"traffic_day"`
	TrafficEvening       TrafficPeriod `json:"traffic_evening"`
	TrafficNight         TrafficPeriod `json:"traffic_night"`
}

// finite reports whether v is neither NaN nor infinite.
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// finitePositive reports whether v is finite and strictly greater than zero.
func finitePositive(v float64) bool {
	return finite(v) && v > 0
}

// finiteNonNegative reports whether v is finite and greater than or equal to zero.
func finiteNonNegative(v float64) bool {
	return finite(v) && v >= 0
}

// Validate validates one rail source payload.
func (s RailSource) Validate() error {
	if err := s.validateGeometry(); err != nil {
		return err
	}

	if err := s.validateClassification(); err != nil {
		return err
	}

	if err := s.validateAcoustics(); err != nil {
		return err
	}

	return s.validateTraffic()
}

func (s RailSource) validateGeometry() error {
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

	return nil
}

func (s RailSource) validateClassification() error {
	if _, ok := allowedTractionTypes[strings.TrimSpace(s.TractionType)]; !ok {
		return fmt.Errorf("rail source %q has unsupported traction_type %q", s.ID, s.TractionType)
	}

	if _, ok := allowedTrackTypes[strings.TrimSpace(s.TrackType)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_type %q", s.ID, s.TrackType)
	}

	if _, ok := allowedRoughnessClasses[strings.TrimSpace(s.TrackRoughnessClass)]; !ok {
		return fmt.Errorf("rail source %q has unsupported track_roughness_class %q", s.ID, s.TrackRoughnessClass)
	}

	return nil
}

func (s RailSource) validateAcoustics() error {
	if !finitePositive(s.AverageTrainSpeedKPH) {
		return fmt.Errorf("rail source %q average_train_speed_kph must be finite and > 0", s.ID)
	}

	if !finite(s.BrakingShare) || s.BrakingShare < 0 || s.BrakingShare > 1 {
		return fmt.Errorf("rail source %q braking_share must be within [0,1]", s.ID)
	}

	if !finite(s.CurveRadiusM) {
		return fmt.Errorf("rail source %q curve_radius_m must be finite", s.ID)
	}

	return nil
}

func (s RailSource) validateTraffic() error {
	if err := validateTrafficPeriod(s.ID, "day", s.TrafficDay); err != nil {
		return err
	}

	if err := validateTrafficPeriod(s.ID, "evening", s.TrafficEvening); err != nil {
		return err
	}

	return validateTrafficPeriod(s.ID, "night", s.TrafficNight)
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if !finiteNonNegative(traffic.TrainsPerHour) {
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
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "Scaffold rail module: a typed source schema and deterministic indicators only, with no CNOSSOS-EU rolling or traction source terms, no roughness or contact-filter spectra and no octave bands; the intended target is Directive 2015/996 Annex II rail, which it does not implement.",
		EvidenceTier:   framework.EvidenceTierScaffold,
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
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around rail extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{
									Name:         "rail_traction_type",
									Kind:         framework.ParameterKindString,
									DefaultValue: TractionElectric,
									Enum:         []string{TractionElectric, TractionDiesel, TractionMixed},
									Description:  "Default traction type for imported rail line sources",
								},
								{
									Name:         "rail_track_type",
									Kind:         framework.ParameterKindString,
									DefaultValue: TrackTypeBallasted,
									Enum:         []string{TrackTypeBallasted, TrackTypeSlab},
									Description:  "Default track type for imported rail line sources",
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
