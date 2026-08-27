package road

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the BUB road mapping module entry in the standards registry.
	StandardID = "bub-road"

	IndicatorLday     = "Lday"
	IndicatorLevening = "Levening"
	IndicatorLnight   = "Lnight"
	IndicatorLden     = "Lden"
)

const (
	SurfaceDenseAsphalt  = "dense_asphalt"
	SurfacePorousAsphalt = "porous_asphalt"
	SurfaceConcrete      = "concrete"
	SurfaceCobblestone   = "cobblestone"
)

const (
	FunctionUrbanMain  = "urban_main"
	FunctionUrbanLocal = "urban_local"
	FunctionRuralMain  = "rural_main"
)

const (
	JunctionNone         = "none"
	JunctionTrafficLight = "traffic_light"
	JunctionRoundabout   = "roundabout"
)

var allowedSurfaceTypes = map[string]struct{}{
	SurfaceDenseAsphalt:  {},
	SurfacePorousAsphalt: {},
	SurfaceConcrete:      {},
	SurfaceCobblestone:   {},
}

var allowedFunctionClasses = map[string]struct{}{
	FunctionUrbanMain:  {},
	FunctionUrbanLocal: {},
	FunctionRuralMain:  {},
}

var allowedJunctionTypes = map[string]struct{}{
	JunctionNone:         {},
	JunctionTrafficLight: {},
	JunctionRoundabout:   {},
}

// TrafficPeriod stores hourly flow split by light/heavy classes.
type TrafficPeriod struct {
	LightVehiclesPerHour      float64 `json:"light_vehicles_per_hour"`
	MediumVehiclesPerHour     float64 `json:"medium_vehicles_per_hour,omitempty"`
	HeavyVehiclesPerHour      float64 `json:"heavy_vehicles_per_hour"`
	PoweredTwoWheelersPerHour float64 `json:"powered_two_wheelers_per_hour,omitempty"`
}

// RoadSource describes one BUB road mapping source segment.
type RoadSource struct {
	ID                string        `json:"id"`
	Centerline        []geo.Point2D `json:"centerline"`
	SurfaceType       string        `json:"surface_type"`
	RoadFunctionClass string        `json:"road_function_class"`
	SpeedKPH          float64       `json:"speed_kph"`
	GradientPercent   float64       `json:"gradient_percent,omitempty"`
	JunctionType      string        `json:"junction_type"`
	JunctionDistanceM float64       `json:"junction_distance_m,omitempty"`
	TemperatureC      float64       `json:"temperature_c,omitempty"`
	StuddedTyreShare  float64       `json:"studded_tyre_share,omitempty"`
	TrafficDay        TrafficPeriod `json:"traffic_day"`
	TrafficEvening    TrafficPeriod `json:"traffic_evening"`
	TrafficNight      TrafficPeriod `json:"traffic_night"`
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

// Validate validates one BUB road source schema payload.
func (s RoadSource) Validate() error {
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

func (s RoadSource) validateGeometry() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("road source id is required")
	}

	if len(s.Centerline) < 2 {
		return fmt.Errorf("road source %q centerline must contain at least 2 points", s.ID)
	}

	for i, point := range s.Centerline {
		if !point.IsFinite() {
			return fmt.Errorf("road source %q centerline point[%d] is not finite", s.ID, i)
		}
	}

	return nil
}

func (s RoadSource) validateClassification() error {
	if _, ok := allowedSurfaceTypes[strings.TrimSpace(s.SurfaceType)]; !ok {
		return fmt.Errorf("road source %q has unsupported surface_type %q", s.ID, s.SurfaceType)
	}

	if _, ok := allowedFunctionClasses[strings.TrimSpace(s.RoadFunctionClass)]; !ok {
		return fmt.Errorf("road source %q has unsupported road_function_class %q", s.ID, s.RoadFunctionClass)
	}

	return nil
}

func (s RoadSource) validateAcoustics() error {
	if !finitePositive(s.SpeedKPH) {
		return fmt.Errorf("road source %q speed_kph must be finite and > 0", s.ID)
	}

	if !finite(s.GradientPercent) {
		return fmt.Errorf("road source %q gradient_percent must be finite", s.ID)
	}

	if _, ok := allowedJunctionTypes[strings.TrimSpace(s.JunctionType)]; !ok {
		return fmt.Errorf("road source %q has unsupported junction_type %q", s.ID, s.JunctionType)
	}

	if !finiteNonNegative(s.JunctionDistanceM) {
		return fmt.Errorf("road source %q junction_distance_m must be finite and >= 0", s.ID)
	}

	if !finite(s.TemperatureC) {
		return fmt.Errorf("road source %q temperature_c must be finite", s.ID)
	}

	if !finite(s.StuddedTyreShare) || s.StuddedTyreShare < 0 || s.StuddedTyreShare > 1 {
		return fmt.Errorf("road source %q studded_tyre_share must be within [0,1]", s.ID)
	}

	return nil
}

func (s RoadSource) validateTraffic() error {
	if err := validateTrafficPeriod(s.ID, "day", s.TrafficDay); err != nil {
		return err
	}

	if err := validateTrafficPeriod(s.ID, "evening", s.TrafficEvening); err != nil {
		return err
	}

	return validateTrafficPeriod(s.ID, "night", s.TrafficNight)
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if !finiteNonNegative(traffic.LightVehiclesPerHour) {
		return fmt.Errorf("road source %q traffic_%s light_vehicles_per_hour must be finite and >= 0", sourceID, period)
	}

	if !finiteNonNegative(traffic.MediumVehiclesPerHour) {
		return fmt.Errorf("road source %q traffic_%s medium_vehicles_per_hour must be finite and >= 0", sourceID, period)
	}

	if !finiteNonNegative(traffic.HeavyVehiclesPerHour) {
		return fmt.Errorf("road source %q traffic_%s heavy_vehicles_per_hour must be finite and >= 0", sourceID, period)
	}

	if !finiteNonNegative(traffic.PoweredTwoWheelersPerHour) {
		return fmt.Errorf("road source %q traffic_%s powered_two_wheelers_per_hour must be finite and >= 0", sourceID, period)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for BUB road.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	minGradient := -20.0
	maxGradient := 20.0
	maxOne := 1.0

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "BUB road mapping baseline with typed source schema, mapping context metadata, and deterministic indicators.",
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
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "road_surface_type", Kind: framework.ParameterKindString, DefaultValue: SurfaceDenseAsphalt, Enum: []string{SurfaceDenseAsphalt, SurfacePorousAsphalt, SurfaceConcrete, SurfaceCobblestone}, Description: "Default surface type for imported road line sources"},
								{Name: "road_function_class", Kind: framework.ParameterKindString, DefaultValue: FunctionUrbanMain, Enum: []string{FunctionUrbanMain, FunctionUrbanLocal, FunctionRuralMain}, Description: "Default road function class for imported road line sources"},
								{Name: "road_speed_kph", Kind: framework.ParameterKindFloat, DefaultValue: "60", Min: &minPositive, Description: "Default speed for imported road line sources"},
								{Name: "road_gradient_percent", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minGradient, Max: &maxGradient, Description: "Default gradient for imported road line sources"},
								{Name: "road_junction_type", Kind: framework.ParameterKindString, DefaultValue: JunctionNone, Enum: []string{JunctionNone, JunctionTrafficLight, JunctionRoundabout}, Description: "Default junction context for imported road line sources"},
								{Name: "road_junction_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Distance to the nearest influencing junction"},
								{Name: "road_temperature_c", Kind: framework.ParameterKindFloat, DefaultValue: "15", Description: "Reference temperature used for BUB mapping adjustments"},
								{Name: "road_studded_tyre_share", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Max: &maxOne, Description: "Share of light vehicles using studded tyres"},
								{Name: "traffic_day_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "900", Min: &minZero, Description: "Day light vehicles per hour"},
								{Name: "traffic_day_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "120", Min: &minZero, Description: "Day medium vehicles per hour"},
								{Name: "traffic_day_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minZero, Description: "Day heavy vehicles per hour"},
								{Name: "traffic_evening_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "500", Min: &minZero, Description: "Evening light vehicles per hour"},
								{Name: "traffic_evening_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "60", Min: &minZero, Description: "Evening medium vehicles per hour"},
								{Name: "traffic_evening_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "45", Min: &minZero, Description: "Evening heavy vehicles per hour"},
								{Name: "traffic_night_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "250", Min: &minZero, Description: "Night light vehicles per hour"},
								{Name: "traffic_night_medium_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Night medium vehicles per hour"},
								{Name: "traffic_night_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Night heavy vehicles per hour"},
								{Name: "traffic_day_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Day powered two-wheelers per hour"},
								{Name: "traffic_evening_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "15", Min: &minZero, Description: "Evening powered two-wheelers per hour"},
								{Name: "traffic_night_ptw_vph", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Night powered two-wheelers per hour"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.2", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "urban_canyon_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Urban canyon mapping adjustment"},
								{Name: "intersection_density_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Intersection density mapping adjustment"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
							},
						},
					},
				},
			},
		},
	}
}
