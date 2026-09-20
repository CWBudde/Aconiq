package cli

import (
	"github.com/aconiq/backend/internal/standards/schall03"
)

// paramBinding is one normalized run-parameter name plus the constructors every
// site that consumes it needs.
//
// A parameter name used to be written three times over: once where the standards
// module declares it in its published parameter schema, once where the CLI parses
// it into a run-options field, and once where the module lists it as a provenance
// key. Nothing tied the three together, so a typo in any one of them failed
// silently — the run kept its default, the provenance said nothing, and no test
// noticed. Routing every site through one binding makes that impossible by
// construction, and TestRunOptionsCoverParameterSchema pins the remaining gap
// between the CLI tables and the published schemas.
//
// The methods build a boundParam, which is what the per-standard tables below are
// made of. They are deliberately not generic: the kind of a parameter is part of
// its contract, and picking the wrong one is a compile error rather than a
// run-time surprise.
type paramBinding string

// key returns the normalized parameter name this binding carries.
func (p paramBinding) key() string {
	return string(p)
}

// float binds the parameter to a float field, requiring a finite value.
func (p paramBinding) float(target *float64) boundParam {
	return boundParam{key: string(p), apply: func(scope string, params map[string]string) error {
		return parseFiniteFloatParam(scope, params, string(p), target)
	}}
}

// minFloat binds the parameter to a float field, requiring a finite value at or
// above minValue.
func (p paramBinding) minFloat(target *float64, minValue float64) boundParam {
	return boundParam{key: string(p), apply: func(scope string, params map[string]string) error {
		return parseMinFloatParam(scope, params, string(p), target, minValue)
	}}
}

// minInt binds the parameter to an int field, requiring a value at or above
// minValue.
func (p paramBinding) minInt(target *int, minValue int) boundParam {
	return boundParam{key: string(p), apply: func(scope string, params map[string]string) error {
		return parseMinIntParam(scope, params, string(p), target, minValue)
	}}
}

// str binds the parameter to a string field.
func (p paramBinding) str(target *string) boundParam {
	return boundParam{key: string(p), apply: func(scope string, params map[string]string) error {
		value, err := stringParamValue(scope, params, string(p))
		if err != nil {
			return err
		}

		*target = value

		return nil
	}}
}

// boolean binds the parameter to a bool field.
func (p paramBinding) boolean(target *bool) boundParam {
	return boundParam{key: string(p), apply: func(scope string, params map[string]string) error {
		return parseBoolParam(scope, params, string(p), target)
	}}
}

// runParams is the CLI's run-parameter vocabulary: every normalized parameter
// name the run pipeline binds, each spelled exactly once.
//
// This is one table, not a bag of constants. Every entry is a paramBinding, so
// the parse tables, the feature-property override tables and the property maps
// the importers write all derive their spelling from the same value; there is no
// second place a name can be typed differently. Adding a parameter to a standards
// module means adding it here and binding it in that standard's table below —
// TestRunOptionsCoverParameterSchema fails otherwise.
var runParams = struct {
	// Receiver grid and engine controls.
	GridResolutionM, GridPaddingM, ReceiverHeightM paramBinding
	SourceEmissionDB, ChunkSize, Workers           paramBinding
	DisableCache                                   paramBinding

	// Propagation terms shared across the road, rail, industry and aircraft
	// modules.
	AirAbsorptionDBPerKM, GroundAttenuationDB paramBinding
	MinDistanceM, MinSlantDistanceM           paramBinding

	// Road source and corridor terms.
	RoadCategory, RoadSurfaceType, RoadFunctionClass paramBinding
	RoadSpeedKPH, RoadGradientPercent                paramBinding
	RoadJunctionType, RoadJunctionDistanceM          paramBinding
	RoadTemperatureC, RoadStuddedTyreShare           paramBinding
	BarrierAttenuationDB, UrbanCanyonDB              paramBinding
	IntersectionDensityPerKM                         paramBinding

	// Hourly road traffic by period and vehicle class.
	TrafficDayLightVPH, TrafficDayMediumVPH         paramBinding
	TrafficDayHeavyVPH, TrafficDayPTWVPH            paramBinding
	TrafficEveningLightVPH, TrafficEveningMediumVPH paramBinding
	TrafficEveningHeavyVPH, TrafficEveningPTWVPH    paramBinding
	TrafficNightLightVPH, TrafficNightMediumVPH     paramBinding
	TrafficNightHeavyVPH, TrafficNightPTWVPH        paramBinding

	// Rail infrastructure, rolling stock and corrections.
	RailTrainClass, RailTractionType           paramBinding
	RailTrackType, RailTrackForm               paramBinding
	RailTrackRoughnessClass                    paramBinding
	RailAverageTrainSpeedKPH, RailBrakingShare paramBinding
	RailCurveRadiusM, RailOnBridge             paramBinding
	BridgeCorrectionDB, CurveSquealDB          paramBinding
	SlabTrackCorrectionDB, CurveCorrectionDB   paramBinding
	TrafficDayTrainsPerHour                    paramBinding
	TrafficEveningTrainsPerHour                paramBinding
	TrafficNightTrainsPerHour                  paramBinding
	Schall03Engine                             paramBinding

	// Industry source terms.
	IndustrySourceCategory, IndustryEnclosureState   paramBinding
	IndustrySoundPowerLevelDB, IndustrySourceHeightM paramBinding
	IndustryTonalityCorrectionDB                     paramBinding
	IndustryImpulsivityCorrectionDB                  paramBinding
	OperationDayFactor, OperationEveningFactor       paramBinding
	OperationNightFactor                             paramBinding
	ScreeningAttenuationDB, FacadeReflectionDB       paramBinding

	// Aircraft operations and procedure terms.
	AirportID, RunwayID                        paramBinding
	AircraftOperationType, AircraftClass       paramBinding
	AircraftProcedureType, AircraftThrustMode  paramBinding
	ReferencePowerLevelDB, EngineStateFactor   paramBinding
	BankAngleDeg, LateralOffsetM               paramBinding
	TrackStartHeightM, TrackEndHeightM         paramBinding
	MovementDayPerHour, MovementEveningPerHour paramBinding
	MovementNightPerHour                       paramBinding
	LateralDirectivityDB, ApproachCorrectionDB paramBinding
	ClimbCorrectionDB                          paramBinding

	// RLS-19 road: its own speed, gradient and traffic vocabulary.
	SurfaceType, SegmentLengthM        paramBinding
	SegmentLengthMode                  paramBinding
	SpeedPkwKPH, SpeedLkw1KPH          paramBinding
	SpeedLkw2KPH, SpeedKradKPH         paramBinding
	GradientPercent                    paramBinding
	TrafficDayPkw, TrafficDayLkw1      paramBinding
	TrafficDayLkw2, TrafficDayKrad     paramBinding
	TrafficNightPkw, TrafficNightLkw1  paramBinding
	TrafficNightLkw2, TrafficNightKrad paramBinding
	JunctionType, JunctionDistanceM    paramBinding
	BuildingHeightM, StreetWidthM      paramBinding

	// BEB exposure aggregation terms.
	UpstreamMappingStandard, BuildingUsageType paramBinding
	MinimumBuildingHeightM, FloorHeightM       paramBinding
	DwellingsPerFloor, PersonsPerDwelling      paramBinding
	ThresholdLdenDB, ThresholdLnightDB         paramBinding
	OccupancyMode, FacadeEvaluationMode        paramBinding
	FacadeReceiverHeightM                      paramBinding

	// ISO 9613-2 point-source and atmosphere terms.
	ISO9613SourceHeightM, ISO9613SoundPowerLevelDB paramBinding
	ISO9613DirectivityCorrectionDB                 paramBinding
	ISO9613TonalityCorrectionDB                    paramBinding
	ISO9613ImpulsivityCorrectionDB                 paramBinding
	GroundFactor, AirTemperatureC                  paramBinding
	RelativeHumidityPercent, MeteorologyAssumption paramBinding
	C0Met                                          paramBinding
}{
	GridResolutionM:  "grid_resolution_m",
	GridPaddingM:     "grid_padding_m",
	ReceiverHeightM:  "receiver_height_m",
	SourceEmissionDB: "source_emission_db",
	ChunkSize:        "chunk_size",
	Workers:          "workers",
	DisableCache:     "disable_cache",

	AirAbsorptionDBPerKM: "air_absorption_db_per_km",
	GroundAttenuationDB:  "ground_attenuation_db",
	MinDistanceM:         "min_distance_m",
	MinSlantDistanceM:    "min_slant_distance_m",

	RoadCategory:             "road_category",
	RoadSurfaceType:          "road_surface_type",
	RoadFunctionClass:        "road_function_class",
	RoadSpeedKPH:             "road_speed_kph",
	RoadGradientPercent:      "road_gradient_percent",
	RoadJunctionType:         "road_junction_type",
	RoadJunctionDistanceM:    "road_junction_distance_m",
	RoadTemperatureC:         "road_temperature_c",
	RoadStuddedTyreShare:     "road_studded_tyre_share",
	BarrierAttenuationDB:     "barrier_attenuation_db",
	UrbanCanyonDB:            "urban_canyon_db",
	IntersectionDensityPerKM: "intersection_density_per_km",

	TrafficDayLightVPH:      "traffic_day_light_vph",
	TrafficDayMediumVPH:     "traffic_day_medium_vph",
	TrafficDayHeavyVPH:      "traffic_day_heavy_vph",
	TrafficDayPTWVPH:        "traffic_day_ptw_vph",
	TrafficEveningLightVPH:  "traffic_evening_light_vph",
	TrafficEveningMediumVPH: "traffic_evening_medium_vph",
	TrafficEveningHeavyVPH:  "traffic_evening_heavy_vph",
	TrafficEveningPTWVPH:    "traffic_evening_ptw_vph",
	TrafficNightLightVPH:    "traffic_night_light_vph",
	TrafficNightMediumVPH:   "traffic_night_medium_vph",
	TrafficNightHeavyVPH:    "traffic_night_heavy_vph",
	TrafficNightPTWVPH:      "traffic_night_ptw_vph",

	RailTrainClass:              "rail_train_class",
	RailTractionType:            "rail_traction_type",
	RailTrackType:               "rail_track_type",
	RailTrackForm:               "rail_track_form",
	RailTrackRoughnessClass:     "rail_track_roughness_class",
	RailAverageTrainSpeedKPH:    "rail_average_train_speed_kph",
	RailBrakingShare:            "rail_braking_share",
	RailCurveRadiusM:            "rail_curve_radius_m",
	RailOnBridge:                "rail_on_bridge",
	BridgeCorrectionDB:          "bridge_correction_db",
	CurveSquealDB:               "curve_squeal_db",
	SlabTrackCorrectionDB:       "slab_track_correction_db",
	CurveCorrectionDB:           "curve_correction_db",
	TrafficDayTrainsPerHour:     "traffic_day_trains_per_hour",
	TrafficEveningTrainsPerHour: "traffic_evening_trains_per_hour",
	TrafficNightTrainsPerHour:   "traffic_night_trains_per_hour",
	Schall03Engine:              paramBinding(schall03.ParamEngine),

	IndustrySourceCategory:          "industry_source_category",
	IndustryEnclosureState:          "industry_enclosure_state",
	IndustrySoundPowerLevelDB:       "industry_sound_power_level_db",
	IndustrySourceHeightM:           "industry_source_height_m",
	IndustryTonalityCorrectionDB:    "industry_tonality_correction_db",
	IndustryImpulsivityCorrectionDB: "industry_impulsivity_correction_db",
	OperationDayFactor:              "operation_day_factor",
	OperationEveningFactor:          "operation_evening_factor",
	OperationNightFactor:            "operation_night_factor",
	ScreeningAttenuationDB:          "screening_attenuation_db",
	FacadeReflectionDB:              "facade_reflection_db",

	AirportID:              "airport_id",
	RunwayID:               "runway_id",
	AircraftOperationType:  "aircraft_operation_type",
	AircraftClass:          "aircraft_class",
	AircraftProcedureType:  "aircraft_procedure_type",
	AircraftThrustMode:     "aircraft_thrust_mode",
	ReferencePowerLevelDB:  "reference_power_level_db",
	EngineStateFactor:      "engine_state_factor",
	BankAngleDeg:           "bank_angle_deg",
	LateralOffsetM:         "lateral_offset_m",
	TrackStartHeightM:      "track_start_height_m",
	TrackEndHeightM:        "track_end_height_m",
	MovementDayPerHour:     "movement_day_per_hour",
	MovementEveningPerHour: "movement_evening_per_hour",
	MovementNightPerHour:   "movement_night_per_hour",
	LateralDirectivityDB:   "lateral_directivity_db",
	ApproachCorrectionDB:   "approach_correction_db",
	ClimbCorrectionDB:      "climb_correction_db",

	SurfaceType:       "surface_type",
	SegmentLengthM:    "segment_length_m",
	SegmentLengthMode: "segment_length_mode",
	SpeedPkwKPH:       "speed_pkw_kph",
	SpeedLkw1KPH:      "speed_lkw1_kph",
	SpeedLkw2KPH:      "speed_lkw2_kph",
	SpeedKradKPH:      "speed_krad_kph",
	GradientPercent:   "gradient_percent",
	TrafficDayPkw:     "traffic_day_pkw",
	TrafficDayLkw1:    "traffic_day_lkw1",
	TrafficDayLkw2:    "traffic_day_lkw2",
	TrafficDayKrad:    "traffic_day_krad",
	TrafficNightPkw:   "traffic_night_pkw",
	TrafficNightLkw1:  "traffic_night_lkw1",
	TrafficNightLkw2:  "traffic_night_lkw2",
	TrafficNightKrad:  "traffic_night_krad",
	JunctionType:      "junction_type",
	JunctionDistanceM: "junction_distance_m",
	BuildingHeightM:   "building_height_m",
	StreetWidthM:      "street_width_m",

	UpstreamMappingStandard: "upstream_mapping_standard",
	BuildingUsageType:       "building_usage_type",
	MinimumBuildingHeightM:  "minimum_building_height_m",
	FloorHeightM:            "floor_height_m",
	DwellingsPerFloor:       "dwellings_per_floor",
	PersonsPerDwelling:      "persons_per_dwelling",
	ThresholdLdenDB:         "threshold_lden_db",
	ThresholdLnightDB:       "threshold_lnight_db",
	OccupancyMode:           "occupancy_mode",
	FacadeEvaluationMode:    "facade_evaluation_mode",
	FacadeReceiverHeightM:   "facade_receiver_height_m",

	ISO9613SourceHeightM:           "iso9613_source_height_m",
	ISO9613SoundPowerLevelDB:       "iso9613_sound_power_level_db",
	ISO9613DirectivityCorrectionDB: "iso9613_directivity_correction_db",
	ISO9613TonalityCorrectionDB:    "iso9613_tonality_correction_db",
	ISO9613ImpulsivityCorrectionDB: "iso9613_impulsivity_correction_db",
	GroundFactor:                   "ground_factor",
	AirTemperatureC:                "air_temperature_c",
	RelativeHumidityPercent:        "relative_humidity_percent",
	MeteorologyAssumption:          "meteorology_assumption",
	C0Met:                          "c0_met",
}
