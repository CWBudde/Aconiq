package cli

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/framework"
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

	// evidenceTierKey names the tier field in provenance metadata, run
	// summaries and JSON command output alike, so the three never drift apart.
	evidenceTierKey = "evidence_tier"
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
	Engine                string
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

// bub-rail and bub-industry are alias modules over the cnossos-rail and
// cnossos-industry scaffolds: their source, propagation and output types are Go
// type aliases of the CNOSSOS ones, so a run of either carries exactly the same
// options. Only the published parameter schema differs, which is why each still
// gets its own parser below.
type (
	bubRailRunOptions     = cnossosRailRunOptions
	bubIndustryRunOptions = cnossosIndustryRunOptions
)

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
	C0Met                   float64
	MinDistanceM            float64
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

// buildRunProvenanceMetadata assembles the provenance metadata for one run.
// The evidence tier is stamped centrally rather than by every module, so that
// the machine-readable tier and the free-text compliance_boundary a module may
// contribute stay independent of one another.
func buildRunProvenanceMetadata(resolved framework.ResolvedProfile, params map[string]string, receiverMode string) map[string]string {
	metadata := map[string]string{
		"receiver_mode": receiverMode,
		evidenceTierKey: string(resolved.EvidenceTier),
	}

	switch resolved.StandardID {
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

// buildRunStandardData assembles the standard-data digest for one run.
//
// The digest answers a question the parameter and metadata maps cannot: which
// coefficient tables produced these numbers. It is recorded as its own
// provenance field rather than as an entry in input_hashes, which is defined as
// input-file path to SHA-256 and is rendered as an "Input files" table in
// reports; an embedded coefficient table is not a file the user supplied.
//
// A module that carries no coefficient data at all — dummy-freefield computes
// from its parameters alone — yields the zero value, and the field is omitted
// from the manifest rather than written empty.
func buildRunStandardData(resolved framework.ResolvedProfile) (project.StandardDataRef, error) {
	data, ok := standardDataForID(resolved.StandardID)
	if !ok {
		return project.StandardDataRef{}, nil
	}

	digest, err := data.Digest(resolved.StandardID, resolved.EvidenceTier)
	if err != nil {
		return project.StandardDataRef{}, domainerrors.New(domainerrors.KindInternal, "cli.buildRunStandardData", "compute standard data digest", err)
	}

	if digest.IsZero() {
		return project.StandardDataRef{}, nil
	}

	tables := make([]project.StandardDataTableRef, 0, len(digest.Tables))
	for _, table := range digest.Tables {
		tables = append(tables, project.StandardDataTableRef{Name: table.Name, Digest: table.Digest})
	}

	return project.StandardDataRef{
		Algorithm:    digest.Algorithm,
		Digest:       digest.Digest,
		EvidenceTier: string(digest.EvidenceTier),
		Tables:       tables,
	}, nil
}

// standardDataForID returns the coefficient data one module carries. The second
// result is false for a module that carries none.
//
// bub-rail and bub-industry are aliases over the cnossos rail and industry
// packages and share their coefficients, so they share their tables too.
func standardDataForID(standardID string) (framework.StandardData, bool) {
	switch standardID {
	case cnossosroad.StandardID:
		return cnossosroad.StandardData(), true
	case cnossosrail.StandardID, bubrail.StandardID:
		return cnossosrail.StandardData(), true
	case cnossosindustry.StandardID, bubindustry.StandardID:
		return cnossosindustry.StandardData(), true
	case cnossosaircraft.StandardID:
		return cnossosaircraft.StandardData(), true
	case bubroad.StandardID:
		return bubroad.StandardData(), true
	case bufaircraft.StandardID:
		return bufaircraft.StandardData(), true
	case bebexposure.StandardID:
		return bebexposure.StandardData(), true
	case iso9613.StandardID:
		return iso9613.StandardData(), true
	case rls19road.StandardID:
		return rls19road.StandardData(), true
	case schall03.StandardID:
		return schall03.StandardData(), true
	default:
		return framework.StandardData{}, false
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
	const scope = "cli.parseDummyRunOptions"

	options := dummyRunOptions{}

	for _, field := range []struct {
		key      string
		target   *float64
		minValue float64
	}{
		{"grid_resolution_m", &options.GridResolutionM, 0.001},
		{"grid_padding_m", &options.GridPaddingM, 0},
		{"receiver_height_m", &options.ReceiverHeightM, 0},
		{"source_emission_db", &options.SourceEmission, 0},
	} {
		err := parseMinFloatParam(scope, params, field.key, field.target, field.minValue)
		if err != nil {
			return dummyRunOptions{}, err
		}
	}

	for _, field := range []struct {
		key      string
		target   *int
		minValue int
	}{
		{"workers", &options.Workers, 0},
		{"chunk_size", &options.ChunkSize, 1},
	} {
		err := parseMinIntParam(scope, params, field.key, field.target, field.minValue)
		if err != nil {
			return dummyRunOptions{}, err
		}
	}

	rawDisable, ok := params["disable_cache"]
	if !ok {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindInternal, scope, `normalized parameter "disable_cache" missing`, nil)
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(rawDisable))
	if err != nil {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid disable_cache=%q", rawDisable), err)
	}

	options.DisableCache = parsed

	return options, nil
}

func parseCnossosRoadRunOptions(params map[string]string) (cnossosRoadRunOptions, error) {
	const scope = "cli.parseCnossosRoadRunOptions"

	options := cnossosRoadRunOptions{}

	err := parseFiniteFloatParams(scope, params, []floatParam{
		{"grid_resolution_m", &options.GridResolutionM},
		{"grid_padding_m", &options.GridPaddingM},
		{"receiver_height_m", &options.ReceiverHeightM},
	})
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	surfaceType, err := stringParamValue(scope, params, "road_surface_type")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	roadCategory, err := stringParamValue(scope, params, "road_category")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	options.RoadCategory = roadCategory
	options.SurfaceType = surfaceType

	err = parseFiniteFloatParams(scope, params, []floatParam{
		{"road_speed_kph", &options.SpeedKPH},
		{"road_gradient_percent", &options.GradientPercent},
	})
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	junctionType, err := stringParamValue(scope, params, "road_junction_type")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	options.JunctionType = junctionType

	err = parseFiniteFloatParams(scope, params, []floatParam{
		{"road_junction_distance_m", &options.JunctionDistanceM},
		{"road_temperature_c", &options.TemperatureC},
		{"road_studded_tyre_share", &options.StuddedTyreShare},
		{"traffic_day_light_vph", &options.TrafficDayLightVPH},
		{"traffic_day_medium_vph", &options.TrafficDayMediumVPH},
		{"traffic_day_heavy_vph", &options.TrafficDayHeavyVPH},
		{"traffic_evening_light_vph", &options.TrafficEveningLightVPH},
		{"traffic_evening_medium_vph", &options.TrafficEveningMediumVPH},
		{"traffic_evening_heavy_vph", &options.TrafficEveningHeavyVPH},
		{"traffic_night_light_vph", &options.TrafficNightLightVPH},
		{"traffic_night_medium_vph", &options.TrafficNightMediumVPH},
		{"traffic_night_heavy_vph", &options.TrafficNightHeavyVPH},
		{"traffic_day_ptw_vph", &options.TrafficDayPTWVPH},
		{"traffic_evening_ptw_vph", &options.TrafficEveningPTWVPH},
		{"traffic_night_ptw_vph", &options.TrafficNightPTWVPH},
		{"air_absorption_db_per_km", &options.AirAbsorptionDBPerKM},
		{"ground_attenuation_db", &options.GroundAttenuationDB},
		{"barrier_attenuation_db", &options.BarrierAttenuationDB},
		{"min_distance_m", &options.MinDistanceM},
	})
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}

	return options, nil
}

// fillSharedRailRunOptions fills every rail run option that cnossos-rail and
// bub-rail declare alike. The two parameter schemas differ only in
// rail_track_type, which each caller resolves for itself.
func fillSharedRailRunOptions(scope string, params map[string]string, options *cnossosRailRunOptions) error {
	err := parseFiniteFloatParams(scope, params, []floatParam{
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
	})
	if err != nil {
		return err
	}

	err = assignStringParams(scope, params, []stringParam{
		{"rail_traction_type", &options.TractionType},
		{"rail_track_roughness_class", &options.TrackRoughnessClass},
	})
	if err != nil {
		return err
	}

	rawOnBridge, err := stringParamValue(scope, params, "rail_on_bridge")
	if err != nil {
		return err
	}

	options.OnBridge, err = strconv.ParseBool(rawOnBridge)
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, scope, fmt.Sprintf("invalid rail_on_bridge=%q", rawOnBridge), err)
	}

	return nil
}

func parseCnossosRailRunOptions(params map[string]string) (cnossosRailRunOptions, error) {
	const scope = "cli.parseCnossosRailRunOptions"

	options := cnossosRailRunOptions{}

	err := fillSharedRailRunOptions(scope, params, &options)
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	options.TrackType, err = stringParamValue(scope, params, "rail_track_type")
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	return options, nil
}

// parseBUBRailRunOptions parses the bub-rail schema, which publishes no
// rail_track_type although the aliased rail source model still requires one.
// The run therefore starts from ballasted track, which a feature's own
// rail_track_type property still overrides.
func parseBUBRailRunOptions(params map[string]string) (bubRailRunOptions, error) {
	const scope = "cli.parseBUBRailRunOptions"

	options := bubRailRunOptions{TrackType: bubrail.TrackTypeBallasted}

	err := fillSharedRailRunOptions(scope, params, &options)
	if err != nil {
		return bubRailRunOptions{}, err
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

	err := fillSchall03RunOptions(&options, params)
	if err != nil {
		return schall03RunOptions{}, err
	}

	return options, nil
}

func fillSchall03RunOptions(options *schall03RunOptions, params map[string]string) error {
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
			return err
		}
	}

	for _, item := range []struct {
		key    string
		target *string
	}{
		{"rail_traction_type", &options.TractionType},
		{"rail_train_class", &options.TrainClass},
		{"rail_track_type", &options.TrackType},
		{"rail_track_form", &options.TrackForm},
		{"rail_track_roughness_class", &options.TrackRoughnessClass},
		{schall03.ParamEngine, &options.Engine},
	} {
		value, err := getString(item.key)
		if err != nil {
			return err
		}

		*item.target = value
	}

	onBridge, err := parseRailOnBridge(params)
	if err != nil {
		return err
	}

	options.OnBridge = onBridge

	return nil
}

func parseRailOnBridge(params map[string]string) (bool, error) {
	rawOnBridge, ok := params["rail_on_bridge"]
	if !ok {
		return false, domainerrors.New(domainerrors.KindInternal, "cli.parseSchall03RunOptions", `normalized parameter "rail_on_bridge" missing`, nil)
	}

	onBridge, err := strconv.ParseBool(strings.TrimSpace(rawOnBridge))
	if err != nil {
		return false, domainerrors.New(domainerrors.KindUserInput, "cli.parseSchall03RunOptions", fmt.Sprintf("invalid rail_on_bridge=%q", rawOnBridge), err)
	}

	return onBridge, nil
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

func parseAircraftRunOptions(params map[string]string, contextName string) (aircraftRunOptions, error) {
	options := aircraftRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, contextName, fmt.Sprintf("normalized parameter %q missing", key), nil)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, contextName, fmt.Sprintf("invalid %s=%q", key, value), err)
		}

		*target = parsed

		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, contextName, fmt.Sprintf("normalized parameter %q missing", key), nil)
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
			return aircraftRunOptions{}, err
		}
	}

	var err error

	options.AirportID, err = getString("airport_id")
	if err != nil {
		return aircraftRunOptions{}, err
	}

	options.RunwayID, err = getString("runway_id")
	if err != nil {
		return aircraftRunOptions{}, err
	}

	options.OperationType, err = getString("aircraft_operation_type")
	if err != nil {
		return aircraftRunOptions{}, err
	}

	options.AircraftClass, err = getString("aircraft_class")
	if err != nil {
		return aircraftRunOptions{}, err
	}

	options.ProcedureType, err = getString("aircraft_procedure_type")
	if err != nil {
		return aircraftRunOptions{}, err
	}

	options.ThrustMode, err = getString("aircraft_thrust_mode")
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

func parseBUFAircraftRunOptions(params map[string]string) (bufAircraftRunOptions, error) {
	options, err := parseAircraftRunOptions(params, "cli.parseBUFAircraftRunOptions")
	if err != nil {
		return bufAircraftRunOptions{}, err
	}

	return toBUFAircraftRunOptions(options), nil
}

func toCnossosAircraftRunOptions(options aircraftRunOptions) cnossosAircraftRunOptions {
	return cnossosAircraftRunOptions(options)
}

func toBUFAircraftRunOptions(options aircraftRunOptions) bufAircraftRunOptions {
	return bufAircraftRunOptions(options)
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

// fillSharedIndustryRunOptions fills every industry run option that
// cnossos-industry and bub-industry declare alike. The two parameter schemas
// differ only in industry_source_category and industry_enclosure_state, which
// each caller resolves for itself.
func fillSharedIndustryRunOptions(scope string, params map[string]string, options *cnossosIndustryRunOptions) error {
	return parseFiniteFloatParams(scope, params, []floatParam{
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
	})
}

func parseCnossosIndustryRunOptions(params map[string]string) (cnossosIndustryRunOptions, error) {
	const scope = "cli.parseCnossosIndustryRunOptions"

	options := cnossosIndustryRunOptions{}

	err := assignStringParams(scope, params, []stringParam{
		{"industry_source_category", &options.SourceCategory},
		{"industry_enclosure_state", &options.EnclosureState},
	})
	if err != nil {
		return cnossosIndustryRunOptions{}, err
	}

	err = fillSharedIndustryRunOptions(scope, params, &options)
	if err != nil {
		return cnossosIndustryRunOptions{}, err
	}

	return options, nil
}

// parseBUBIndustryRunOptions parses the bub-industry schema, which publishes
// neither industry_source_category nor industry_enclosure_state although the
// aliased industry source model still requires both. The run therefore starts
// from an open process source, which a feature's own industry_source_category
// and industry_enclosure_state properties still override.
func parseBUBIndustryRunOptions(params map[string]string) (bubIndustryRunOptions, error) {
	const scope = "cli.parseBUBIndustryRunOptions"

	options := bubIndustryRunOptions{
		SourceCategory: bubindustry.CategoryProcess,
		EnclosureState: bubindustry.EnclosureOpen,
	}

	err := fillSharedIndustryRunOptions(scope, params, &options)
	if err != nil {
		return bubIndustryRunOptions{}, err
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
		{"c0_met", &options.C0Met},
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
		C0:                      o.C0Met,
		MinDistanceM:            o.MinDistanceM,
	}
}
