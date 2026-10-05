package cli

import (
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
)

type bubRoadRunOptions struct {
	GridResolutionM          float64
	GridPaddingM             float64
	ReceiverHeightM          float64
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
}

type bufAircraftRunOptions struct {
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

// bub-rail and bub-industry are alias modules over the cnossos-rail and
// cnossos-industry scaffolds: their source, propagation and output types are Go
// type aliases of the CNOSSOS ones, so a run of either carries exactly the same
// options. Only the published parameter schema differs, which is why each still
// gets its own parser below.
type (
	bubRailRunOptions     = cnossosRailRunOptions
	bubIndustryRunOptions = cnossosIndustryRunOptions
)

// bubRailParamBindings binds the bub-rail parameter schema, which publishes no
// rail_track_type although the aliased rail source model still requires one.
func bubRailParamBindings(options *bubRailRunOptions) []boundParam {
	return sharedRailParamBindings(options)
}

// parseBUBRailRunOptions parses the bub-rail schema. The run starts from
// ballasted track because the schema publishes no rail_track_type, which a
// feature's own rail_track_type property still overrides.
func parseBUBRailRunOptions(params map[string]string) (bubRailRunOptions, error) {
	options := bubRailRunOptions{TrackType: bubrail.TrackTypeBallasted}

	err := applyBoundParams("cli.parseBUBRailRunOptions", params, bubRailParamBindings(&options))
	if err != nil {
		return bubRailRunOptions{}, err
	}

	return options, nil
}

// bubRoadParamBindings binds the bub-road parameter schema.
func bubRoadParamBindings(options *bubRoadRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.RoadSurfaceType.str(&options.SurfaceType),
		runParams.RoadFunctionClass.str(&options.RoadFunctionClass),
		runParams.RoadJunctionType.str(&options.JunctionType),
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
	}
}

func parseBUBRoadRunOptions(params map[string]string) (bubRoadRunOptions, error) {
	options := bubRoadRunOptions{}

	err := applyBoundParams("cli.parseBUBRoadRunOptions", params, bubRoadParamBindings(&options))
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	return options, nil
}

func parseBUFAircraftRunOptions(params map[string]string) (bufAircraftRunOptions, error) {
	options, err := parseAircraftRunOptions(params, "cli.parseBUFAircraftRunOptions")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	return toBUFAircraftRunOptions(options), nil
}

func toBUFAircraftRunOptions(options aircraftRunOptions) bufAircraftRunOptions {
	return bufAircraftRunOptions(options)
}

func (o bubRoadRunOptions) PropagationConfig() bubroad.PropagationConfig {
	return bubroad.PropagationConfig{
		AirAbsorptionDBPerKM:     o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:      o.GroundAttenuationDB,
		UrbanCanyonDB:            o.UrbanCanyonDB,
		IntersectionDensityPerKM: o.IntersectionDensityPerKM,
		MinDistanceM:             o.MinDistanceM,
	}
}

func (o bufAircraftRunOptions) PropagationConfig() bufaircraft.PropagationConfig {
	return bufaircraft.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		LateralDirectivityDB: o.LateralDirectivityDB,
		ApproachCorrectionDB: o.ApproachCorrectionDB,
		ClimbCorrectionDB:    o.ClimbCorrectionDB,
		MinSlantDistanceM:    o.MinSlantDistanceM,
	}
}

// bubIndustryParamBindings binds the bub-industry parameter schema, which
// publishes neither industry_source_category nor industry_enclosure_state.
func bubIndustryParamBindings(options *bubIndustryRunOptions) []boundParam {
	return sharedIndustryParamBindings(options)
}

// parseBUBIndustryRunOptions parses the bub-industry schema, which publishes
// neither industry_source_category nor industry_enclosure_state although the
// aliased industry source model still requires both. The run therefore starts
// from an open process source, which a feature's own industry_source_category
// and industry_enclosure_state properties still override.
func parseBUBIndustryRunOptions(params map[string]string) (bubIndustryRunOptions, error) {
	options := bubIndustryRunOptions{
		SourceCategory: bubindustry.CategoryProcess,
		EnclosureState: bubindustry.EnclosureOpen,
	}

	err := applyBoundParams("cli.parseBUBIndustryRunOptions", params, bubIndustryParamBindings(&options))
	if err != nil {
		return bubIndustryRunOptions{}, err
	}

	return options, nil
}
