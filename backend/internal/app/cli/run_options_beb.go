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

// bebExposureParamBindings binds the beb-exposure parameter schema. The module
// aggregates upstream road and aircraft levels, so its schema republishes both
// vocabularies alongside its own exposure terms.
func bebExposureParamBindings(options *bebExposureRunOptions) []boundParam {
	return []boundParam{
		runParams.MinimumBuildingHeightM.float(&options.MinimumBuildingHeightM),
		runParams.FloorHeightM.float(&options.FloorHeightM),
		runParams.DwellingsPerFloor.float(&options.DwellingsPerFloor),
		runParams.PersonsPerDwelling.float(&options.PersonsPerDwelling),
		runParams.ThresholdLdenDB.float(&options.ThresholdLdenDB),
		runParams.ThresholdLnightDB.float(&options.ThresholdLnightDB),
		runParams.FacadeReceiverHeightM.float(&options.FacadeReceiverHeightM),
		runParams.RoadSpeedKPH.float(&options.SpeedKPH),
		runParams.RoadGradientPercent.float(&options.GradientPercent),
		runParams.RoadJunctionDistanceM.float(&options.JunctionDistanceM),
		runParams.RoadTemperatureC.float(&options.TemperatureC),
		runParams.RoadStuddedTyreShare.float(&options.StuddedTyreShare),
		runParams.TrafficDayLightVPH.float(&options.TrafficDayLightVPH),
		runParams.TrafficDayMediumVPH.float(&options.TrafficDayMediumVPH),
		runParams.TrafficDayHeavyVPH.float(&options.TrafficDayHeavyVPH),
		runParams.TrafficDayPTWVPH.float(&options.TrafficDayPTWVPH),
		runParams.TrafficEveningLightVPH.float(&options.TrafficEveningLightVPH),
		runParams.TrafficEveningMediumVPH.float(&options.TrafficEveningMediumVPH),
		runParams.TrafficEveningHeavyVPH.float(&options.TrafficEveningHeavyVPH),
		runParams.TrafficEveningPTWVPH.float(&options.TrafficEveningPTWVPH),
		runParams.TrafficNightLightVPH.float(&options.TrafficNightLightVPH),
		runParams.TrafficNightMediumVPH.float(&options.TrafficNightMediumVPH),
		runParams.TrafficNightHeavyVPH.float(&options.TrafficNightHeavyVPH),
		runParams.TrafficNightPTWVPH.float(&options.TrafficNightPTWVPH),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.UrbanCanyonDB.float(&options.UrbanCanyonDB),
		runParams.IntersectionDensityPerKM.float(&options.IntersectionDensityPerKM),
		runParams.MinDistanceM.float(&options.MinDistanceM),
		runParams.ReferencePowerLevelDB.float(&options.ReferencePowerLevelDB),
		runParams.EngineStateFactor.float(&options.EngineStateFactor),
		runParams.BankAngleDeg.float(&options.BankAngleDeg),
		// lateral_offset_m was declared by the schema but bound nowhere until
		// the binding tables made the gap visible; the schema default is 0, so
		// binding it changes nothing for a default run and makes
		// `--param lateral_offset_m=...` take effect instead of being dropped.
		runParams.LateralOffsetM.float(&options.LateralOffsetM),
		runParams.TrackStartHeightM.float(&options.TrackStartHeightM),
		runParams.TrackEndHeightM.float(&options.TrackEndHeightM),
		runParams.MovementDayPerHour.float(&options.MovementDayPerHour),
		runParams.MovementEveningPerHour.float(&options.MovementEveningPerHour),
		runParams.MovementNightPerHour.float(&options.MovementNightPerHour),
		runParams.LateralDirectivityDB.float(&options.LateralDirectivityDB),
		runParams.ApproachCorrectionDB.float(&options.ApproachCorrectionDB),
		runParams.ClimbCorrectionDB.float(&options.ClimbCorrectionDB),
		runParams.MinSlantDistanceM.float(&options.MinSlantDistanceM),
		runParams.UpstreamMappingStandard.str(&options.UpstreamMappingStandard),
		runParams.BuildingUsageType.str(&options.BuildingUsageType),
		runParams.OccupancyMode.str(&options.OccupancyMode),
		runParams.FacadeEvaluationMode.str(&options.FacadeEvaluationMode),
		runParams.RoadSurfaceType.str(&options.SurfaceType),
		runParams.RoadFunctionClass.str(&options.RoadFunctionClass),
		runParams.RoadJunctionType.str(&options.JunctionType),
		runParams.AirportID.str(&options.AirportID),
		runParams.RunwayID.str(&options.RunwayID),
		runParams.AircraftOperationType.str(&options.OperationType),
		runParams.AircraftClass.str(&options.AircraftClass),
		runParams.AircraftProcedureType.str(&options.ProcedureType),
		runParams.AircraftThrustMode.str(&options.ThrustMode),
	}
}

func parseBEBExposureRunOptions(params map[string]string) (bebExposureRunOptions, error) {
	options := bebExposureRunOptions{}

	err := applyBoundParams("cli.parseBEBExposureRunOptions", params, bebExposureParamBindings(&options))
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
