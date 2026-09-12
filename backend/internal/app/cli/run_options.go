package cli

import (
	"fmt"
	"maps"
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
	dummyfreefield "github.com/aconiq/backend/internal/standards/dummy/freefield"
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

// runOptionParamKeys returns the normalized parameter names the CLI binds for
// one standard ID, in binding order. The second result is false for an ID the
// run pipeline does not parse options for.
//
// It exists so TestRunOptionsCoverParameterSchema can compare the binding tables
// against the parameter schemas the standards modules publish. Reading the keys
// off the same tables the parsers run is the point: a table that stops matching
// its schema fails the test rather than silently dropping a parameter.
func runOptionParamKeys(standardID string) ([]string, bool) {
	switch standardID {
	case dummyfreefield.StandardID:
		return boundParamKeys(dummyParamBindings(&dummyRunOptions{})), true
	case cnossosroad.StandardID:
		return boundParamKeys(cnossosRoadParamBindings(&cnossosRoadRunOptions{})), true
	case cnossosrail.StandardID:
		return boundParamKeys(cnossosRailParamBindings(&cnossosRailRunOptions{})), true
	case bubrail.StandardID:
		return boundParamKeys(bubRailParamBindings(&bubRailRunOptions{})), true
	case cnossosindustry.StandardID:
		return boundParamKeys(cnossosIndustryParamBindings(&cnossosIndustryRunOptions{})), true
	case bubindustry.StandardID:
		return boundParamKeys(bubIndustryParamBindings(&bubIndustryRunOptions{})), true
	case cnossosaircraft.StandardID, bufaircraft.StandardID:
		return boundParamKeys(aircraftParamBindings(&aircraftRunOptions{})), true
	case bubroad.StandardID:
		return boundParamKeys(bubRoadParamBindings(&bubRoadRunOptions{})), true
	case rls19road.StandardID:
		return boundParamKeys(rls19RoadParamBindings(&rls19RoadRunOptions{})), true
	case schall03.StandardID:
		return boundParamKeys(schall03ParamBindings(&schall03RunOptions{})), true
	case bebexposure.StandardID:
		return boundParamKeys(bebExposureParamBindings(&bebExposureRunOptions{})), true
	case iso9613.StandardID:
		return boundParamKeys(iso9613ParamBindings(&iso9613RunOptions{})), true
	default:
		return nil, false
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

// dummyParamBindings binds the dummy-freefield parameter schema.
//
// It is the only table that carries the engine controls, and the only one whose
// grid terms are range-checked here rather than by the schema alone.
func dummyParamBindings(options *dummyRunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.minFloat(&options.GridResolutionM, 0.001),
		runParams.GridPaddingM.minFloat(&options.GridPaddingM, 0),
		runParams.ReceiverHeightM.minFloat(&options.ReceiverHeightM, 0),
		runParams.SourceEmissionDB.minFloat(&options.SourceEmission, 0),
		runParams.Workers.minInt(&options.Workers, 0),
		runParams.ChunkSize.minInt(&options.ChunkSize, 1),
		runParams.DisableCache.boolean(&options.DisableCache),
	}
}

func parseDummyRunOptions(params map[string]string) (dummyRunOptions, error) {
	options := dummyRunOptions{}

	err := applyBoundParams("cli.parseDummyRunOptions", params, dummyParamBindings(&options))
	if err != nil {
		return dummyRunOptions{}, err
	}

	return options, nil
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

// bubRailParamBindings binds the bub-rail parameter schema, which publishes no
// rail_track_type although the aliased rail source model still requires one.
func bubRailParamBindings(options *bubRailRunOptions) []boundParam {
	return sharedRailParamBindings(options)
}

func parseCnossosRailRunOptions(params map[string]string) (cnossosRailRunOptions, error) {
	options := cnossosRailRunOptions{}

	err := applyBoundParams("cli.parseCnossosRailRunOptions", params, cnossosRailParamBindings(&options))
	if err != nil {
		return cnossosRailRunOptions{}, err
	}

	return options, nil
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

func (o cnossosRailRunOptions) PropagationConfig() cnossosrail.PropagationConfig {
	return cnossosrail.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		BridgeCorrectionDB:   o.BridgeCorrectionDB,
		CurveSquealDB:        o.CurveSquealDB,
		MinDistanceM:         o.MinDistanceM,
	}
}

// schall03ParamBindings binds the schall03 parameter schema.
func schall03ParamBindings(options *schall03RunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.RailAverageTrainSpeedKPH.float(&options.AverageTrainSpeedKPH),
		runParams.RailCurveRadiusM.float(&options.CurveRadiusM),
		runParams.TrafficDayTrainsPerHour.float(&options.TrafficDayTrainsPH),
		runParams.TrafficNightTrainsPerHour.float(&options.TrafficNightTrainsPH),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.SlabTrackCorrectionDB.float(&options.SlabTrackCorrectionDB),
		runParams.BridgeCorrectionDB.float(&options.BridgeCorrectionDB),
		runParams.CurveCorrectionDB.float(&options.CurveCorrectionDB),
		runParams.MinDistanceM.float(&options.MinDistanceM),
		runParams.RailTractionType.str(&options.TractionType),
		runParams.RailTrainClass.str(&options.TrainClass),
		runParams.RailTrackType.str(&options.TrackType),
		runParams.RailTrackForm.str(&options.TrackForm),
		runParams.RailTrackRoughnessClass.str(&options.TrackRoughnessClass),
		runParams.Schall03Engine.str(&options.Engine),
		runParams.RailOnBridge.boolean(&options.OnBridge),
	}
}

func parseSchall03RunOptions(params map[string]string) (schall03RunOptions, error) {
	options := schall03RunOptions{}

	err := applyBoundParams("cli.parseSchall03RunOptions", params, schall03ParamBindings(&options))
	if err != nil {
		return schall03RunOptions{}, err
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

// rls19RoadParamBindings binds the rls19-road parameter schema, which carries
// its own speed and traffic vocabulary rather than the shared road one.
func rls19RoadParamBindings(options *rls19RoadRunOptions) []boundParam {
	return []boundParam{
		runParams.SurfaceType.str(&options.SurfaceType),
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.SpeedPkwKPH.float(&options.SpeedPkwKPH),
		runParams.SpeedLkw1KPH.float(&options.SpeedLkw1KPH),
		runParams.SpeedLkw2KPH.float(&options.SpeedLkw2KPH),
		runParams.SpeedKradKPH.float(&options.SpeedKradKPH),
		runParams.GradientPercent.float(&options.GradientPercent),
		runParams.TrafficDayPkw.float(&options.TrafficDayPkw),
		runParams.TrafficDayLkw1.float(&options.TrafficDayLkw1),
		runParams.TrafficDayLkw2.float(&options.TrafficDayLkw2),
		runParams.TrafficDayKrad.float(&options.TrafficDayKrad),
		runParams.TrafficNightPkw.float(&options.TrafficNightPkw),
		runParams.TrafficNightLkw1.float(&options.TrafficNightLkw1),
		runParams.TrafficNightLkw2.float(&options.TrafficNightLkw2),
		runParams.TrafficNightKrad.float(&options.TrafficNightKrad),
		runParams.SegmentLengthM.float(&options.SegmentLengthM),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

func parseRLS19RoadRunOptions(params map[string]string) (rls19RoadRunOptions, error) {
	options := rls19RoadRunOptions{}

	err := applyBoundParams("cli.parseRLS19RoadRunOptions", params, rls19RoadParamBindings(&options))
	if err != nil {
		return rls19RoadRunOptions{}, err
	}

	return options, nil
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

// bubIndustryParamBindings binds the bub-industry parameter schema, which
// publishes neither industry_source_category nor industry_enclosure_state.
func bubIndustryParamBindings(options *bubIndustryRunOptions) []boundParam {
	return sharedIndustryParamBindings(options)
}

func parseCnossosIndustryRunOptions(params map[string]string) (cnossosIndustryRunOptions, error) {
	options := cnossosIndustryRunOptions{}

	err := applyBoundParams("cli.parseCnossosIndustryRunOptions", params, cnossosIndustryParamBindings(&options))
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

func (o cnossosIndustryRunOptions) PropagationConfig() cnossosindustry.PropagationConfig {
	return cnossosindustry.PropagationConfig{
		AirAbsorptionDBPerKM:   o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:    o.GroundAttenuationDB,
		ScreeningAttenuationDB: o.ScreeningAttenuationDB,
		FacadeReflectionDB:     o.FacadeReflectionDB,
		MinDistanceM:           o.MinDistanceM,
	}
}

// iso9613ParamBindings binds the iso9613 parameter schema.
func iso9613ParamBindings(options *iso9613RunOptions) []boundParam {
	return []boundParam{
		runParams.MeteorologyAssumption.str(&options.MeteorologyAssumption),
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.ISO9613SourceHeightM.float(&options.SourceHeightM),
		runParams.ISO9613SoundPowerLevelDB.float(&options.SoundPowerLevelDB),
		runParams.ISO9613DirectivityCorrectionDB.float(&options.DirectivityCorrectionDB),
		runParams.ISO9613TonalityCorrectionDB.float(&options.TonalityCorrectionDB),
		runParams.ISO9613ImpulsivityCorrectionDB.float(&options.ImpulsivityCorrectionDB),
		runParams.GroundFactor.float(&options.GroundFactor),
		runParams.AirTemperatureC.float(&options.AirTemperatureC),
		runParams.RelativeHumidityPercent.float(&options.RelativeHumidityPercent),
		runParams.C0Met.float(&options.C0Met),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

func parseISO9613RunOptions(params map[string]string) (iso9613RunOptions, error) {
	options := iso9613RunOptions{}

	err := applyBoundParams("cli.parseISO9613RunOptions", params, iso9613ParamBindings(&options))
	if err != nil {
		return iso9613RunOptions{}, err
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
