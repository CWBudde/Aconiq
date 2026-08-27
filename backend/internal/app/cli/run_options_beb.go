package cli

import (
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
)

type bebExposureRunOptions struct {
	UpstreamMappingStandard  string
	BuildingUsageType        string
	MinimumBuildingHeightM   float64
	FloorHeightM             float64
	DwellingsPerFloor        float64
	PersonsPerDwelling       float64
	ThresholdLdenDB          float64
	ThresholdLnightDB        float64
	OccupancyMode            string
	FacadeEvaluationMode     string
	FacadeReceiverHeightM    float64
	SurfaceType              string
	RoadFunctionClass        string
	SpeedKPH                 float64
	GradientPercent          float64
	JunctionType             string
	JunctionDistanceM        float64
	TemperatureC             float64
	StuddedTyreShare         float64
	TrafficDayLightVPH       float64
	TrafficDayMediumVPH      float64
	TrafficDayHeavyVPH       float64
	TrafficDayPTWVPH         float64
	TrafficEveningLightVPH   float64
	TrafficEveningMediumVPH  float64
	TrafficEveningHeavyVPH   float64
	TrafficEveningPTWVPH     float64
	TrafficNightLightVPH     float64
	TrafficNightMediumVPH    float64
	TrafficNightHeavyVPH     float64
	TrafficNightPTWVPH       float64
	AirAbsorptionDBPerKM     float64
	GroundAttenuationDB      float64
	UrbanCanyonDB            float64
	IntersectionDensityPerKM float64
	MinDistanceM             float64
	AirportID                string
	RunwayID                 string
	OperationType            string
	AircraftClass            string
	ProcedureType            string
	ThrustMode               string
	ReferencePowerLevelDB    float64
	EngineStateFactor        float64
	BankAngleDeg             float64
	LateralOffsetM           float64
	TrackStartHeightM        float64
	TrackEndHeightM          float64
	MovementDayPerHour       float64
	MovementEveningPerHour   float64
	MovementNightPerHour     float64
	LateralDirectivityDB     float64
	ApproachCorrectionDB     float64
	ClimbCorrectionDB        float64
	MinSlantDistanceM        float64
}

func parseBEBExposureRunOptions(params map[string]string) (bebExposureRunOptions, error) {
	const scope = "cli.parseBEBExposureRunOptions"

	options := bebExposureRunOptions{}

	err := parseFiniteFloatParams(scope, params, []floatParam{
		{"minimum_building_height_m", &options.MinimumBuildingHeightM},
		{"floor_height_m", &options.FloorHeightM},
		{"dwellings_per_floor", &options.DwellingsPerFloor},
		{"persons_per_dwelling", &options.PersonsPerDwelling},
		{"threshold_lden_db", &options.ThresholdLdenDB},
		{"threshold_lnight_db", &options.ThresholdLnightDB},
		{"facade_receiver_height_m", &options.FacadeReceiverHeightM},
		{"road_speed_kph", &options.SpeedKPH},
		{"road_gradient_percent", &options.GradientPercent},
		{"road_junction_distance_m", &options.JunctionDistanceM},
		{"road_temperature_c", &options.TemperatureC},
		{"road_studded_tyre_share", &options.StuddedTyreShare},
		{"traffic_day_light_vph", &options.TrafficDayLightVPH},
		{"traffic_day_medium_vph", &options.TrafficDayMediumVPH},
		{"traffic_day_heavy_vph", &options.TrafficDayHeavyVPH},
		{"traffic_day_ptw_vph", &options.TrafficDayPTWVPH},
		{"traffic_evening_light_vph", &options.TrafficEveningLightVPH},
		{"traffic_evening_medium_vph", &options.TrafficEveningMediumVPH},
		{"traffic_evening_heavy_vph", &options.TrafficEveningHeavyVPH},
		{"traffic_evening_ptw_vph", &options.TrafficEveningPTWVPH},
		{"traffic_night_light_vph", &options.TrafficNightLightVPH},
		{"traffic_night_medium_vph", &options.TrafficNightMediumVPH},
		{"traffic_night_heavy_vph", &options.TrafficNightHeavyVPH},
		{"traffic_night_ptw_vph", &options.TrafficNightPTWVPH},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"urban_canyon_db", &options.UrbanCanyonDB},
		{"intersection_density_per_km", &options.IntersectionDensityPerKM},
		{"min_distance_m", &options.MinDistanceM},
		{"reference_power_level_db", &options.ReferencePowerLevelDB},
		{"engine_state_factor", &options.EngineStateFactor},
		{"bank_angle_deg", &options.BankAngleDeg},
		{"track_start_height_m", &options.TrackStartHeightM},
		{"track_end_height_m", &options.TrackEndHeightM},
		{"movement_day_per_hour", &options.MovementDayPerHour},
		{"movement_evening_per_hour", &options.MovementEveningPerHour},
		{"movement_night_per_hour", &options.MovementNightPerHour},
		{"lateral_directivity_db", &options.LateralDirectivityDB},
		{"approach_correction_db", &options.ApproachCorrectionDB},
		{"climb_correction_db", &options.ClimbCorrectionDB},
		{"min_slant_distance_m", &options.MinSlantDistanceM},
	})
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	err = assignStringParams(scope, params, []stringParam{
		{"upstream_mapping_standard", &options.UpstreamMappingStandard},
		{"building_usage_type", &options.BuildingUsageType},
		{"occupancy_mode", &options.OccupancyMode},
		{"facade_evaluation_mode", &options.FacadeEvaluationMode},
		{"road_surface_type", &options.SurfaceType},
		{"road_function_class", &options.RoadFunctionClass},
		{"road_junction_type", &options.JunctionType},
		{"airport_id", &options.AirportID},
		{"runway_id", &options.RunwayID},
		{"aircraft_operation_type", &options.OperationType},
		{"aircraft_class", &options.AircraftClass},
		{"aircraft_procedure_type", &options.ProcedureType},
		{"aircraft_thrust_mode", &options.ThrustMode},
	})
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	return options, nil
}

func (o bebExposureRunOptions) BUBRoadOptions() bubRoadRunOptions {
	return bubRoadRunOptions{
		SurfaceType:              o.SurfaceType,
		RoadFunctionClass:        o.RoadFunctionClass,
		SpeedKPH:                 o.SpeedKPH,
		GradientPercent:          o.GradientPercent,
		JunctionType:             o.JunctionType,
		JunctionDistanceM:        o.JunctionDistanceM,
		TemperatureC:             o.TemperatureC,
		StuddedTyreShare:         o.StuddedTyreShare,
		TrafficDayLightVPH:       o.TrafficDayLightVPH,
		TrafficDayMediumVPH:      o.TrafficDayMediumVPH,
		TrafficDayHeavyVPH:       o.TrafficDayHeavyVPH,
		TrafficDayPTWVPH:         o.TrafficDayPTWVPH,
		TrafficEveningLightVPH:   o.TrafficEveningLightVPH,
		TrafficEveningMediumVPH:  o.TrafficEveningMediumVPH,
		TrafficEveningHeavyVPH:   o.TrafficEveningHeavyVPH,
		TrafficEveningPTWVPH:     o.TrafficEveningPTWVPH,
		TrafficNightLightVPH:     o.TrafficNightLightVPH,
		TrafficNightMediumVPH:    o.TrafficNightMediumVPH,
		TrafficNightHeavyVPH:     o.TrafficNightHeavyVPH,
		TrafficNightPTWVPH:       o.TrafficNightPTWVPH,
		AirAbsorptionDBPerKM:     o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:      o.GroundAttenuationDB,
		UrbanCanyonDB:            o.UrbanCanyonDB,
		IntersectionDensityPerKM: o.IntersectionDensityPerKM,
		MinDistanceM:             o.MinDistanceM,
	}
}

func (o bebExposureRunOptions) ExposureConfig() bebexposure.ExposureConfig {
	return bebexposure.ExposureConfig{
		FloorHeightM:            o.FloorHeightM,
		DwellingsPerFloor:       o.DwellingsPerFloor,
		PersonsPerDwelling:      o.PersonsPerDwelling,
		ThresholdLdenDB:         o.ThresholdLdenDB,
		ThresholdLnightDB:       o.ThresholdLnightDB,
		OccupancyMode:           o.OccupancyMode,
		FacadeEvaluationMode:    o.FacadeEvaluationMode,
		UpstreamMappingStandard: o.UpstreamMappingStandard,
	}
}

func (o bebExposureRunOptions) BUFAircraftOptions() bufAircraftRunOptions {
	return bufAircraftRunOptions{
		AirportID:              o.AirportID,
		RunwayID:               o.RunwayID,
		OperationType:          o.OperationType,
		AircraftClass:          o.AircraftClass,
		ProcedureType:          o.ProcedureType,
		ThrustMode:             o.ThrustMode,
		ReferencePowerLevelDB:  o.ReferencePowerLevelDB,
		EngineStateFactor:      o.EngineStateFactor,
		BankAngleDeg:           o.BankAngleDeg,
		LateralOffsetM:         o.LateralOffsetM,
		TrackStartHeightM:      o.TrackStartHeightM,
		TrackEndHeightM:        o.TrackEndHeightM,
		MovementDayPerHour:     o.MovementDayPerHour,
		MovementEveningPerHour: o.MovementEveningPerHour,
		MovementNightPerHour:   o.MovementNightPerHour,
		AirAbsorptionDBPerKM:   o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:    o.GroundAttenuationDB,
		LateralDirectivityDB:   o.LateralDirectivityDB,
		ApproachCorrectionDB:   o.ApproachCorrectionDB,
		ClimbCorrectionDB:      o.ClimbCorrectionDB,
		MinSlantDistanceM:      o.MinSlantDistanceM,
	}
}
