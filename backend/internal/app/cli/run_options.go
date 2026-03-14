package cli

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const (
	dummyResultUnit       = "dB"
	defaultModelPath      = ".noise/model/model.normalized.geojson"
	maxDummyReceivers     = 250000
	receiverModeAutoGrid  = "auto-grid"
	receiverModeCustom    = "custom"
	explicitReceiverSetID = "explicit-manual"
)

type dummyRunOptions struct {
	GridResolutionM float64
	GridPaddingM    float64
	ReceiverHeightM float64
	SourceEmission  float64
	Workers         int
	ChunkSize       int
	DisableCache    bool
}

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

type rls19RoadRunOptions struct {
	GridResolutionM  float64
	GridPaddingM     float64
	ReceiverHeightM  float64
	SurfaceType      string
	SpeedPkwKPH      float64
	SpeedLkw1KPH     float64
	SpeedLkw2KPH     float64
	SpeedKradKPH     float64
	GradientPercent  float64
	TrafficDayPkw    float64
	TrafficDayLkw1   float64
	TrafficDayLkw2   float64
	TrafficDayKrad   float64
	TrafficNightPkw  float64
	TrafficNightLkw1 float64
	TrafficNightLkw2 float64
	TrafficNightKrad float64
	SegmentLengthM   float64
	MinDistanceM     float64
}

type schall03RunOptions struct {
	GridResolutionM       float64
	GridPaddingM          float64
	ReceiverHeightM       float64
	TrainClass            string
	TractionType          string
	TrackType             string
	TrackForm             string
	TrackRoughnessClass   string
	AverageTrainSpeedKPH  float64
	CurveRadiusM          float64
	OnBridge              bool
	TrafficDayTrainsPH    float64
	TrafficNightTrainsPH  float64
	AirAbsorptionDBPerKM  float64
	GroundAttenuationDB   float64
	SlabTrackCorrectionDB float64
	BridgeCorrectionDB    float64
	CurveCorrectionDB     float64
	MinDistanceM          float64
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

type iso9613RunOptions struct {
	GridResolutionM         float64
	GridPaddingM            float64
	ReceiverHeightM         float64
	SourceHeightM           float64
	SoundPowerLevelDB       float64
	DirectivityCorrectionDB float64
	TonalityCorrectionDB    float64
	ImpulsivityCorrectionDB float64
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	BarrierAttenuationDB    float64
	MinDistanceM            float64
}

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

type persistedRunOutputs struct {
	ReceiverJSONPath   string
	ReceiverCSVPath    string
	RasterMetadataPath string
	RasterDataPath     string
	SummaryPath        string
}

func parseKeyValueFlags(values []string) (map[string]string, error) {
	params := make(map[string]string, len(values))
	for _, item := range values {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (expected key=value)", item), nil)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (empty key)", item), nil)
		}

		params[key] = value
	}

	return params, nil
}

func buildRunProvenanceMetadata(standardID string, params map[string]string, receiverMode string) map[string]string {
	metadata := map[string]string{
		"receiver_mode": receiverMode,
	}

	switch standardID {
	case cnossosroad.StandardID:
		return mergeMetadata(metadata, cnossosroad.ProvenanceMetadata(params))
	case cnossosrail.StandardID:
		return mergeMetadata(metadata, cnossosrail.ProvenanceMetadata(params))
	case cnossosindustry.StandardID:
		return mergeMetadata(metadata, cnossosindustry.ProvenanceMetadata(params))
	case cnossosaircraft.StandardID:
		return mergeMetadata(metadata, cnossosaircraft.ProvenanceMetadata(params))
	case bubroad.StandardID:
		return mergeMetadata(metadata, bubroad.ProvenanceMetadata(params))
	case iso9613.StandardID:
		return mergeMetadata(metadata, iso9613.ProvenanceMetadata(params))
	case bufaircraft.StandardID:
		return mergeMetadata(metadata, bufaircraft.ProvenanceMetadata(params))
	case bebexposure.StandardID:
		return mergeMetadata(metadata, bebexposure.ProvenanceMetadata(params))
	case rls19road.StandardID:
		return mergeMetadata(metadata, rls19road.ProvenanceMetadata(params))
	case schall03.StandardID:
		return mergeMetadata(metadata, schall03.ProvenanceMetadata(params))
	default:
		return metadata
	}
}

func mergeMetadata(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(extra))
	maps.Copy(merged, base)

	maps.Copy(merged, extra)

	return merged
}

func validateReceiverMode(mode string) error {
	switch mode {
	case receiverModeAutoGrid, receiverModeCustom:
		return nil
	default:
		return domainerrors.New(domainerrors.KindUserInput, "cli.run", fmt.Sprintf("invalid receiver mode %q", mode), nil)
	}
}

func receiverSetID(mode string) string {
	if mode == receiverModeCustom {
		return explicitReceiverSetID
	}

	return ""
}

func parseDummyRunOptions(params map[string]string) (dummyRunOptions, error) {
	options := dummyRunOptions{}

	parseFloat := func(key string, target *float64, min float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		if math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < min {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("%s must be >= %g", key, min), nil)
		}

		*target = parsed

		return nil
	}

	parseInt := func(key string, target *int, min int) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		if parsed < min {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("%s must be >= %d", key, min), nil)
		}

		*target = parsed

		return nil
	}

	err := parseFloat("grid_resolution_m", &options.GridResolutionM, 0.001)
	if err != nil {
		return dummyRunOptions{}, err
	}

	err = parseFloat("grid_padding_m", &options.GridPaddingM, 0)
	if err != nil {
		return dummyRunOptions{}, err
	}

	err = parseFloat("receiver_height_m", &options.ReceiverHeightM, 0)
	if err != nil {
		return dummyRunOptions{}, err
	}

	err = parseFloat("source_emission_db", &options.SourceEmission, 0)
	if err != nil {
		return dummyRunOptions{}, err
	}

	err = parseInt("workers", &options.Workers, 0)
	if err != nil {
		return dummyRunOptions{}, err
	}

	err = parseInt("chunk_size", &options.ChunkSize, 1)
	if err != nil {
		return dummyRunOptions{}, err
	}

	rawDisable, ok := params["disable_cache"]
	if !ok {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", `normalized parameter "disable_cache" missing`, nil)
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(rawDisable))
	if err != nil {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid disable_cache=%q", rawDisable), err)
	}

	options.DisableCache = parsed

	return options, nil
}

func parseCnossosRoadRunOptions(params map[string]string) (cnossosRoadRunOptions, error) {
	options := cnossosRoadRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	err := parseFloat("grid_resolution_m", &options.GridResolutionM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("grid_padding_m", &options.GridPaddingM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("receiver_height_m", &options.ReceiverHeightM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	surfaceType, err := getString("road_surface_type")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	roadCategory, err := getString("road_category")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	options.RoadCategory = roadCategory
	options.SurfaceType = surfaceType

	err = parseFloat("road_speed_kph", &options.SpeedKPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("road_gradient_percent", &options.GradientPercent)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	junctionType, err := getString("road_junction_type")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	options.JunctionType = junctionType

	err = parseFloat("road_junction_distance_m", &options.JunctionDistanceM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("road_temperature_c", &options.TemperatureC)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("road_studded_tyre_share", &options.StuddedTyreShare)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_day_light_vph", &options.TrafficDayLightVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_day_medium_vph", &options.TrafficDayMediumVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_day_heavy_vph", &options.TrafficDayHeavyVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_evening_light_vph", &options.TrafficEveningLightVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_evening_medium_vph", &options.TrafficEveningMediumVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_evening_heavy_vph", &options.TrafficEveningHeavyVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_night_light_vph", &options.TrafficNightLightVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_night_medium_vph", &options.TrafficNightMediumVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_night_heavy_vph", &options.TrafficNightHeavyVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_day_ptw_vph", &options.TrafficDayPTWVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_evening_ptw_vph", &options.TrafficEveningPTWVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("traffic_night_ptw_vph", &options.TrafficNightPTWVPH)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("air_absorption_db_per_km", &options.AirAbsorptionDBPerKM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("ground_attenuation_db", &options.GroundAttenuationDB)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("barrier_attenuation_db", &options.BarrierAttenuationDB)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	err = parseFloat("min_distance_m", &options.MinDistanceM)
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	return options, nil
}

func parseCnossosRailRunOptions(params map[string]string) (cnossosRailRunOptions, error) {
	options := cnossosRailRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRailRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosRailRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRailRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"rail_average_train_speed_kph", &options.AverageTrainSpeedKPH},
		{"rail_braking_share", &options.BrakingShare},
		{"rail_curve_radius_m", &options.CurveRadiusM},
		{"traffic_day_trains_per_hour", &options.TrafficDayTrainsPerHour},
		{"traffic_evening_trains_per_hour", &options.TrafficEveningTrainsPerHour},
		{"traffic_night_trains_per_hour", &options.TrafficNightTrainsPerHour},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"bridge_correction_db", &options.BridgeCorrectionDB},
		{"curve_squeal_db", &options.CurveSquealDB},
		{"min_distance_m", &options.MinDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return cnossosRailRunOptions{}, err
		}
	}

	var err error

	options.TractionType, err = getString("rail_traction_type")
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	options.TrackType, err = getString("rail_track_type")
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	options.TrackRoughnessClass, err = getString("rail_track_roughness_class")
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	rawOnBridge, ok := params["rail_on_bridge"]
	if !ok {
		return cnossosRailRunOptions{}, domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRailRunOptions", `normalized parameter "rail_on_bridge" missing`, nil)
	}

	options.OnBridge, err = strconv.ParseBool(strings.TrimSpace(rawOnBridge))
	if err != nil {
		return cnossosRailRunOptions{}, domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosRailRunOptions", fmt.Sprintf("invalid rail_on_bridge=%q", rawOnBridge), err)
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

func parseSchall03RunOptions(params map[string]string) (schall03RunOptions, error) {
	options := schall03RunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseSchall03RunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseSchall03RunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseSchall03RunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"rail_average_train_speed_kph", &options.AverageTrainSpeedKPH},
		{"rail_curve_radius_m", &options.CurveRadiusM},
		{"traffic_day_trains_per_hour", &options.TrafficDayTrainsPH},
		{"traffic_night_trains_per_hour", &options.TrafficNightTrainsPH},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"slab_track_correction_db", &options.SlabTrackCorrectionDB},
		{"bridge_correction_db", &options.BridgeCorrectionDB},
		{"curve_correction_db", &options.CurveCorrectionDB},
		{"min_distance_m", &options.MinDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return schall03RunOptions{}, err
		}
	}

	var err error

	options.TractionType, err = getString("rail_traction_type")
	if err != nil {
		return schall03RunOptions{}, err
	}

	options.TrainClass, err = getString("rail_train_class")
	if err != nil {
		return schall03RunOptions{}, err
	}

	options.TrackType, err = getString("rail_track_type")
	if err != nil {
		return schall03RunOptions{}, err
	}

	options.TrackForm, err = getString("rail_track_form")
	if err != nil {
		return schall03RunOptions{}, err
	}

	options.TrackRoughnessClass, err = getString("rail_track_roughness_class")
	if err != nil {
		return schall03RunOptions{}, err
	}

	rawOnBridge, ok := params["rail_on_bridge"]
	if !ok {
		return schall03RunOptions{}, domainerrors.New(domainerrors.KindInternal, "cli.parseSchall03RunOptions", `normalized parameter "rail_on_bridge" missing`, nil)
	}

	options.OnBridge, err = strconv.ParseBool(strings.TrimSpace(rawOnBridge))
	if err != nil {
		return schall03RunOptions{}, domainerrors.New(domainerrors.KindUserInput, "cli.parseSchall03RunOptions", fmt.Sprintf("invalid rail_on_bridge=%q", rawOnBridge), err)
	}

	return options, nil
}

func (o schall03RunOptions) PropagationConfig() schall03.PropagationConfig {
	return schall03.PropagationConfig{
		AirAbsorptionDBPerKM:  o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:   o.GroundAttenuationDB,
		SlabTrackCorrectionDB: o.SlabTrackCorrectionDB,
		BridgeCorrectionDB:    o.BridgeCorrectionDB,
		CurveCorrectionDB:     o.CurveCorrectionDB,
		MinDistanceM:          o.MinDistanceM,
	}
}

func parseBUBRoadRunOptions(params map[string]string) (bubRoadRunOptions, error) {
	options := bubRoadRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseBUBRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseBUBRoadRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseBUBRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	err := parseFloat("grid_resolution_m", &options.GridResolutionM)
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	err = parseFloat("grid_padding_m", &options.GridPaddingM)
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	err = parseFloat("receiver_height_m", &options.ReceiverHeightM)
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	options.SurfaceType, err = getString("road_surface_type")
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	options.RoadFunctionClass, err = getString("road_function_class")
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	options.JunctionType, err = getString("road_junction_type")
	if err != nil {
		return bubRoadRunOptions{}, err
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
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
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return bubRoadRunOptions{}, err
		}
	}

	return options, nil
}

func parseRLS19RoadRunOptions(params map[string]string) (rls19RoadRunOptions, error) {
	options := rls19RoadRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseRLS19RoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseRLS19RoadRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseRLS19RoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	var err error

	options.SurfaceType, err = getString("surface_type")
	if err != nil {
		return rls19RoadRunOptions{}, err
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"speed_pkw_kph", &options.SpeedPkwKPH},
		{"speed_lkw1_kph", &options.SpeedLkw1KPH},
		{"speed_lkw2_kph", &options.SpeedLkw2KPH},
		{"speed_krad_kph", &options.SpeedKradKPH},
		{"gradient_percent", &options.GradientPercent},
		{"traffic_day_pkw", &options.TrafficDayPkw},
		{"traffic_day_lkw1", &options.TrafficDayLkw1},
		{"traffic_day_lkw2", &options.TrafficDayLkw2},
		{"traffic_day_krad", &options.TrafficDayKrad},
		{"traffic_night_pkw", &options.TrafficNightPkw},
		{"traffic_night_lkw1", &options.TrafficNightLkw1},
		{"traffic_night_lkw2", &options.TrafficNightLkw2},
		{"traffic_night_krad", &options.TrafficNightKrad},
		{"segment_length_m", &options.SegmentLengthM},
		{"min_distance_m", &options.MinDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return rls19RoadRunOptions{}, err
		}
	}

	return options, nil
}

func parseCnossosAircraftRunOptions(params map[string]string) (cnossosAircraftRunOptions, error) {
	options := cnossosAircraftRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosAircraftRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosAircraftRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosAircraftRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"reference_power_level_db", &options.ReferencePowerLevelDB},
		{"engine_state_factor", &options.EngineStateFactor},
		{"bank_angle_deg", &options.BankAngleDeg},
		{"lateral_offset_m", &options.LateralOffsetM},
		{"track_start_height_m", &options.TrackStartHeightM},
		{"track_end_height_m", &options.TrackEndHeightM},
		{"movement_day_per_hour", &options.MovementDayPerHour},
		{"movement_evening_per_hour", &options.MovementEveningPerHour},
		{"movement_night_per_hour", &options.MovementNightPerHour},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"lateral_directivity_db", &options.LateralDirectivityDB},
		{"approach_correction_db", &options.ApproachCorrectionDB},
		{"climb_correction_db", &options.ClimbCorrectionDB},
		{"min_slant_distance_m", &options.MinSlantDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return cnossosAircraftRunOptions{}, err
		}
	}

	var err error

	options.AirportID, err = getString("airport_id")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	options.RunwayID, err = getString("runway_id")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	options.OperationType, err = getString("aircraft_operation_type")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	options.AircraftClass, err = getString("aircraft_class")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	options.ProcedureType, err = getString("aircraft_procedure_type")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	options.ThrustMode, err = getString("aircraft_thrust_mode")
	if err != nil {
		return cnossosAircraftRunOptions{}, err
	}

	return options, nil
}

func parseBUFAircraftRunOptions(params map[string]string) (bufAircraftRunOptions, error) {
	options := bufAircraftRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseBUFAircraftRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseBUFAircraftRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseBUFAircraftRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"reference_power_level_db", &options.ReferencePowerLevelDB},
		{"engine_state_factor", &options.EngineStateFactor},
		{"bank_angle_deg", &options.BankAngleDeg},
		{"lateral_offset_m", &options.LateralOffsetM},
		{"track_start_height_m", &options.TrackStartHeightM},
		{"track_end_height_m", &options.TrackEndHeightM},
		{"movement_day_per_hour", &options.MovementDayPerHour},
		{"movement_evening_per_hour", &options.MovementEveningPerHour},
		{"movement_night_per_hour", &options.MovementNightPerHour},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"lateral_directivity_db", &options.LateralDirectivityDB},
		{"approach_correction_db", &options.ApproachCorrectionDB},
		{"climb_correction_db", &options.ClimbCorrectionDB},
		{"min_slant_distance_m", &options.MinSlantDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return bufAircraftRunOptions{}, err
		}
	}

	var err error

	options.AirportID, err = getString("airport_id")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	options.RunwayID, err = getString("runway_id")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	options.OperationType, err = getString("aircraft_operation_type")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	options.AircraftClass, err = getString("aircraft_class")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	options.ProcedureType, err = getString("aircraft_procedure_type")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	options.ThrustMode, err = getString("aircraft_thrust_mode")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	return options, nil
}

func (o cnossosRoadRunOptions) PropagationConfig() cnossosroad.PropagationConfig {
	return cnossosroad.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		BarrierAttenuationDB: o.BarrierAttenuationDB,
		MinDistanceM:         o.MinDistanceM,
	}
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

func (o rls19RoadRunOptions) PropagationConfig() rls19road.PropagationConfig {
	return rls19road.PropagationConfig{
		SegmentLengthM:  o.SegmentLengthM,
		MinDistanceM:    o.MinDistanceM,
		ReceiverHeightM: o.ReceiverHeightM,
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

func parseCnossosIndustryRunOptions(params map[string]string) (cnossosIndustryRunOptions, error) {
	options := cnossosIndustryRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosIndustryRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosIndustryRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosIndustryRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	var err error

	options.SourceCategory, err = getString("industry_source_category")
	if err != nil {
		return cnossosIndustryRunOptions{}, err
	}

	options.EnclosureState, err = getString("industry_enclosure_state")
	if err != nil {
		return cnossosIndustryRunOptions{}, err
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"industry_sound_power_level_db", &options.SoundPowerLevelDB},
		{"industry_source_height_m", &options.SourceHeightM},
		{"industry_tonality_correction_db", &options.TonalityCorrectionDB},
		{"industry_impulsivity_correction_db", &options.ImpulsivityCorrectionDB},
		{"operation_day_factor", &options.OperationDayFactor},
		{"operation_evening_factor", &options.OperationEveningFactor},
		{"operation_night_factor", &options.OperationNightFactor},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"screening_attenuation_db", &options.ScreeningAttenuationDB},
		{"facade_reflection_db", &options.FacadeReflectionDB},
		{"min_distance_m", &options.MinDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return cnossosIndustryRunOptions{}, err
		}
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

func parseISO9613RunOptions(params map[string]string) (iso9613RunOptions, error) {
	options := iso9613RunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseISO9613RunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseISO9613RunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseISO9613RunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	var err error

	options.MeteorologyAssumption, err = getString("meteorology_assumption")
	if err != nil {
		return iso9613RunOptions{}, err
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
		{"iso9613_source_height_m", &options.SourceHeightM},
		{"iso9613_sound_power_level_db", &options.SoundPowerLevelDB},
		{"iso9613_directivity_correction_db", &options.DirectivityCorrectionDB},
		{"iso9613_tonality_correction_db", &options.TonalityCorrectionDB},
		{"iso9613_impulsivity_correction_db", &options.ImpulsivityCorrectionDB},
		{"ground_factor", &options.GroundFactor},
		{"air_temperature_c", &options.AirTemperatureC},
		{"relative_humidity_percent", &options.RelativeHumidityPercent},
		{"barrier_attenuation_db", &options.BarrierAttenuationDB},
		{"min_distance_m", &options.MinDistanceM},
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return iso9613RunOptions{}, err
		}
	}

	return options, nil
}

func (o iso9613RunOptions) PropagationConfig() iso9613.PropagationConfig {
	return iso9613.PropagationConfig{
		GroundFactor:            o.GroundFactor,
		AirTemperatureC:         o.AirTemperatureC,
		RelativeHumidityPercent: o.RelativeHumidityPercent,
		MeteorologyAssumption:   o.MeteorologyAssumption,
		BarrierAttenuationDB:    o.BarrierAttenuationDB,
		MinDistanceM:            o.MinDistanceM,
	}
}

func parseBEBExposureRunOptions(params map[string]string) (bebExposureRunOptions, error) {
	options := bebExposureRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseBEBExposureRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseBEBExposureRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseBEBExposureRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		return strings.TrimSpace(value), nil
	}

	for _, item := range []struct {
		key    string
		target *float64
	}{
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
	} {
		err := parseFloat(item.key, item.target)
		if err != nil {
			return bebExposureRunOptions{}, err
		}
	}

	var err error

	options.UpstreamMappingStandard, err = getString("upstream_mapping_standard")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.BuildingUsageType, err = getString("building_usage_type")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.OccupancyMode, err = getString("occupancy_mode")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.FacadeEvaluationMode, err = getString("facade_evaluation_mode")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.SurfaceType, err = getString("road_surface_type")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.RoadFunctionClass, err = getString("road_function_class")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.JunctionType, err = getString("road_junction_type")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.AirportID, err = getString("airport_id")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.RunwayID, err = getString("runway_id")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.OperationType, err = getString("aircraft_operation_type")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.AircraftClass, err = getString("aircraft_class")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.ProcedureType, err = getString("aircraft_procedure_type")
	if err != nil {
		return bebExposureRunOptions{}, err
	}

	options.ThrustMode, err = getString("aircraft_thrust_mode")
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
