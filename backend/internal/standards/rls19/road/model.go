package road

import (
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the RLS-19 road module entry in the standards registry.
	StandardID = "rls19-road"

	IndicatorLrDay   = "LrDay"
	IndicatorLrNight = "LrNight"
)

const (
	SurfaceDenseAsphalt = "dense_asphalt"
	SurfaceOpenAsphalt  = "open_asphalt"
	SurfaceConcrete     = "concrete"
	SurfacePaving       = "paving"
)

var allowedSurfaceTypes = map[string]struct{}{
	SurfaceDenseAsphalt: {},
	SurfaceOpenAsphalt:  {},
	SurfaceConcrete:     {},
	SurfacePaving:       {},
}

// TrafficPeriod stores hourly flow split by light/heavy vehicle classes.
type TrafficPeriod struct {
	LightVehiclesPerHour float64 `json:"light_vehicles_per_hour"`
	HeavyVehiclesPerHour float64 `json:"heavy_vehicles_per_hour"`
}

// RoadSource describes one RLS-19 road source segment.
type RoadSource struct {
	ID                string        `json:"id"`
	Centerline        []geo.Point2D `json:"centerline"`
	SurfaceType       string        `json:"surface_type"`
	SpeedLightKPH     float64       `json:"speed_light_kph"`
	SpeedHeavyKPH     float64       `json:"speed_heavy_kph"`
	GradientPercent   float64       `json:"gradient_percent,omitempty"`
	JunctionDistanceM float64       `json:"junction_distance_m,omitempty"`
	TrafficDay        TrafficPeriod `json:"traffic_day"`
	TrafficNight      TrafficPeriod `json:"traffic_night"`
}

// Validate validates one road source schema payload.
func (s RoadSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("road source id is required")
	}
	if len(s.Centerline) < 2 {
		return fmt.Errorf("road source %q centerline must contain at least 2 points", s.ID)
	}
	for i, point := range s.Centerline {
		if !point.IsFinite() {
			return fmt.Errorf("road source %q centerline point[%d] is not finite", s.ID, i)
		}
	}
	if !isAllowedSurfaceType(s.SurfaceType) {
		return fmt.Errorf("road source %q has unsupported surface_type %q", s.ID, s.SurfaceType)
	}
	if math.IsNaN(s.SpeedLightKPH) || math.IsInf(s.SpeedLightKPH, 0) || s.SpeedLightKPH <= 0 {
		return fmt.Errorf("road source %q speed_light_kph must be finite and > 0", s.ID)
	}
	if math.IsNaN(s.SpeedHeavyKPH) || math.IsInf(s.SpeedHeavyKPH, 0) || s.SpeedHeavyKPH <= 0 {
		return fmt.Errorf("road source %q speed_heavy_kph must be finite and > 0", s.ID)
	}
	if math.IsNaN(s.GradientPercent) || math.IsInf(s.GradientPercent, 0) {
		return fmt.Errorf("road source %q gradient_percent must be finite", s.ID)
	}
	if math.IsNaN(s.JunctionDistanceM) || math.IsInf(s.JunctionDistanceM, 0) || s.JunctionDistanceM < 0 {
		return fmt.Errorf("road source %q junction_distance_m must be finite and >= 0", s.ID)
	}
	if err := validateTrafficPeriod(s.ID, "day", s.TrafficDay); err != nil {
		return err
	}
	if err := validateTrafficPeriod(s.ID, "night", s.TrafficNight); err != nil {
		return err
	}
	return nil
}

func validateTrafficPeriod(sourceID string, period string, traffic TrafficPeriod) error {
	if math.IsNaN(traffic.LightVehiclesPerHour) || math.IsInf(traffic.LightVehiclesPerHour, 0) || traffic.LightVehiclesPerHour < 0 {
		return fmt.Errorf("road source %q traffic_%s light_vehicles_per_hour must be finite and >= 0", sourceID, period)
	}
	if math.IsNaN(traffic.HeavyVehiclesPerHour) || math.IsInf(traffic.HeavyVehiclesPerHour, 0) || traffic.HeavyVehiclesPerHour < 0 {
		return fmt.Errorf("road source %q traffic_%s heavy_vehicles_per_hour must be finite and >= 0", sourceID, period)
	}
	return nil
}

func isAllowedSurfaceType(surfaceType string) bool {
	_, ok := allowedSurfaceTypes[strings.TrimSpace(surfaceType)]
	return ok
}

// Descriptor returns the standards-framework descriptor for RLS-19 road.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	minGradient := -12.0
	maxGradient := 12.0
	return framework.StandardDescriptor{
		ID:             StandardID,
		Description:    "RLS-19 road planning preview module with deterministic day/night indicators and acceptance hooks.",
		DefaultVersion: "2019-preview",
		Versions: []framework.Version{
			{
				Name:           "2019-preview",
				DefaultProfile: "default",
				Profiles: []framework.Profile{
					{
						Name:                 "default",
						SupportedSourceTypes: []string{"line"},
						SupportedIndicators:  []string{IndicatorLrDay, IndicatorLrNight},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{
									Name:         "road_surface_type",
									Kind:         framework.ParameterKindString,
									DefaultValue: SurfaceDenseAsphalt,
									Enum:         []string{SurfaceDenseAsphalt, SurfaceOpenAsphalt, SurfaceConcrete, SurfacePaving},
									Description:  "Default surface type for imported line sources",
								},
								{Name: "speed_light_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default light-vehicle speed for imported line sources"},
								{Name: "speed_heavy_kph", Kind: framework.ParameterKindFloat, DefaultValue: "80", Min: &minPositive, Description: "Default heavy-vehicle speed for imported line sources"},
								{Name: "gradient_percent", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minGradient, Max: &maxGradient, Description: "Default road gradient for imported line sources"},
								{Name: "junction_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "1000", Min: &minZero, Description: "Distance from source to nearest relevant junction"},
								{Name: "traffic_day_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "900", Min: &minZero, Description: "Day light vehicles per hour"},
								{Name: "traffic_day_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "90", Min: &minZero, Description: "Day heavy vehicles per hour"},
								{Name: "traffic_night_light_vph", Kind: framework.ParameterKindFloat, DefaultValue: "250", Min: &minZero, Description: "Night light vehicles per hour"},
								{Name: "traffic_night_heavy_vph", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Night heavy vehicles per hour"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.6", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "barrier_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Barrier attenuation term"},
								{Name: "reflection_gain_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Reflection gain term"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
							},
						},
					},
				},
			},
		},
	}
}

