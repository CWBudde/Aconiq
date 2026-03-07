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
	// StandardID identifies the RLS-19 road module in the standards registry.
	StandardID = "rls19-road"

	IndicatorLrDay   = "LrDay"
	IndicatorLrNight = "LrNight"
)

// VehicleGroup identifies one of the four RLS-19 vehicle categories.
type VehicleGroup int

const (
	Pkw  VehicleGroup = iota // passenger cars
	Lkw1                     // light trucks (<=3.5 t)
	Lkw2                     // heavy trucks (>3.5 t) + buses
	Krad                     // motorcycles
)

var vehicleGroupNames = [...]string{"Pkw", "Lkw1", "Lkw2", "Krad"}

func (vg VehicleGroup) String() string {
	if int(vg) < len(vehicleGroupNames) {
		return vehicleGroupNames[vg]
	}

	return fmt.Sprintf("VehicleGroup(%d)", int(vg))
}

// AllVehicleGroups returns all four RLS-19 vehicle groups in canonical order.
func AllVehicleGroups() [4]VehicleGroup { return [4]VehicleGroup{Pkw, Lkw1, Lkw2, Krad} }

// JunctionType classifies the nearest relevant junction.
type JunctionType int

const (
	JunctionNone       JunctionType = iota // no relevant junction
	JunctionSignalized                     // lichtsignalgeregelt
	JunctionRoundabout                     // Kreisverkehr
	JunctionOther                          // sonstige plangleiche Knotenpunkte
)

var junctionTypeNames = map[string]JunctionType{
	"none":       JunctionNone,
	"signalized": JunctionSignalized,
	"roundabout": JunctionRoundabout,
	"other":      JunctionOther,
}

func ParseJunctionType(s string) (JunctionType, error) {
	jt, ok := junctionTypeNames[strings.ToLower(strings.TrimSpace(s))]
	if !ok {
		return JunctionNone, fmt.Errorf("unknown junction type %q", s)
	}

	return jt, nil
}

func (jt JunctionType) String() string {
	for name, v := range junctionTypeNames {
		if v == jt {
			return name
		}
	}

	return "none"
}

// SurfaceType is an RLS-19 road surface (Straßendeckschicht) identifier.
// The correction value depends on vehicle group and speed range, looked up
// from the DStrO table in tables.go.
type SurfaceType string

const (
	// SurfaceNotSpecified indicates no surface type was specified.
	SurfaceNotSpecified     SurfaceType = ""
	SurfaceSMA              SurfaceType = "SMA"         // Splittmastixasphalt
	SurfaceAB               SurfaceType = "AB"          // Asphaltbeton
	SurfaceOPA              SurfaceType = "OPA"         // Offenporiger Asphalt
	SurfacePaving           SurfaceType = "Pflaster"    // Pflaster
	SurfaceConcrete         SurfaceType = "Beton"       // Waschbeton/Beton
	SurfaceLOA              SurfaceType = "LOA"         // Lärmoptimierter Asphalt
	SurfaceDSHV             SurfaceType = "DSH-V"       // Dünne Schicht im Heißeinbau auf Versiegelung
	SurfaceGussasphalt      SurfaceType = "Gussasphalt" // Gussasphalt
	SurfaceUnpavedOrDamaged SurfaceType = "beschaedigt" // unbefestigt/stark beschädigt
)

var allowedSurfaceTypes = map[SurfaceType]struct{}{
	SurfaceSMA:              {},
	SurfaceAB:               {},
	SurfaceOPA:              {},
	SurfacePaving:           {},
	SurfaceConcrete:         {},
	SurfaceLOA:              {},
	SurfaceDSHV:             {},
	SurfaceGussasphalt:      {},
	SurfaceUnpavedOrDamaged: {},
}

func isAllowedSurface(st SurfaceType) bool {
	_, ok := allowedSurfaceTypes[st]
	return ok
}

// TrafficInput holds hourly traffic counts per vehicle group for one time period.
type TrafficInput struct {
	PkwPerHour  float64 `json:"pkw_per_hour"`
	Lkw1PerHour float64 `json:"lkw1_per_hour"`
	Lkw2PerHour float64 `json:"lkw2_per_hour"`
	KradPerHour float64 `json:"krad_per_hour"`
}

// CountForGroup returns the hourly count for a specific vehicle group.
func (t TrafficInput) CountForGroup(vg VehicleGroup) float64 {
	switch vg {
	case Pkw:
		return t.PkwPerHour
	case Lkw1:
		return t.Lkw1PerHour
	case Lkw2:
		return t.Lkw2PerHour
	case Krad:
		return t.KradPerHour
	default:
		return 0
	}
}

// TotalPerHour returns the total hourly vehicle count across all groups.
func (t TrafficInput) TotalPerHour() float64 {
	return t.PkwPerHour + t.Lkw1PerHour + t.Lkw2PerHour + t.KradPerHour
}

func validateTrafficInput(sourceID, period string, t TrafficInput) error {
	check := func(name string, v float64) error {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("road source %q traffic_%s %s must be finite and >= 0", sourceID, period, name)
		}

		return nil
	}
	err := check("pkw_per_hour", t.PkwPerHour)
	if err != nil {
		return err
	}

	err = check("lkw1_per_hour", t.Lkw1PerHour)
	if err != nil {
		return err
	}

	err = check("lkw2_per_hour", t.Lkw2PerHour)
	if err != nil {
		return err
	}

	err = check("krad_per_hour", t.KradPerHour)
	if err != nil {
		return err
	}

	return nil
}

// SpeedInput holds the permitted speed per vehicle group in km/h.
type SpeedInput struct {
	PkwKPH  float64 `json:"pkw_kph"`
	Lkw1KPH float64 `json:"lkw1_kph"`
	Lkw2KPH float64 `json:"lkw2_kph"`
	KradKPH float64 `json:"krad_kph"`
}

// SpeedForGroup returns the speed for a specific vehicle group.
func (s SpeedInput) SpeedForGroup(vg VehicleGroup) float64 {
	switch vg {
	case Pkw:
		return s.PkwKPH
	case Lkw1:
		return s.Lkw1KPH
	case Lkw2:
		return s.Lkw2KPH
	case Krad:
		return s.KradKPH
	default:
		return 0
	}
}

func validateSpeedInput(sourceID string, s SpeedInput) error {
	check := func(name string, v float64) error {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return fmt.Errorf("road source %q speed %s must be finite and > 0", sourceID, name)
		}

		return nil
	}
	err := check("pkw_kph", s.PkwKPH)
	if err != nil {
		return err
	}

	err = check("lkw1_kph", s.Lkw1KPH)
	if err != nil {
		return err
	}

	err = check("lkw2_kph", s.Lkw2KPH)
	if err != nil {
		return err
	}

	err = check("krad_kph", s.KradKPH)
	if err != nil {
		return err
	}

	return nil
}

// RoadSource describes one RLS-19 road source segment (one direction/lane group).
type RoadSource struct {
	ID string `json:"id"`

	// Geometry: source line for this direction/lane group.
	Centerline []geo.Point2D `json:"centerline"`

	// Road attributes.
	SurfaceType       SurfaceType  `json:"surface_type"`
	Speeds            SpeedInput   `json:"speeds"`
	GradientPercent   float64      `json:"gradient_percent,omitempty"`
	JunctionType      JunctionType `json:"junction_type,omitempty"`
	JunctionDistanceM float64      `json:"junction_distance_m,omitempty"`

	// Multiple-reflection surcharge input (E5): building heights and
	// street canyon width for the Mehrfachreflexionszuschlag.
	// Zero values mean no surcharge.
	ReflectionSurchargeDB float64 `json:"reflection_surcharge_db,omitempty"`

	// Traffic per time period (maßgebende stündliche Verkehrsstärke).
	TrafficDay   TrafficInput `json:"traffic_day"`
	TrafficNight TrafficInput `json:"traffic_night"`
}

// Validate validates a road source.
func (s RoadSource) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("road source id is required")
	}

	if len(s.Centerline) < 2 {
		return fmt.Errorf("road source %q centerline must contain at least 2 points", s.ID)
	}

	for i, pt := range s.Centerline {
		if !pt.IsFinite() {
			return fmt.Errorf("road source %q centerline point[%d] is not finite", s.ID, i)
		}
	}

	if s.SurfaceType != SurfaceNotSpecified && !isAllowedSurface(s.SurfaceType) {
		return fmt.Errorf("road source %q has unsupported surface_type %q", s.ID, s.SurfaceType)
	}

	err := validateSpeedInput(s.ID, s.Speeds)
	if err != nil {
		return err
	}

	if !isFinite(s.GradientPercent) {
		return fmt.Errorf("road source %q gradient_percent must be finite", s.ID)
	}

	if !isFinite(s.JunctionDistanceM) || s.JunctionDistanceM < 0 {
		return fmt.Errorf("road source %q junction_distance_m must be finite and >= 0", s.ID)
	}

	if !isFinite(s.ReflectionSurchargeDB) || s.ReflectionSurchargeDB < 0 {
		return fmt.Errorf("road source %q reflection_surcharge_db must be finite and >= 0", s.ID)
	}

	err = validateTrafficInput(s.ID, "day", s.TrafficDay)
	if err != nil {
		return err
	}

	err = validateTrafficInput(s.ID, "night", s.TrafficNight)
	if err != nil {
		return err
	}

	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// Descriptor returns the standards-framework descriptor for RLS-19 road.
//
// Legal note: This module implements the calculation method described in
// RLS-19 (Richtlinien fuer den Laermschutz an Strassen, Ausgabe 2019).
// Normative coefficients are stored as data tables (see tables.go) and
// are structured so they can be replaced by an external data pack.
// No restricted normative text is embedded verbatim.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001
	minGradient := -12.0
	maxGradient := 12.0

	return framework.StandardDescriptor{
		ID:             StandardID,
		Description:    "RLS-19 road noise (16. BImSchV planning track): Lr day/night with TEST-20 emission/propagation chain.",
		DefaultVersion: "2019",
		Versions: []framework.Version{
			{
				Name:           "2019",
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
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height above ground in meters"},
								{
									Name:         "surface_type",
									Kind:         framework.ParameterKindString,
									DefaultValue: string(SurfaceSMA),
									Enum: []string{
										string(SurfaceSMA), string(SurfaceAB), string(SurfaceOPA),
										string(SurfacePaving), string(SurfaceConcrete), string(SurfaceLOA),
										string(SurfaceDSHV), string(SurfaceGussasphalt), string(SurfaceUnpavedOrDamaged),
									},
									Description: "Default surface type (DStrO) for road sources",
								},
								{Name: "speed_pkw_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default Pkw speed"},
								{Name: "speed_lkw1_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default Lkw1 speed"},
								{Name: "speed_lkw2_kph", Kind: framework.ParameterKindFloat, DefaultValue: "80", Min: &minPositive, Description: "Default Lkw2 speed"},
								{Name: "speed_krad_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default Krad speed"},
								{Name: "gradient_percent", Kind: framework.ParameterKindFloat, DefaultValue: "0", Min: &minGradient, Max: &maxGradient, Description: "Default road gradient"},
								{Name: "traffic_day_pkw", Kind: framework.ParameterKindFloat, DefaultValue: "900", Min: &minZero, Description: "Day Pkw per hour"},
								{Name: "traffic_day_lkw1", Kind: framework.ParameterKindFloat, DefaultValue: "40", Min: &minZero, Description: "Day Lkw1 per hour"},
								{Name: "traffic_day_lkw2", Kind: framework.ParameterKindFloat, DefaultValue: "60", Min: &minZero, Description: "Day Lkw2 per hour"},
								{Name: "traffic_day_krad", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minZero, Description: "Day Krad per hour"},
								{Name: "traffic_night_pkw", Kind: framework.ParameterKindFloat, DefaultValue: "200", Min: &minZero, Description: "Night Pkw per hour"},
								{Name: "traffic_night_lkw1", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minZero, Description: "Night Lkw1 per hour"},
								{Name: "traffic_night_lkw2", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minZero, Description: "Night Lkw2 per hour"},
								{Name: "traffic_night_krad", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Night Krad per hour"},
								{Name: "segment_length_m", Kind: framework.ParameterKindFloat, DefaultValue: "1", Min: &minPositive, Description: "Sub-segment length for Teilstueckverfahren"},
								{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum propagation distance"},
							},
						},
					},
				},
			},
		},
	}
}
