package cli

import (
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
)

type cnossosRoadRunOptions struct {
	GridResolutionM         float64
	GridPaddingM            float64
	ReceiverHeightM         float64
	RoadCategory            string
	SurfaceType             string
	SpeedKPH                float64
	GradientPercent         float64
	JunctionType            string
	JunctionDistanceM       float64
	TemperatureC            float64
	StuddedTyreShare        float64
	TrafficDayLightVPH      float64
	TrafficDayMediumVPH     float64
	TrafficDayHeavyVPH      float64
	TrafficEveningLightVPH  float64
	TrafficEveningMediumVPH float64
	TrafficEveningHeavyVPH  float64
	TrafficNightLightVPH    float64
	TrafficNightMediumVPH   float64
	TrafficNightHeavyVPH    float64
	TrafficDayPTWVPH        float64
	TrafficEveningPTWVPH    float64
	TrafficNightPTWVPH      float64
	AirAbsorptionDBPerKM    float64
	GroundAttenuationDB     float64
	BarrierAttenuationDB    float64
	MinDistanceM            float64
}

type cnossosRailRunOptions struct {
	GridResolutionM             float64
	GridPaddingM                float64
	ReceiverHeightM             float64
	TractionType                string
	TrackType                   string
	TrackRoughnessClass         string
	AverageTrainSpeedKPH        float64
	BrakingShare                float64
	CurveRadiusM                float64
	OnBridge                    bool
	TrafficDayTrainsPerHour     float64
	TrafficEveningTrainsPerHour float64
	TrafficNightTrainsPerHour   float64
	AirAbsorptionDBPerKM        float64
	GroundAttenuationDB         float64
	BridgeCorrectionDB          float64
	CurveSquealDB               float64
	MinDistanceM                float64
}

type aircraftRunOptions struct {
	GridResolutionM        float64
	GridPaddingM           float64
	ReceiverHeightM        float64
	AirportID              string
	RunwayID               string
	OperationType          string
	AircraftClass          string
	ProcedureType          string
	ThrustMode             string
	ReferencePowerLevelDB  float64
	EngineStateFactor      float64
	BankAngleDeg           float64
	LateralOffsetM         float64
	TrackStartHeightM      float64
	TrackEndHeightM        float64
	MovementDayPerHour     float64
	MovementEveningPerHour float64
	MovementNightPerHour   float64
	AirAbsorptionDBPerKM   float64
	GroundAttenuationDB    float64
	LateralDirectivityDB   float64
	ApproachCorrectionDB   float64
	ClimbCorrectionDB      float64
	MinSlantDistanceM      float64
}

type cnossosAircraftRunOptions struct {
	GridResolutionM        float64
	GridPaddingM           float64
	ReceiverHeightM        float64
	AirportID              string
	RunwayID               string
	OperationType          string
	AircraftClass          string
	ProcedureType          string
	ThrustMode             string
	ReferencePowerLevelDB  float64
	EngineStateFactor      float64
	BankAngleDeg           float64
	LateralOffsetM         float64
	TrackStartHeightM      float64
	TrackEndHeightM        float64
	MovementDayPerHour     float64
	MovementEveningPerHour float64
	MovementNightPerHour   float64
	AirAbsorptionDBPerKM   float64
	GroundAttenuationDB    float64
	LateralDirectivityDB   float64
	ApproachCorrectionDB   float64
	ClimbCorrectionDB      float64
	MinSlantDistanceM      float64
}

type cnossosIndustryRunOptions struct {
	GridResolutionM         float64
	GridPaddingM            float64
	ReceiverHeightM         float64
	SourceCategory          string
	EnclosureState          string
	SoundPowerLevelDB       float64
	SourceHeightM           float64
	TonalityCorrectionDB    float64
	ImpulsivityCorrectionDB float64
	OperationDayFactor      float64
	OperationEveningFactor  float64
	OperationNightFactor    float64
	AirAbsorptionDBPerKM    float64
	GroundAttenuationDB     float64
	ScreeningAttenuationDB  float64
	FacadeReflectionDB      float64
	MinDistanceM            float64
}

// cnossosRoadParamBindings binds the cnossos-road parameter schema.
func cnossosRoadParamBindings(options *cnossosRoadRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.RoadSurfaceType.str(&options.SurfaceType),
		runParams.RoadCategory.str(&options.RoadCategory),
		runParams.RoadSpeedKPH.float(&options.SpeedKPH),
		runParams.RoadGradientPercent.float(&options.GradientPercent),
		runParams.RoadJunctionType.str(&options.JunctionType),
		runParams.RoadJunctionDistanceM.float(&options.JunctionDistanceM),
		runParams.RoadTemperatureC.float(&options.TemperatureC),
		runParams.RoadStuddedTyreShare.float(&options.StuddedTyreShare),
		runParams.TrafficDayLightVPH.float(&options.TrafficDayLightVPH),
		runParams.TrafficDayMediumVPH.float(&options.TrafficDayMediumVPH),
		runParams.TrafficDayHeavyVPH.float(&options.TrafficDayHeavyVPH),
		runParams.TrafficEveningLightVPH.float(&options.TrafficEveningLightVPH),
		runParams.TrafficEveningMediumVPH.float(&options.TrafficEveningMediumVPH),
		runParams.TrafficEveningHeavyVPH.float(&options.TrafficEveningHeavyVPH),
		runParams.TrafficNightLightVPH.float(&options.TrafficNightLightVPH),
		runParams.TrafficNightMediumVPH.float(&options.TrafficNightMediumVPH),
		runParams.TrafficNightHeavyVPH.float(&options.TrafficNightHeavyVPH),
		runParams.TrafficDayPTWVPH.float(&options.TrafficDayPTWVPH),
		runParams.TrafficEveningPTWVPH.float(&options.TrafficEveningPTWVPH),
		runParams.TrafficNightPTWVPH.float(&options.TrafficNightPTWVPH),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.BarrierAttenuationDB.float(&options.BarrierAttenuationDB),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

func parseCnossosRoadRunOptions(params map[string]string) (cnossosRoadRunOptions, error) {
	options := cnossosRoadRunOptions{}

	err := applyBoundParams("cli.parseCnossosRoadRunOptions", params, cnossosRoadParamBindings(&options))
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	return options, nil
}

// sharedRailParamBindings binds every rail parameter that cnossos-rail and
// bub-rail declare alike. The two parameter schemas differ only in
// rail_track_type, which each caller adds for itself.
func sharedRailParamBindings(options *cnossosRailRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.RailAverageTrainSpeedKPH.float(&options.AverageTrainSpeedKPH),
		runParams.RailBrakingShare.float(&options.BrakingShare),
		runParams.RailCurveRadiusM.float(&options.CurveRadiusM),
		runParams.TrafficDayTrainsPerHour.float(&options.TrafficDayTrainsPerHour),
		runParams.TrafficEveningTrainsPerHour.float(&options.TrafficEveningTrainsPerHour),
		runParams.TrafficNightTrainsPerHour.float(&options.TrafficNightTrainsPerHour),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.BridgeCorrectionDB.float(&options.BridgeCorrectionDB),
		runParams.CurveSquealDB.float(&options.CurveSquealDB),
		runParams.MinDistanceM.float(&options.MinDistanceM),
		runParams.RailTractionType.str(&options.TractionType),
		runParams.RailTrackRoughnessClass.str(&options.TrackRoughnessClass),
		runParams.RailOnBridge.boolean(&options.OnBridge),
	}
}

// cnossosRailParamBindings binds the cnossos-rail parameter schema.
func cnossosRailParamBindings(options *cnossosRailRunOptions) []boundParam {
	return append(sharedRailParamBindings(options), runParams.RailTrackType.str(&options.TrackType))
}

func parseCnossosRailRunOptions(params map[string]string) (cnossosRailRunOptions, error) {
	options := cnossosRailRunOptions{}

	err := applyBoundParams("cli.parseCnossosRailRunOptions", params, cnossosRailParamBindings(&options))
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	return options, nil
}

func (o cnossosRailRunOptions) PropagationConfig() cnossosrail.PropagationConfig {
	return cnossosrail.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		BridgeCorrectionDB:   o.BridgeCorrectionDB,
		CurveSquealDB:        o.CurveSquealDB,
		MinDistanceM:         o.MinDistanceM,
	}
}

// aircraftParamBindings binds the aircraft parameter schema that cnossos-aircraft
// and buf-aircraft share.
func aircraftParamBindings(options *aircraftRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.ReferencePowerLevelDB.float(&options.ReferencePowerLevelDB),
		runParams.EngineStateFactor.float(&options.EngineStateFactor),
		runParams.BankAngleDeg.float(&options.BankAngleDeg),
		runParams.LateralOffsetM.float(&options.LateralOffsetM),
		runParams.TrackStartHeightM.float(&options.TrackStartHeightM),
		runParams.TrackEndHeightM.float(&options.TrackEndHeightM),
		runParams.MovementDayPerHour.float(&options.MovementDayPerHour),
		runParams.MovementEveningPerHour.float(&options.MovementEveningPerHour),
		runParams.MovementNightPerHour.float(&options.MovementNightPerHour),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.LateralDirectivityDB.float(&options.LateralDirectivityDB),
		runParams.ApproachCorrectionDB.float(&options.ApproachCorrectionDB),
		runParams.ClimbCorrectionDB.float(&options.ClimbCorrectionDB),
		runParams.MinSlantDistanceM.float(&options.MinSlantDistanceM),
		runParams.AirportID.str(&options.AirportID),
		runParams.RunwayID.str(&options.RunwayID),
		runParams.AircraftOperationType.str(&options.OperationType),
		runParams.AircraftClass.str(&options.AircraftClass),
		runParams.AircraftProcedureType.str(&options.ProcedureType),
		runParams.AircraftThrustMode.str(&options.ThrustMode),
	}
}

func parseAircraftRunOptions(params map[string]string, contextName string) (aircraftRunOptions, error) {
	options := aircraftRunOptions{}

	err := applyBoundParams(contextName, params, aircraftParamBindings(&options))
	if err != nil {
		return aircraftRunOptions{}, err
	}

	return options, nil
}

func parseCnossosAircraftRunOptions(params map[string]string) (cnossosAircraftRunOptions, error) {
	options, err := parseAircraftRunOptions(params, "cli.parseCnossosAircraftRunOptions")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	return toCnossosAircraftRunOptions(options), nil
}

func toCnossosAircraftRunOptions(options aircraftRunOptions) cnossosAircraftRunOptions {
	return cnossosAircraftRunOptions(options)
}

func (o cnossosRoadRunOptions) PropagationConfig() cnossosroad.PropagationConfig {
	return cnossosroad.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		BarrierAttenuationDB: o.BarrierAttenuationDB,
		MinDistanceM:         o.MinDistanceM,
	}
}

func (o cnossosAircraftRunOptions) PropagationConfig() cnossosaircraft.PropagationConfig {
	return cnossosaircraft.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		LateralDirectivityDB: o.LateralDirectivityDB,
		ApproachCorrectionDB: o.ApproachCorrectionDB,
		ClimbCorrectionDB:    o.ClimbCorrectionDB,
		MinSlantDistanceM:    o.MinSlantDistanceM,
	}
}

// sharedIndustryParamBindings binds every industry parameter that
// cnossos-industry and bub-industry declare alike. The two parameter schemas
// differ only in industry_source_category and industry_enclosure_state, which
// cnossos-industry adds for itself.
func sharedIndustryParamBindings(options *cnossosIndustryRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.IndustrySoundPowerLevelDB.float(&options.SoundPowerLevelDB),
		runParams.IndustrySourceHeightM.float(&options.SourceHeightM),
		runParams.IndustryTonalityCorrectionDB.float(&options.TonalityCorrectionDB),
		runParams.IndustryImpulsivityCorrectionDB.float(&options.ImpulsivityCorrectionDB),
		runParams.OperationDayFactor.float(&options.OperationDayFactor),
		runParams.OperationEveningFactor.float(&options.OperationEveningFactor),
		runParams.OperationNightFactor.float(&options.OperationNightFactor),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.ScreeningAttenuationDB.float(&options.ScreeningAttenuationDB),
		runParams.FacadeReflectionDB.float(&options.FacadeReflectionDB),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

// cnossosIndustryParamBindings binds the cnossos-industry parameter schema.
func cnossosIndustryParamBindings(options *cnossosIndustryRunOptions) []boundParam {
	return append([]boundParam{
		runParams.IndustrySourceCategory.str(&options.SourceCategory),
		runParams.IndustryEnclosureState.str(&options.EnclosureState),
	}, sharedIndustryParamBindings(options)...)
}

func parseCnossosIndustryRunOptions(params map[string]string) (cnossosIndustryRunOptions, error) {
	options := cnossosIndustryRunOptions{}

	err := applyBoundParams("cli.parseCnossosIndustryRunOptions", params, cnossosIndustryParamBindings(&options))
	if err != nil {
		return cnossosIndustryRunOptions{}, err
	}

	return options, nil
}

func (o cnossosIndustryRunOptions) PropagationConfig() cnossosindustry.PropagationConfig {
	return cnossosindustry.PropagationConfig{
		AirAbsorptionDBPerKM:   o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:    o.GroundAttenuationDB,
		ScreeningAttenuationDB: o.ScreeningAttenuationDB,
		FacadeReflectionDB:     o.FacadeReflectionDB,
		MinDistanceM:           o.MinDistanceM,
	}
}
