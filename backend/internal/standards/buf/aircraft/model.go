package aircraft

import "github.com/aconiq/backend/internal/standards/framework"

func Descriptor() framework.StandardDescriptor {
	minZero := 0.0
	minPositive := 0.001

	return framework.StandardDescriptor{
		Context:        framework.StandardContextMapping,
		ID:             StandardID,
		Description:    "Scaffold aircraft mapping module, a near-copy of cnossos-aircraft: typed flight-track sources with deterministic indicators only, with no NPD curves, no ECAC Doc 29 flight profiles and no BUF procedure; CNOSSOS-EU (Directive 2015/996 Annex II) defines no aircraft method at all and covers road, rail and industry only.",
		EvidenceTier:   framework.EvidenceTierScaffold,
		DefaultVersion: "2021-preview",
		Versions: []framework.Version{
			{
				Name:           "2021-preview",
				DefaultProfile: "strategic-mapping",
				Profiles: []framework.Profile{
					{
						Name:                 "strategic-mapping",
						SupportedSourceTypes: []string{SourceTypeLine},
						SupportedIndicators:  []string{IndicatorLday, IndicatorLevening, IndicatorLnight, IndicatorLden},
						ParameterSchema: framework.ParameterSchema{
							Parameters: []framework.ParameterDefinition{
								{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "25", Min: &minPositive, Description: "Receiver grid spacing in meters"},
								{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "100", Min: &minZero, Description: "Padding around source extent in meters"},
								{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
								{Name: "airport_id", Kind: framework.ParameterKindString, DefaultValue: "DE-APT", Description: "Airport identifier for imported aircraft sources"},
								{Name: "runway_id", Kind: framework.ParameterKindString, DefaultValue: "RWY", Description: "Runway identifier for imported aircraft sources"},
								{Name: "aircraft_operation_type", Kind: framework.ParameterKindString, DefaultValue: OperationDeparture, Enum: []string{OperationDeparture, OperationArrival}, Description: "Operation type for imported aircraft sources"},
								{Name: "aircraft_class", Kind: framework.ParameterKindString, DefaultValue: AircraftClassNarrow, Enum: []string{AircraftClassRegional, AircraftClassNarrow, AircraftClassWide, AircraftClassCargo}, Description: "Aircraft class for imported aircraft sources"},
								{Name: "aircraft_procedure_type", Kind: framework.ParameterKindString, DefaultValue: ProcedureStandardSID, Enum: []string{ProcedureStandardSID, ProcedureStandardSTAR, ProcedureContinuousDescent}, Description: "Procedure context for imported aircraft sources"},
								{Name: "aircraft_thrust_mode", Kind: framework.ParameterKindString, DefaultValue: ThrustTakeoff, Enum: []string{ThrustTakeoff, ThrustReduced, ThrustIdle}, Description: "Thrust state for imported aircraft sources"},
								{Name: "reference_power_level_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "110", Description: "Reference sound power level for imported aircraft sources"},
								{Name: "engine_state_factor", Kind: framework.ParameterKindFloat, DefaultValue: "1.0", Min: &minPositive, Description: "Engine state multiplier for imported aircraft sources"},
								{Name: "bank_angle_deg", Kind: framework.ParameterKindFloat, Unit: framework.UnitDegree, DefaultValue: "0", Description: "Bank angle used for directivity adjustment"},
								{Name: "lateral_offset_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "0", Description: "Lateral offset of the procedure relative to runway centerline"},
								{Name: "track_start_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "20", Min: &minZero, Description: "Start altitude of imported flight tracks"},
								{Name: "track_end_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "250", Min: &minZero, Description: "End altitude of imported flight tracks"},
								{Name: "movement_day_per_hour", Kind: framework.ParameterKindFloat, Unit: framework.UnitPerHour, DefaultValue: "12", Min: &minZero, Description: "Day aircraft movements per hour"},
								{Name: "movement_evening_per_hour", Kind: framework.ParameterKindFloat, Unit: framework.UnitPerHour, DefaultValue: "6", Min: &minZero, Description: "Evening aircraft movements per hour"},
								{Name: "movement_night_per_hour", Kind: framework.ParameterKindFloat, Unit: framework.UnitPerHour, DefaultValue: "2", Min: &minZero, Description: "Night aircraft movements per hour"},
								{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibelPerKilometer, DefaultValue: "0.7", Min: &minZero, Description: "Air absorption term"},
								{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0.8", Min: &minZero, Description: "Ground attenuation term"},
								{Name: "lateral_directivity_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "1.0", Description: "Lateral directivity adjustment"},
								{Name: "approach_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "1.5", Min: &minZero, Description: "Arrival correction term"},
								{Name: "climb_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "2.5", Min: &minZero, Description: "Departure climb correction term"},
								{Name: "min_slant_distance_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "20", Min: &minPositive, Description: "Minimum slant propagation distance"},
							},
						},
					},
				},
			},
		},
	}
}
