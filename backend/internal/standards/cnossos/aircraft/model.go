package aircraft

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// StandardID identifies the CNOSSOS-EU aircraft module entry in the standards registry.
	StandardID = "cnossos-aircraft"

	IndicatorLday     = "Lday"
	IndicatorLevening = "Levening"
	IndicatorLnight   = "Lnight"
	IndicatorLden     = "Lden"
)

const (
	SourceTypeLine = "line"
)

const (
	OperationDeparture = "departure"
	OperationArrival   = "arrival"
)

const (
	AircraftClassRegional = "regional"
	AircraftClassNarrow   = "narrow_body"
	AircraftClassWide     = "wide_body"
	AircraftClassCargo    = "cargo"
)

const (
	ProcedureStandardSID       = "standard_sid"
	ProcedureStandardSTAR      = "standard_star"
	ProcedureContinuousDescent = "continuous_descent"
)

const (
	ThrustTakeoff = "takeoff"
	ThrustReduced = "reduced"
	ThrustIdle    = "idle"
)

var allowedOperations = map[string]struct{}{
	OperationDeparture: {},
	OperationArrival:   {},
}

var allowedAircraftClasses = map[string]struct{}{
	AircraftClassRegional: {},
	AircraftClassNarrow:   {},
	AircraftClassWide:     {},
	AircraftClassCargo:    {},
}

var allowedProcedureTypes = map[string]struct{}{
	ProcedureStandardSID:       {},
	ProcedureStandardSTAR:      {},
	ProcedureContinuousDescent: {},
}

var allowedThrustModes = map[string]struct{}{
	ThrustTakeoff: {},
	ThrustReduced: {},
	ThrustIdle:    {},
}

// MovementPeriod stores normalized aircraft movements for one time period.
type MovementPeriod struct {
	MovementsPerHour float64 `json:"movements_per_hour"`
}

// AirportRef identifies the airport/runway context for one source.
type AirportRef struct {
	AirportID string `json:"airport_id"`
	RunwayID  string `json:"runway_id"`
}

// AircraftSource describes one aircraft trajectory source.
type AircraftSource struct {
	ID                    string         `json:"id"`
	SourceType            string         `json:"source_type"`
	Airport               AirportRef     `json:"airport"`
	OperationType         string         `json:"operation_type"`
	AircraftClass         string         `json:"aircraft_class"`
	ProcedureType         string         `json:"procedure_type"`
	ThrustMode            string         `json:"thrust_mode"`
	FlightTrack           []geo.Point3D  `json:"flight_track"`
	LateralOffsetM        float64        `json:"lateral_offset_m,omitempty"`
	ReferencePowerLevelDB float64        `json:"reference_power_level_db"`
	EngineStateFactor     float64        `json:"engine_state_factor"`
	BankAngleDeg          float64        `json:"bank_angle_deg,omitempty"`
	MovementDay           MovementPeriod `json:"movement_day"`
	MovementEvening       MovementPeriod `json:"movement_evening"`
	MovementNight         MovementPeriod `json:"movement_night"`
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

// Validate validates one aircraft source payload.
func (s AircraftSource) Validate() error {
	if err := s.validateIdentity(); err != nil {
		return err
	}

	if err := s.validateClassification(); err != nil {
		return err
	}

	if err := s.validateTrack(); err != nil {
		return err
	}

	if err := s.validateAcoustics(); err != nil {
		return err
	}

	return s.validateMovements()
}

func (s AircraftSource) validateIdentity() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("aircraft source id is required")
	}

	if strings.TrimSpace(s.SourceType) != SourceTypeLine {
		return fmt.Errorf("aircraft source %q has unsupported source_type %q", s.ID, s.SourceType)
	}

	if strings.TrimSpace(s.Airport.AirportID) == "" {
		return fmt.Errorf("aircraft source %q airport.airport_id is required", s.ID)
	}

	if strings.TrimSpace(s.Airport.RunwayID) == "" {
		return fmt.Errorf("aircraft source %q airport.runway_id is required", s.ID)
	}

	return nil
}

func (s AircraftSource) validateClassification() error {
	if _, ok := allowedOperations[strings.TrimSpace(s.OperationType)]; !ok {
		return fmt.Errorf("aircraft source %q has unsupported operation_type %q", s.ID, s.OperationType)
	}

	if _, ok := allowedAircraftClasses[strings.TrimSpace(s.AircraftClass)]; !ok {
		return fmt.Errorf("aircraft source %q has unsupported aircraft_class %q", s.ID, s.AircraftClass)
	}

	if _, ok := allowedProcedureTypes[strings.TrimSpace(s.ProcedureType)]; !ok {
		return fmt.Errorf("aircraft source %q has unsupported procedure_type %q", s.ID, s.ProcedureType)
	}

	if _, ok := allowedThrustModes[strings.TrimSpace(s.ThrustMode)]; !ok {
		return fmt.Errorf("aircraft source %q has unsupported thrust_mode %q", s.ID, s.ThrustMode)
	}

	return nil
}

func (s AircraftSource) validateTrack() error {
	if len(s.FlightTrack) < 2 {
		return fmt.Errorf("aircraft source %q flight_track must contain at least 2 points", s.ID)
	}

	for i, point := range s.FlightTrack {
		if !point.IsFinite() {
			return fmt.Errorf("aircraft source %q flight_track point[%d] is not finite", s.ID, i)
		}
	}

	return nil
}

func (s AircraftSource) validateAcoustics() error {
	if !finite(s.ReferencePowerLevelDB) {
		return fmt.Errorf("aircraft source %q reference_power_level_db must be finite", s.ID)
	}

	if !finite(s.LateralOffsetM) {
		return fmt.Errorf("aircraft source %q lateral_offset_m must be finite", s.ID)
	}

	if !finitePositive(s.EngineStateFactor) {
		return fmt.Errorf("aircraft source %q engine_state_factor must be finite and > 0", s.ID)
	}

	if !finite(s.BankAngleDeg) {
		return fmt.Errorf("aircraft source %q bank_angle_deg must be finite", s.ID)
	}

	return nil
}

func (s AircraftSource) validateMovements() error {
	if err := validateMovementPeriod(s.ID, "day", s.MovementDay); err != nil {
		return err
	}

	if err := validateMovementPeriod(s.ID, "evening", s.MovementEvening); err != nil {
		return err
	}

	return validateMovementPeriod(s.ID, "night", s.MovementNight)
}

func validateMovementPeriod(sourceID string, period string, movement MovementPeriod) error {
	if !finiteNonNegative(movement.MovementsPerHour) {
		return fmt.Errorf("aircraft source %q movement_%s movements_per_hour must be finite and >= 0", sourceID, period)
	}

	return nil
}

// Descriptor returns the standards-framework descriptor for CNOSSOS aircraft.
func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             StandardID,
		Description:    "Scaffold aircraft module: typed flight-track sources with deterministic indicators only, with no NPD curves and no ECAC Doc 29 flight profiles; CNOSSOS-EU (Directive 2015/996 Annex II) defines no aircraft method at all and covers road, rail and industry only, so this id names a method its standard does not contain.",
		EvidenceTier:   framework.EvidenceTierScaffold,
		DefaultVersion: "2020-preview",
		Versions: []framework.Version{
			{
				Name:           "2020-preview",
				DefaultProfile: "airport-vicinity",
				Profiles: []framework.Profile{
					{
						Name:                 "airport-vicinity",
						SupportedSourceTypes: []string{SourceTypeLine},
						SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "25", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "airport_id", Kind: framework.ParameterKindString, DefaultValue: "APT", Description: "Airport identifier for imported aircraft sources"},
								{Name: "runway_id", Kind: framework.ParameterKindString, DefaultValue: "RWY", Description: "Runway identifier for imported aircraft sources"},
								{Name: "aircraft_operation_type", Kind: framework.ParameterKindString, DefaultValue: OperationDeparture, Enum: []string{OperationDeparture, OperationArrival}, Description: "Operation type for imported aircraft sources"},
								{Name: "aircraft_class", Kind: framework.ParameterKindString, DefaultValue: AircraftClassNarrow, Enum: []string{AircraftClassRegional, AircraftClassNarrow, AircraftClassWide, AircraftClassCargo}, Description: "Aircraft class for imported aircraft sources"},
								{Name: "aircraft_procedure_type", Kind: framework.ParameterKindString, DefaultValue: ProcedureStandardSID, Enum: []string{ProcedureStandardSID, ProcedureStandardSTAR, ProcedureContinuousDescent}, Description: "Procedure context for imported aircraft sources"},
								{Name: "aircraft_thrust_mode", Kind: framework.ParameterKindString, DefaultValue: ThrustTakeoff, Enum: []string{ThrustTakeoff, ThrustReduced, ThrustIdle}, Description: "Thrust state for imported aircraft sources"},
								{Name: "reference_power_level_db", Kind: framework.ParameterKindFloat, DefaultValue: "108", Description: "Reference sound power level for imported aircraft sources"},
								{Name: "engine_state_factor", Kind: framework.ParameterKindFloat, DefaultValue: "1.0", Min: &minPositive, Description: "Engine state multiplier for imported aircraft sources"},
								{Name: "bank_angle_deg", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Bank angle used for directivity adjustment"},
								{Name: "lateral_offset_m", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Lateral offset of the procedure relative to runway centerline"},
								{Name: "track_start_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Start altitude of imported flight tracks"},
								{Name: "track_end_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "300", Min: &minZero, Description: "End altitude of imported flight tracks"},
								{Name: "movement_day_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minZero, Description: "Day aircraft movements per hour"},
								{Name: "movement_evening_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "5", Min: &minZero, Description: "Evening aircraft movements per hour"},
								{Name: "movement_night_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Night aircraft movements per hour"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "0.8", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "lateral_directivity_db", Kind: framework.ParameterKindFloat, DefaultValue: "0", Description: "Lateral directivity adjustment"},
								{Name: "approach_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Arrival correction term"},
								{Name: "climb_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "2.5", Min: &minZero, Description: "Departure climb correction term"},
								{Name: "min_slant_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "20", Min: &minPositive, Description: "Minimum slant propagation distance"},
							},
						},
					},
				},
			},
		},
	}
}
