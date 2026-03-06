package road

import (
	"fmt"
	"math"
	"strings"

	"github.com/soundplan/soundplan/backend/internal/geo"
	"github.com/soundplan/soundplan/backend/internal/standards/framework"
)

const (
	// StandardID identifies the CNOSSOS-EU road module entry in the standards registry.
	StandardID = "cnossos-road"

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

var allowedSurfaceTypes = map[string]struct{}{
	SurfaceDenseAsphalt:  {},
	SurfacePorousAsphalt: {},
	SurfaceConcrete:      {},
	SurfaceCobblestone:   {},
}

// TrafficPeriod stores hourly flow split by light/heavy classes.
type TrafficPeriod struct {
	LightVehiclesPerHour float64 `json:"light_vehicles_per_hour"`
	HeavyVehiclesPerHour float64 `json:"heavy_vehicles_per_hour"`
}

// RoadSource describes one road source segment.
type RoadSource struct {
	ID              string       `json:"id"`
	Centerline      []geo.Point2D `json:"centerline"`
	SurfaceType     string       `json:"surface_type"`
	SpeedKPH        float64      `json:"speed_kph"`
	GradientPercent float64      `json:"gradient_percent,omitempty"`
	TrafficDay      TrafficPeriod `json:"traffic_day"`
	TrafficEvening  TrafficPeriod `json:"traffic_evening"`
	TrafficNight    TrafficPeriod `json:"traffic_night"`
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
	if math.IsNaN(s.SpeedKPH) || math.IsInf(s.SpeedKPH, 0) || s.SpeedKPH <= 0 {
		return fmt.Errorf("road source %q speed_kph must be finite and > 0", s.ID)
	}
	if math.IsNaN(s.GradientPercent) || math.IsInf(s.GradientPercent, 0) {
		return fmt.Errorf("road source %q gradient_percent must be finite", s.ID)
	}
	if err := validateTrafficPeriod(s.ID, "day", s.TrafficDay); err != nil {
		return err
	}
	if err := validateTrafficPeriod(s.ID, "evening", s.TrafficEvening); err != nil {
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

// Descriptor returns the standards-framework descriptor for CNOSSOS road.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	minChunk := 1.0
	return framework.StandardDescriptor{
		ID:             StandardID,
		Description:    "CNOSSOS-EU road preview module with typed source schema and deterministic indicators.",
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
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "barrier_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minZero, Description: "Barrier attenuation term"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
								{Name: "chunk_size", Kind: framework.ParameterKindInt, DefaultValue: "128", Min: &minChunk, Description: "Engine receiver chunk size"},
								{Name: "workers", Kind: framework.ParameterKindInt, DefaultValue: "0", Min: &minZero, Description: "Engine worker count (0=auto)"},
								{Name: "disable_cache", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Disable chunk cache"},
							},
						},
					},
				},
			},
		},
	}
}
