package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
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
	TractionType          string
	TrackType             string
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

func newRunCommand() *cobra.Command {
	var scenarioID string
	var standardID string
	var standardVersion string
	var standardProfile string
	var modelPath string
	var receiverMode string
	var rawParams []string
	var inputPaths []string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a run and persist result artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.run", "command state unavailable", nil)
			}

			params, err := parseKeyValueFlags(rawParams)
			if err != nil {
				return err
			}

			if err := validateReceiverMode(receiverMode); err != nil {
				return err
			}

			registry, err := standards.NewRegistry()
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.run", "initialize standards registry", err)
			}

			resolvedStandard, err := registry.Resolve(standardID, standardVersion, standardProfile)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
			}

			resolvedParams, err := resolvedStandard.RunParameterSchema.NormalizeAndValidate(params)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			resolvedModelPath := resolvePath(store.Root(), modelPath)
			relModelPath := relativePath(store.Root(), resolvedModelPath)
			combinedInputs := mergeInputPaths(append([]string{relModelPath}, inputPaths...))

			run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
				ScenarioID: scenarioID,
				Standard: project.StandardRef{
					Context: resolvedStandard.Context,
					ID:      resolvedStandard.StandardID,
					Version: resolvedStandard.Version,
					Profile: resolvedStandard.Profile,
				},
				ReceiverMode:  receiverMode,
				ReceiverSetID: receiverSetID(receiverMode),
				Parameters:    resolvedParams,
				Metadata:      buildRunProvenanceMetadata(resolvedStandard.StandardID, resolvedParams, receiverMode),
				InputPaths:    combinedInputs,
				Status:        project.RunStatusRunning,
				LogLines: []string{
					nowUTC().Format(time.RFC3339) + " run started",
				},
			})
			if err != nil {
				return err
			}

			logLines := []string{
				run.StartedAt.Format(time.RFC3339) + " run started",
				fmt.Sprintf("%s standard=%s version=%s profile=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.StandardID, resolvedStandard.Version, resolvedStandard.Profile),
				fmt.Sprintf("%s model=%s", run.StartedAt.Format(time.RFC3339), relModelPath),
				fmt.Sprintf("%s receiver_mode=%s", run.StartedAt.Format(time.RFC3339), receiverMode),
			}

			model, err := loadValidatedModel(resolvedModelPath, proj.CRS, relModelPath)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to load model: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			runDir := filepath.Join(store.Root(), ".noise", "runs", run.ID)
			var persisted persistedRunOutputs
			var outputHash string
			var finishedAt time.Time
			var sourceCount int
			var receiverCount int

			switch resolvedStandard.StandardID {
			case freefield.StandardID:
				options, parseErr := parseDummyRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				sources, extractErr := extractDummySources(model, options.SourceEmission, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildDummyReceivers(sources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(sources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				engineRunner := engine.NewRunner(func(event engine.ProgressEvent) {
					if event.Stage == "compute" && event.Message == "chunk_done" {
						logLines = append(logLines, fmt.Sprintf("%s stage=%s chunk=%d %d/%d", event.Time.Format(time.RFC3339), event.Stage, event.ChunkIndex, event.CompletedChunks, event.TotalChunks))
						return
					}

					logLines = append(logLines, fmt.Sprintf("%s stage=%s message=%s", event.Time.Format(time.RFC3339), event.Stage, event.Message))
				})

				engineSources := make([]engine.Source, 0, len(sources))
				for _, source := range sources {
					engineSources = append(engineSources, engine.Source{
						ID:       source.ID,
						Point:    source.Point,
						Emission: source.EmissionDB,
					})
				}

				runOutput, runErr := engineRunner.Run(context.Background(), engine.RunConfig{
					RunID:          run.ID,
					Workers:        options.Workers,
					ChunkSize:      options.ChunkSize,
					CacheDir:       state.Config.CacheDir,
					Receivers:      receivers,
					Sources:        engineSources,
					DisableCache:   options.DisableCache,
					DeterminismTag: "phase8-dummy-freefield",
				})
				if runErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s engine failed: %v", nowUTC().Format(time.RFC3339), runErr))
					return finalizeRunFailure(store, run, logLines, runErr)
				}

				persisted, err = persistDummyRunOutputs(runDir, runOutput, receivers, gridWidth, gridHeight, firstIndicator(resolvedStandard.SupportedIndicators))
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}

				outputHash = runOutput.OutputHash
				finishedAt = runOutput.FinishedAt
			case cnossosroad.StandardID:
				options, parseErr := parseCnossosRoadRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				roadSources, extractErr := extractCnossosRoadSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract road sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildCnossosRoadReceivers(roadSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(roadSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := cnossosroad.ComputeReceiverOutputs(receivers, roadSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s cnossos compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistCnossosRoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case cnossosrail.StandardID:
				options, parseErr := parseCnossosRailRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				railSources, extractErr := extractCnossosRailSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract rail sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildCnossosRailReceivers(railSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(railSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s rail_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := cnossosrail.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s cnossos rail compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistCnossosRailRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case bubroad.StandardID:
				options, parseErr := parseBUBRoadRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				roadSources, extractErr := extractBUBRoadSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract BUB road sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildBUBRoadReceivers(roadSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(roadSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s bub_road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := bubroad.ComputeReceiverOutputs(receivers, roadSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s bub road compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistBUBRoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case rls19road.StandardID:
				options, parseErr := parseRLS19RoadRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				roadSources, extractErr := extractRLS19RoadSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract RLS-19 road sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				barriers, extractErr := extractRLS19Barriers(model)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract RLS-19 barriers: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				buildings, extractErr := extractRLS19Buildings(model)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract RLS-19 buildings: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildRLS19RoadReceivers(roadSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(roadSources)
				receiverCount = len(receivers)
				logLines = append(
					logLines,
					fmt.Sprintf("%s rls19_road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount),
					fmt.Sprintf("%s rls19_barriers=%d", nowUTC().Format(time.RFC3339), len(barriers)),
					fmt.Sprintf("%s rls19_buildings=%d", nowUTC().Format(time.RFC3339), len(buildings)),
				)
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				propagationConfig := options.PropagationConfig()
				propagationConfig.Buildings = buildings

				receiverOutputs, computeErr := rls19road.ComputeReceiverOutputs(receivers, roadSources, barriers, propagationConfig)
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s rls19 compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistRLS19RoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case schall03.StandardID:
				options, parseErr := parseSchall03RunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				railSources, extractErr := extractSchall03Sources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract Schall 03 rail sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildSchall03Receivers(railSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(railSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s schall03_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := schall03.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s schall03 compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistSchall03RunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case bebexposure.StandardID:
				if receiverMode == receiverModeCustom {
					return domainerrors.New(domainerrors.KindUserInput, "cli.run", "custom receiver mode is not supported for building exposure runs", nil)
				}

				options, parseErr := parseBEBExposureRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				buildings, extractErr := extractBEBBuildings(model, options)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract BEB buildings: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receiverCount = len(buildings)
				var buildingOutputs []bebexposure.BuildingExposureOutput
				var summary bebexposure.Summary

				switch options.UpstreamMappingStandard {
				case bebexposure.UpstreamStandardBUBRoad:
					roadSources, extractErr := extractBUBRoadSources(model, options.BUBRoadOptions(), []string{"line"})
					if extractErr != nil {
						logLines = append(logLines, fmt.Sprintf("%s failed to extract BEB upstream road sources: %v", nowUTC().Format(time.RFC3339), extractErr))
						return finalizeRunFailure(store, run, logLines, extractErr)
					}

					sourceCount = len(roadSources)
					logLines = append(
						logLines,
						fmt.Sprintf("%s beb_upstream_standard=%s", nowUTC().Format(time.RFC3339), options.UpstreamMappingStandard),
						fmt.Sprintf("%s beb_upstream_sources=%d", nowUTC().Format(time.RFC3339), sourceCount),
						fmt.Sprintf("%s beb_buildings=%d", nowUTC().Format(time.RFC3339), receiverCount),
					)

					buildingOutputs, summary, err = bebexposure.ComputeOutputs(
						buildings,
						roadSources,
						options.ExposureConfig(),
						options.BUBRoadOptions().PropagationConfig(),
						options.FacadeReceiverHeightM,
					)
				case bebexposure.UpstreamStandardBUFAircraft:
					aircraftSources, extractErr := extractBUFAircraftSources(model, options.BUFAircraftOptions(), []string{"line"})
					if extractErr != nil {
						logLines = append(logLines, fmt.Sprintf("%s failed to extract BEB upstream aircraft sources: %v", nowUTC().Format(time.RFC3339), extractErr))
						return finalizeRunFailure(store, run, logLines, extractErr)
					}

					sourceCount = len(aircraftSources)
					logLines = append(
						logLines,
						fmt.Sprintf("%s beb_upstream_standard=%s", nowUTC().Format(time.RFC3339), options.UpstreamMappingStandard),
						fmt.Sprintf("%s beb_upstream_sources=%d", nowUTC().Format(time.RFC3339), sourceCount),
						fmt.Sprintf("%s beb_buildings=%d", nowUTC().Format(time.RFC3339), receiverCount),
					)

					buildingOutputs, summary, err = bebexposure.ComputeOutputsFromAircraft(
						buildings,
						aircraftSources,
						options.ExposureConfig(),
						options.BUFAircraftOptions().PropagationConfig(),
						options.FacadeReceiverHeightM,
					)
				default:
					err = domainerrors.New(
						domainerrors.KindUserInput,
						"cli.run",
						fmt.Sprintf("unsupported BEB upstream_mapping_standard %q", options.UpstreamMappingStandard),
						nil,
					)
				}

				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s beb exposure compute failed: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}

				persisted, outputHash, finishedAt, err = persistBEBExposureRunOutputs(runDir, buildingOutputs, summary, sourceCount)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case bufaircraft.StandardID:
				options, parseErr := parseBUFAircraftRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				aircraftSources, extractErr := extractBUFAircraftSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract BUF aircraft sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildBUFAircraftReceivers(aircraftSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(aircraftSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s buf_aircraft_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := bufaircraft.ComputeReceiverOutputs(receivers, aircraftSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s buf aircraft compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistBUFAircraftRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case cnossosaircraft.StandardID:
				options, parseErr := parseCnossosAircraftRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				aircraftSources, extractErr := extractCnossosAircraftSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract aircraft sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildCnossosAircraftReceivers(aircraftSources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(aircraftSources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s aircraft_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := cnossosaircraft.ComputeReceiverOutputs(receivers, aircraftSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s cnossos aircraft compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistCnossosAircraftRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			case cnossosindustry.StandardID:
				options, parseErr := parseCnossosIndustryRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				industrySources, extractErr := extractCnossosIndustrySources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract industry sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
					return buildCnossosIndustryReceivers(industrySources, options)
				})
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(industrySources)
				receiverCount = len(receivers)
				logLines = append(logLines, fmt.Sprintf("%s industry_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
				if receiverMode == receiverModeCustom {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
				} else {
					logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
				}

				receiverOutputs, computeErr := cnossosindustry.ComputeReceiverOutputs(receivers, industrySources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s cnossos industry compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistCnossosIndustryRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, receiverMode)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			default:
				return domainerrors.New(
					domainerrors.KindUserInput,
					"cli.run",
					fmt.Sprintf("standard %q is registered but not wired in run pipeline yet", resolvedStandard.StandardID),
					nil,
				)
			}

			artifacts := buildRunArtifacts(store.Root(), run.ID, persisted)

			logLines = append(
				logLines,
				fmt.Sprintf("%s output_hash=%s", nowUTC().Format(time.RFC3339), outputHash),
				fmt.Sprintf("%s persisted=%s", nowUTC().Format(time.RFC3339), relativePath(store.Root(), persisted.SummaryPath)),
				nowUTC().Format(time.RFC3339)+" run completed",
			)

			err = finalizeRun(store, run, project.RunStatusCompleted, finishedAt, logLines, artifacts)
			if err != nil {
				return err
			}

			state.Logger.Info(
				"run completed",
				"run_id", run.ID,
				"status", project.RunStatusCompleted,
				"standard_id", run.Standard.ID,
				"provenance", provenance.ManifestPath,
				"output_hash", outputHash,
			)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Completed run %s (%s)\n", run.ID, project.RunStatusCompleted)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provenance: %s\n", provenance.ManifestPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Results: %s\n", relativePath(store.Root(), filepath.Join(runDir, "results")))

			return nil
		},
	}

	cmd.Flags().StringVar(&scenarioID, "scenario", "default", "Scenario ID")
	cmd.Flags().StringVar(&standardID, "standard", freefield.StandardID, "Standard identifier")
	cmd.Flags().StringVar(&standardID, "standard-id", freefield.StandardID, "Deprecated alias for --standard")
	cmd.Flags().StringVar(&standardVersion, "standard-version", "", "Standard version (defaults to standard default)")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "", "Standard profile (defaults to version profile default)")
	cmd.Flags().StringVar(&modelPath, "model", defaultModelPath, "Path to normalized GeoJSON model")
	cmd.Flags().StringVar(&receiverMode, "receiver-mode", receiverModeAutoGrid, "Receiver mode: auto-grid or custom")
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Run parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&inputPaths, "input", nil, "Input path to hash into provenance (repeatable)")

	return cmd
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
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}

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

	options.TrackType, err = getString("rail_track_type")
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

	if err := parseFloat("grid_resolution_m", &options.GridResolutionM); err != nil {
		return bubRoadRunOptions{}, err
	}

	if err := parseFloat("grid_padding_m", &options.GridPaddingM); err != nil {
		return bubRoadRunOptions{}, err
	}

	if err := parseFloat("receiver_height_m", &options.ReceiverHeightM); err != nil {
		return bubRoadRunOptions{}, err
	}

	var err error

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

func mergeInputPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))

	for _, rawPath := range paths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}

		normalized := filepath.ToSlash(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out
}

func firstIndicator(indicators []string) string {
	for _, indicator := range indicators {
		trimmed := strings.TrimSpace(indicator)
		if trimmed != "" {
			return trimmed
		}
	}

	return freefield.IndicatorLdummy
}

func loadValidatedModel(modelPath string, projectCRS string, sourcePath string) (modelgeojson.Model, error) {
	payload, err := os.ReadFile(modelPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return modelgeojson.Model{}, domainerrors.New(domainerrors.KindNotFound, "cli.loadValidatedModel", "model file not found: "+modelPath, err)
		}

		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindInternal, "cli.loadValidatedModel", "read model file "+modelPath, err)
	}

	model, err := modelgeojson.Normalize(payload, projectCRS, sourcePath)
	if err != nil {
		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", "normalize model file", err)
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
		}

		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", summarizeValidationErrors(messages, 5), nil)
	}

	return model, nil
}

func extractExplicitReceivers(model modelgeojson.Model) ([]geo.PointReceiver, error) {
	receivers := make([]geo.PointReceiver, 0)
	seen := make(map[string]struct{})

	for _, feature := range model.Features {
		if feature.Kind != "receiver" {
			continue
		}

		if feature.GeometryType != "Point" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q geometry must be Point", feature.ID), nil)
		}

		if feature.HeightM == nil || *feature.HeightM <= 0 || math.IsNaN(*feature.HeightM) || math.IsInf(*feature.HeightM, 0) {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q height_m must be finite and > 0", feature.ID), nil)
		}

		if _, exists := seen[feature.ID]; exists {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q is duplicated", feature.ID), nil)
		}

		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractExplicitReceivers", fmt.Sprintf("receiver %q: %v", feature.ID, err), nil)
		}

		receivers = append(receivers, geo.PointReceiver{
			ID:      feature.ID,
			Point:   point,
			HeightM: *feature.HeightM,
		})
		seen[feature.ID] = struct{}{}
	}

	if len(receivers) == 0 {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.extractExplicitReceivers", "custom receiver mode requires at least one explicit receiver in the model", nil)
	}

	return receivers, nil
}

func resolveReceiverSet(
	mode string,
	model modelgeojson.Model,
	buildGrid func() ([]geo.PointReceiver, int, int, error),
) ([]geo.PointReceiver, int, int, error) {
	if mode == receiverModeCustom {
		receivers, err := extractExplicitReceivers(model)
		if err != nil {
			return nil, 0, 0, err
		}

		return receivers, 0, 0, nil
	}

	return buildGrid()
}

func featurePropertyString(feature modelgeojson.Feature, keys ...string) (string, bool, error) {
	for _, key := range keys {
		raw, ok := feature.Properties[key]
		if !ok || raw == nil {
			continue
		}

		switch value := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return "", false, nil
			}

			return trimmed, true, nil
		default:
			return "", false, fmt.Errorf("property %q must be a string", key)
		}
	}

	return "", false, nil
}

func featurePropertyFloat(feature modelgeojson.Feature, keys ...string) (float64, bool, error) {
	for _, key := range keys {
		raw, ok := feature.Properties[key]
		if !ok || raw == nil {
			continue
		}

		value, hasValue, err := readFeatureFloat(raw)
		if err != nil {
			return 0, false, fmt.Errorf("property %q: %w", key, err)
		}

		if !hasValue {
			return 0, false, nil
		}

		return value, true, nil
	}

	return 0, false, nil
}

func featurePropertyBool(feature modelgeojson.Feature, keys ...string) (bool, bool, error) {
	for _, key := range keys {
		raw, ok := feature.Properties[key]
		if !ok || raw == nil {
			continue
		}

		switch value := raw.(type) {
		case bool:
			return value, true, nil
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return false, false, nil
			}

			parsed, err := strconv.ParseBool(trimmed)
			if err != nil {
				return false, false, fmt.Errorf("property %q must be a bool", key)
			}

			return parsed, true, nil
		default:
			return false, false, fmt.Errorf("property %q must be a bool", key)
		}
	}

	return false, false, nil
}

func readFeatureFloat(raw any) (float64, bool, error) {
	switch value := raw.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false, errors.New("must be finite")
		}

		return value, true, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, false, nil
		}

		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, false, errors.New("must be a finite number")
		}

		return parsed, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported type %T", raw)
	}
}

func extractDummySources(model modelgeojson.Model, emissionDB float64, supportedSourceTypes []string) ([]freefield.Source, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]freefield.Source, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractDummySources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}

			sources = append(sources, freefield.Source{
				ID:         sourceID,
				Point:      point,
				EmissionDB: emissionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", "model does not contain any supported source features", nil)
	}

	return sources, nil
}

func extractCnossosRoadSources(model modelgeojson.Model, options cnossosRoadRunOptions, supportedSourceTypes []string) ([]cnossosroad.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosroad.RoadSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosRoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			surfaceType := options.SurfaceType

			if value, ok, err := featurePropertyString(feature, "road_surface_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				surfaceType = value
			}

			roadCategory := options.RoadCategory

			if value, ok, err := featurePropertyString(feature, "road_category"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roadCategory = value
			}

			speedKPH := options.SpeedKPH

			if value, ok, err := featurePropertyFloat(feature, "road_speed_kph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				speedKPH = value
			}

			gradientPercent := options.GradientPercent

			if value, ok, err := featurePropertyFloat(feature, "road_gradient_percent"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				gradientPercent = value
			}

			junctionType := options.JunctionType

			if value, ok, err := featurePropertyString(feature, "road_junction_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionType = value
			}

			junctionDistanceM := options.JunctionDistanceM

			if value, ok, err := featurePropertyFloat(feature, "road_junction_distance_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionDistanceM = value
			}

			temperatureC := options.TemperatureC

			if value, ok, err := featurePropertyFloat(feature, "road_temperature_c"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				temperatureC = value
			}

			studdedTyreShare := options.StuddedTyreShare

			if value, ok, err := featurePropertyFloat(feature, "road_studded_tyre_share"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				studdedTyreShare = value
			}

			trafficDayLightVPH := options.TrafficDayLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayLightVPH = value
			}

			trafficDayMediumVPH := options.TrafficDayMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayMediumVPH = value
			}

			trafficDayHeavyVPH := options.TrafficDayHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayHeavyVPH = value
			}

			trafficEveningLightVPH := options.TrafficEveningLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningLightVPH = value
			}

			trafficEveningMediumVPH := options.TrafficEveningMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningMediumVPH = value
			}

			trafficEveningHeavyVPH := options.TrafficEveningHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningHeavyVPH = value
			}

			trafficNightLightVPH := options.TrafficNightLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightLightVPH = value
			}

			trafficNightMediumVPH := options.TrafficNightMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightMediumVPH = value
			}

			trafficNightHeavyVPH := options.TrafficNightHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightHeavyVPH = value
			}

			trafficDayPTWVPH := options.TrafficDayPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayPTWVPH = value
			}

			trafficEveningPTWVPH := options.TrafficEveningPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningPTWVPH = value
			}

			trafficNightPTWVPH := options.TrafficNightPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightPTWVPH = value
			}

			sources = append(sources, cnossosroad.RoadSource{
				ID:                sourceID,
				Centerline:        line,
				RoadCategory:      roadCategory,
				SurfaceType:       surfaceType,
				SpeedKPH:          speedKPH,
				GradientPercent:   gradientPercent,
				JunctionType:      junctionType,
				JunctionDistanceM: junctionDistanceM,
				TemperatureC:      temperatureC,
				StuddedTyreShare:  studdedTyreShare,
				TrafficDay: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficDayLightVPH,
					MediumVehiclesPerHour:     trafficDayMediumVPH,
					HeavyVehiclesPerHour:      trafficDayHeavyVPH,
					PoweredTwoWheelersPerHour: trafficDayPTWVPH,
				},
				TrafficEvening: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficEveningLightVPH,
					MediumVehiclesPerHour:     trafficEveningMediumVPH,
					HeavyVehiclesPerHour:      trafficEveningHeavyVPH,
					PoweredTwoWheelersPerHour: trafficEveningPTWVPH,
				},
				TrafficNight: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficNightLightVPH,
					MediumVehiclesPerHour:     trafficNightMediumVPH,
					HeavyVehiclesPerHour:      trafficNightHeavyVPH,
					PoweredTwoWheelersPerHour: trafficNightPTWVPH,
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractCnossosRailSources(model modelgeojson.Model, options cnossosRailRunOptions, supportedSourceTypes []string) ([]cnossosrail.RailSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosrail.RailSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosRailSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rail-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			tractionType := options.TractionType

			if value, ok, err := featurePropertyString(feature, "rail_traction_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				tractionType = value
			}

			trackType := options.TrackType

			if value, ok, err := featurePropertyString(feature, "rail_track_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackType = value
			}

			roughnessClass := options.TrackRoughnessClass

			if value, ok, err := featurePropertyString(feature, "rail_track_roughness_class"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roughnessClass = value
			}

			averageSpeedKPH := options.AverageTrainSpeedKPH

			if value, ok, err := featurePropertyFloat(feature, "rail_average_train_speed_kph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				averageSpeedKPH = value
			}

			brakingShare := options.BrakingShare

			if value, ok, err := featurePropertyFloat(feature, "rail_braking_share"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				brakingShare = value
			}

			curveRadiusM := options.CurveRadiusM

			if value, ok, err := featurePropertyFloat(feature, "rail_curve_radius_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				curveRadiusM = value
			}

			onBridge := options.OnBridge

			if value, ok, err := featurePropertyBool(feature, "rail_on_bridge"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				onBridge = value
			}

			trafficDay := options.TrafficDayTrainsPerHour

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_trains_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDay = value
			}

			trafficEvening := options.TrafficEveningTrainsPerHour

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_trains_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEvening = value
			}

			trafficNight := options.TrafficNightTrainsPerHour

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_trains_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNight = value
			}

			sources = append(sources, cnossosrail.RailSource{
				ID:                   sourceID,
				TrackCenterline:      line,
				TractionType:         tractionType,
				TrackType:            trackType,
				TrackRoughnessClass:  roughnessClass,
				AverageTrainSpeedKPH: averageSpeedKPH,
				BrakingShare:         brakingShare,
				CurveRadiusM:         curveRadiusM,
				OnBridge:             onBridge,
				TrafficDay:           cnossosrail.TrafficPeriod{TrainsPerHour: trafficDay},
				TrafficEvening:       cnossosrail.TrafficPeriod{TrainsPerHour: trafficEvening},
				TrafficNight:         cnossosrail.TrafficPeriod{TrainsPerHour: trafficNight},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractBUBRoadSources(model modelgeojson.Model, options bubRoadRunOptions, supportedSourceTypes []string) ([]bubroad.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]bubroad.RoadSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractBUBRoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("bub-road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			surfaceType := options.SurfaceType

			if value, ok, err := featurePropertyString(feature, "road_surface_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				surfaceType = value
			}

			roadFunctionClass := options.RoadFunctionClass

			if value, ok, err := featurePropertyString(feature, "road_function_class"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roadFunctionClass = value
			}

			junctionType := options.JunctionType

			if value, ok, err := featurePropertyString(feature, "road_junction_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionType = value
			}

			speedKPH := options.SpeedKPH

			if value, ok, err := featurePropertyFloat(feature, "road_speed_kph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				speedKPH = value
			}

			gradientPercent := options.GradientPercent

			if value, ok, err := featurePropertyFloat(feature, "road_gradient_percent"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				gradientPercent = value
			}

			junctionDistanceM := options.JunctionDistanceM

			if value, ok, err := featurePropertyFloat(feature, "road_junction_distance_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionDistanceM = value
			}

			temperatureC := options.TemperatureC

			if value, ok, err := featurePropertyFloat(feature, "road_temperature_c"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				temperatureC = value
			}

			studdedTyreShare := options.StuddedTyreShare

			if value, ok, err := featurePropertyFloat(feature, "road_studded_tyre_share"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				studdedTyreShare = value
			}

			trafficDayLightVPH := options.TrafficDayLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayLightVPH = value
			}

			trafficDayMediumVPH := options.TrafficDayMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayMediumVPH = value
			}

			trafficDayHeavyVPH := options.TrafficDayHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayHeavyVPH = value
			}

			trafficDayPTWVPH := options.TrafficDayPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayPTWVPH = value
			}

			trafficEveningLightVPH := options.TrafficEveningLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningLightVPH = value
			}

			trafficEveningMediumVPH := options.TrafficEveningMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningMediumVPH = value
			}

			trafficEveningHeavyVPH := options.TrafficEveningHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningHeavyVPH = value
			}

			trafficEveningPTWVPH := options.TrafficEveningPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_evening_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningPTWVPH = value
			}

			trafficNightLightVPH := options.TrafficNightLightVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_light_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightLightVPH = value
			}

			trafficNightMediumVPH := options.TrafficNightMediumVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_medium_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightMediumVPH = value
			}

			trafficNightHeavyVPH := options.TrafficNightHeavyVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_heavy_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightHeavyVPH = value
			}

			trafficNightPTWVPH := options.TrafficNightPTWVPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_ptw_vph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightPTWVPH = value
			}

			sources = append(sources, bubroad.RoadSource{
				ID:                sourceID,
				Centerline:        line,
				SurfaceType:       surfaceType,
				RoadFunctionClass: roadFunctionClass,
				SpeedKPH:          speedKPH,
				GradientPercent:   gradientPercent,
				JunctionType:      junctionType,
				JunctionDistanceM: junctionDistanceM,
				TemperatureC:      temperatureC,
				StuddedTyreShare:  studdedTyreShare,
				TrafficDay: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficDayLightVPH,
					MediumVehiclesPerHour:     trafficDayMediumVPH,
					HeavyVehiclesPerHour:      trafficDayHeavyVPH,
					PoweredTwoWheelersPerHour: trafficDayPTWVPH,
				},
				TrafficEvening: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficEveningLightVPH,
					MediumVehiclesPerHour:     trafficEveningMediumVPH,
					HeavyVehiclesPerHour:      trafficEveningHeavyVPH,
					PoweredTwoWheelersPerHour: trafficEveningPTWVPH,
				},
				TrafficNight: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficNightLightVPH,
					MediumVehiclesPerHour:     trafficNightMediumVPH,
					HeavyVehiclesPerHour:      trafficNightHeavyVPH,
					PoweredTwoWheelersPerHour: trafficNightPTWVPH,
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractRLS19RoadSources(model modelgeojson.Model, options rls19RoadRunOptions, supportedSourceTypes []string) ([]rls19road.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]rls19road.RoadSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractRLS19RoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			surfaceType := options.SurfaceType

			if value, ok, err := featurePropertyString(feature, "surface_type", "road_surface_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				surfaceType = value
			}

			speedPkwKPH := options.SpeedPkwKPH
			speedLkw1KPH := options.SpeedLkw1KPH
			speedLkw2KPH := options.SpeedLkw2KPH
			speedKradKPH := options.SpeedKradKPH

			if value, ok, err := featurePropertyFloat(feature, "road_speed_kph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				speedPkwKPH = value
				speedLkw1KPH = value
				speedLkw2KPH = value
				speedKradKPH = value
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"speed_pkw_kph"}, &speedPkwKPH},
				{[]string{"speed_lkw1_kph"}, &speedLkw1KPH},
				{[]string{"speed_lkw2_kph"}, &speedLkw2KPH},
				{[]string{"speed_krad_kph"}, &speedKradKPH},
			} {
				if value, ok, err := featurePropertyFloat(feature, item.keys...); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					*item.target = value
				}
			}

			gradientPercent := options.GradientPercent

			if value, ok, err := featurePropertyFloat(feature, "gradient_percent", "road_gradient_percent"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				gradientPercent = value
			}

			junctionDistanceM := 0.0

			if value, ok, err := featurePropertyFloat(feature, "junction_distance_m", "road_junction_distance_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionDistanceM = value
			}

			reflectionSurchargeDB := 0.0

			if value, ok, err := featurePropertyFloat(feature, "reflection_surcharge_db"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				reflectionSurchargeDB = value
			}

			junctionType := rls19road.JunctionNone

			if value, ok, err := featurePropertyString(feature, "junction_type", "road_junction_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				parsed, err := rls19road.ParseJunctionType(value)
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				}

				junctionType = parsed
			}

			trafficDay := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficDayPkw,
				Lkw1PerHour: options.TrafficDayLkw1,
				Lkw2PerHour: options.TrafficDayLkw2,
				KradPerHour: options.TrafficDayKrad,
			}
			trafficNight := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficNightPkw,
				Lkw1PerHour: options.TrafficNightLkw1,
				Lkw2PerHour: options.TrafficNightLkw2,
				KradPerHour: options.TrafficNightKrad,
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"traffic_day_pkw"}, &trafficDay.PkwPerHour},
				{[]string{"traffic_day_lkw1"}, &trafficDay.Lkw1PerHour},
				{[]string{"traffic_day_lkw2"}, &trafficDay.Lkw2PerHour},
				{[]string{"traffic_day_krad"}, &trafficDay.KradPerHour},
				{[]string{"traffic_night_pkw"}, &trafficNight.PkwPerHour},
				{[]string{"traffic_night_lkw1"}, &trafficNight.Lkw1PerHour},
				{[]string{"traffic_night_lkw2"}, &trafficNight.Lkw2PerHour},
				{[]string{"traffic_night_krad"}, &trafficNight.KradPerHour},
			} {
				if value, ok, err := featurePropertyFloat(feature, item.keys...); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					*item.target = value
				}
			}

			source := rls19road.RoadSource{
				ID:          sourceID,
				Centerline:  line,
				SurfaceType: rls19road.SurfaceType(surfaceType),
				Speeds: rls19road.SpeedInput{
					PkwKPH:  speedPkwKPH,
					Lkw1KPH: speedLkw1KPH,
					Lkw2KPH: speedLkw2KPH,
					KradKPH: speedKradKPH,
				},
				GradientPercent:       gradientPercent,
				JunctionType:          junctionType,
				JunctionDistanceM:     junctionDistanceM,
				ReflectionSurchargeDB: reflectionSurchargeDB,
				TrafficDay:            trafficDay,
				TrafficNight:          trafficNight,
			}

			err := source.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractSchall03Sources(model modelgeojson.Model, options schall03RunOptions, supportedSourceTypes []string) ([]schall03.RailSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]schall03.RailSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractSchall03Sources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("schall03-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			tractionType := options.TractionType

			if value, ok, err := featurePropertyString(feature, "rail_traction_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				tractionType = value
			}

			trackType := options.TrackType

			if value, ok, err := featurePropertyString(feature, "rail_track_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackType = value
			}

			roughnessClass := options.TrackRoughnessClass

			if value, ok, err := featurePropertyString(feature, "rail_track_roughness_class"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roughnessClass = value
			}

			averageSpeedKPH := options.AverageTrainSpeedKPH

			if value, ok, err := featurePropertyFloat(feature, "rail_average_train_speed_kph"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				averageSpeedKPH = value
			}

			curveRadiusM := options.CurveRadiusM

			if value, ok, err := featurePropertyFloat(feature, "rail_curve_radius_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				curveRadiusM = value
			}

			onBridge := options.OnBridge

			if value, ok, err := featurePropertyBool(feature, "rail_on_bridge"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				onBridge = value
			}

			elevationM := 0.0

			if value, ok, err := featurePropertyFloat(feature, "elevation_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				elevationM = value
			}

			trafficDay := options.TrafficDayTrainsPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_day_trains_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDay = value
			}

			trafficNight := options.TrafficNightTrainsPH

			if value, ok, err := featurePropertyFloat(feature, "traffic_night_trains_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNight = value
			}

			sources = append(sources, schall03.RailSource{
				ID:              sourceID,
				TrackCenterline: line,
				ElevationM:      elevationM,
				AverageSpeedKPH: averageSpeedKPH,
				Infrastructure: schall03.RailInfrastructure{
					TractionType:        tractionType,
					TrackType:           trackType,
					TrackRoughnessClass: roughnessClass,
					OnBridge:            onBridge,
					CurveRadiusM:        curveRadiusM,
				},
				TrafficDay:   schall03.TrafficPeriod{TrainsPerHour: trafficDay},
				TrafficNight: schall03.TrafficPeriod{TrainsPerHour: trafficNight},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractRLS19Barriers(model modelgeojson.Model) ([]rls19road.Barrier, error) {
	barriers := make([]rls19road.Barrier, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "barrier" {
			continue
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "barrier_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q missing barrier height_m", feature.ID), nil)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-barrier-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			barrierID := baseID
			if len(lines) > 1 {
				barrierID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			barrier := rls19road.Barrier{
				ID:       barrierID,
				Geometry: line,
				HeightM:  heightM,
			}

			err := barrier.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
			}

			barriers = append(barriers, barrier)
		}
	}

	return barriers, nil
}

func extractRLS19Buildings(model modelgeojson.Model) ([]rls19road.Building, error) {
	buildings := make([]rls19road.Building, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "building" {
			continue
		}

		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "building_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q missing building height_m", feature.ID), nil)
		}

		reflectionLossDB := 1.0

		if value, ok, err := featurePropertyFloat(feature, "reflection_loss_db"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			reflectionLossDB = value
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-building-%03d", featureIndex)
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			building := rls19road.Building{
				ID:               buildingID,
				Footprint:        polygon[0],
				HeightM:          heightM,
				ReflectionLossDB: reflectionLossDB,
			}

			err := building.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
			}

			buildings = append(buildings, building)
		}
	}

	return buildings, nil
}

func extractCnossosAircraftSources(model modelgeojson.Model, options cnossosAircraftRunOptions, supportedSourceTypes []string) ([]cnossosaircraft.AircraftSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosaircraft.AircraftSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosAircraftSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		trackOptions := options

		if value, ok, err := featurePropertyFloat(feature, "track_start_height_m"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			trackOptions.TrackStartHeightM = value
		}

		if value, ok, err := featurePropertyFloat(feature, "track_end_height_m"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			trackOptions.TrackEndHeightM = value
		}

		tracks, err := flightTracksFromFeature(feature, trackOptions)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("aircraft-source-%03d", featureIndex)
		}

		for trackIndex, track := range tracks {
			sourceID := baseID
			if len(tracks) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, trackIndex+1)
			}

			airportID := options.AirportID

			if value, ok, err := featurePropertyString(feature, "airport_id"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				airportID = value
			}

			runwayID := options.RunwayID

			if value, ok, err := featurePropertyString(feature, "runway_id"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				runwayID = value
			}

			operationType := options.OperationType

			if value, ok, err := featurePropertyString(feature, "aircraft_operation_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				operationType = value
			}

			aircraftClass := options.AircraftClass

			if value, ok, err := featurePropertyString(feature, "aircraft_class"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				aircraftClass = value
			}

			procedureType := options.ProcedureType

			if value, ok, err := featurePropertyString(feature, "aircraft_procedure_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				procedureType = value
			}

			thrustMode := options.ThrustMode

			if value, ok, err := featurePropertyString(feature, "aircraft_thrust_mode"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				thrustMode = value
			}

			referencePowerLevelDB := options.ReferencePowerLevelDB

			if value, ok, err := featurePropertyFloat(feature, "reference_power_level_db"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				referencePowerLevelDB = value
			}

			engineStateFactor := options.EngineStateFactor

			if value, ok, err := featurePropertyFloat(feature, "engine_state_factor"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				engineStateFactor = value
			}

			bankAngleDeg := options.BankAngleDeg

			if value, ok, err := featurePropertyFloat(feature, "bank_angle_deg"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				bankAngleDeg = value
			}

			lateralOffsetM := options.LateralOffsetM

			if value, ok, err := featurePropertyFloat(feature, "lateral_offset_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				lateralOffsetM = value
			}

			movementDayPerHour := options.MovementDayPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_day_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementDayPerHour = value
			}

			movementEveningPerHour := options.MovementEveningPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_evening_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementEveningPerHour = value
			}

			movementNightPerHour := options.MovementNightPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_night_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementNightPerHour = value
			}

			sources = append(sources, cnossosaircraft.AircraftSource{
				ID:         sourceID,
				SourceType: cnossosaircraft.SourceTypeLine,
				Airport: cnossosaircraft.AirportRef{
					AirportID: airportID,
					RunwayID:  runwayID,
				},
				OperationType:         operationType,
				AircraftClass:         aircraftClass,
				ProcedureType:         procedureType,
				ThrustMode:            thrustMode,
				FlightTrack:           track,
				LateralOffsetM:        lateralOffsetM,
				ReferencePowerLevelDB: referencePowerLevelDB,
				EngineStateFactor:     engineStateFactor,
				BankAngleDeg:          bankAngleDeg,
				MovementDay:           cnossosaircraft.MovementPeriod{MovementsPerHour: movementDayPerHour},
				MovementEvening:       cnossosaircraft.MovementPeriod{MovementsPerHour: movementEveningPerHour},
				MovementNight:         cnossosaircraft.MovementPeriod{MovementsPerHour: movementNightPerHour},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractBUFAircraftSources(model modelgeojson.Model, options bufAircraftRunOptions, supportedSourceTypes []string) ([]bufaircraft.AircraftSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]bufaircraft.AircraftSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractBUFAircraftSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		trackOptions := options

		if value, ok, err := featurePropertyFloat(feature, "track_start_height_m"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			trackOptions.TrackStartHeightM = value
		}

		if value, ok, err := featurePropertyFloat(feature, "track_end_height_m"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			trackOptions.TrackEndHeightM = value
		}

		tracks, err := flightTracksFromFeatureBUF(feature, trackOptions)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("buf-aircraft-source-%03d", featureIndex)
		}

		for trackIndex, track := range tracks {
			sourceID := baseID
			if len(tracks) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, trackIndex+1)
			}

			airportID := options.AirportID

			if value, ok, err := featurePropertyString(feature, "airport_id"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				airportID = value
			}

			runwayID := options.RunwayID

			if value, ok, err := featurePropertyString(feature, "runway_id"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				runwayID = value
			}

			operationType := options.OperationType

			if value, ok, err := featurePropertyString(feature, "aircraft_operation_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				operationType = value
			}

			aircraftClass := options.AircraftClass

			if value, ok, err := featurePropertyString(feature, "aircraft_class"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				aircraftClass = value
			}

			procedureType := options.ProcedureType

			if value, ok, err := featurePropertyString(feature, "aircraft_procedure_type"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				procedureType = value
			}

			thrustMode := options.ThrustMode

			if value, ok, err := featurePropertyString(feature, "aircraft_thrust_mode"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				thrustMode = value
			}

			referencePowerLevelDB := options.ReferencePowerLevelDB

			if value, ok, err := featurePropertyFloat(feature, "reference_power_level_db"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				referencePowerLevelDB = value
			}

			engineStateFactor := options.EngineStateFactor

			if value, ok, err := featurePropertyFloat(feature, "engine_state_factor"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				engineStateFactor = value
			}

			bankAngleDeg := options.BankAngleDeg

			if value, ok, err := featurePropertyFloat(feature, "bank_angle_deg"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				bankAngleDeg = value
			}

			lateralOffsetM := options.LateralOffsetM

			if value, ok, err := featurePropertyFloat(feature, "lateral_offset_m"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				lateralOffsetM = value
			}

			movementDayPerHour := options.MovementDayPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_day_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementDayPerHour = value
			}

			movementEveningPerHour := options.MovementEveningPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_evening_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementEveningPerHour = value
			}

			movementNightPerHour := options.MovementNightPerHour

			if value, ok, err := featurePropertyFloat(feature, "movement_night_per_hour"); err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				movementNightPerHour = value
			}

			sources = append(sources, bufaircraft.AircraftSource{
				ID:         sourceID,
				SourceType: bufaircraft.SourceTypeLine,
				Airport: bufaircraft.AirportRef{
					AirportID: airportID,
					RunwayID:  runwayID,
				},
				OperationType:         operationType,
				AircraftClass:         aircraftClass,
				ProcedureType:         procedureType,
				ThrustMode:            thrustMode,
				FlightTrack:           track,
				LateralOffsetM:        lateralOffsetM,
				ReferencePowerLevelDB: referencePowerLevelDB,
				EngineStateFactor:     engineStateFactor,
				BankAngleDeg:          bankAngleDeg,
				MovementDay:           bufaircraft.MovementPeriod{MovementsPerHour: movementDayPerHour},
				MovementEvening:       bufaircraft.MovementPeriod{MovementsPerHour: movementEveningPerHour},
				MovementNight:         bufaircraft.MovementPeriod{MovementsPerHour: movementNightPerHour},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractBEBBuildings(model modelgeojson.Model, options bebExposureRunOptions) ([]bebexposure.BuildingUnit, error) {
	buildings := make([]bebexposure.BuildingUnit, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "building" {
			continue
		}

		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("beb-building-%03d", featureIndex)
		}

		heightM := options.MinimumBuildingHeightM
		if feature.HeightM != nil && *feature.HeightM > 0 {
			heightM = *feature.HeightM
		}

		usageType := options.BuildingUsageType

		if value, ok, err := featurePropertyString(feature, "building_usage_type", "usage_type"); err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		} else if ok {
			usageType = value
		}

		estimatedDwellings, hasEstimatedDwellings, err := featurePropertyFloat(feature, "estimated_dwellings")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		estimatedPersons, hasEstimatedPersons, err := featurePropertyFloat(feature, "estimated_persons", "occupancy", "occupants")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		floorCount, hasFloorCount, err := featurePropertyFloat(feature, "floor_count", "estimated_floors")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			var dwellingsOverride *float64

			if hasEstimatedDwellings {
				value := estimatedDwellings
				dwellingsOverride = &value
			}

			var personsOverride *float64

			if hasEstimatedPersons {
				value := estimatedPersons
				personsOverride = &value
			}

			var floorCountOverride *float64

			if hasFloorCount {
				value := floorCount
				floorCountOverride = &value
			}

			buildings = append(buildings, bebexposure.BuildingUnit{
				ID:                 buildingID,
				UsageType:          usageType,
				HeightM:            heightM,
				FloorCount:         floorCountOverride,
				EstimatedDwellings: dwellingsOverride,
				EstimatedPersons:   personsOverride,
				Footprint:          polygon,
			})
		}
	}

	if len(buildings) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", "model does not contain any building polygon features", nil)
	}

	return buildings, nil
}

func extractCnossosIndustrySources(model modelgeojson.Model, options cnossosIndustryRunOptions, supportedSourceTypes []string) ([]cnossosindustry.IndustrySource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosindustry.IndustrySource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType == "" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q source_type is required for cnossos-industry", feature.ID), nil)
		}

		if _, ok := allowedSourceType[normalizedSourceType]; !ok {
			return nil, domainerrors.New(
				domainerrors.KindValidation,
				"cli.extractCnossosIndustrySources",
				fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
				nil,
			)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("industry-source-%03d", featureIndex)
		}

		switch normalizedSourceType {
		case cnossosindustry.SourceTypePoint:
			points, err := sourcePointsFromFeature(feature)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			for pointIndex, point := range points {
				sourceID := baseID
				if len(points) > 1 {
					sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
				}

				sourceHeightM := options.SourceHeightM

				if value, ok, err := featurePropertyFloat(feature, "industry_source_height_m"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceHeightM = value
				}

				soundPowerLevelDB := options.SoundPowerLevelDB

				if value, ok, err := featurePropertyFloat(feature, "industry_sound_power_level_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					soundPowerLevelDB = value
				}

				sourceCategory := options.SourceCategory

				if value, ok, err := featurePropertyString(feature, "industry_source_category"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceCategory = value
				}

				enclosureState := options.EnclosureState

				if value, ok, err := featurePropertyString(feature, "industry_enclosure_state"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					enclosureState = value
				}

				tonalityCorrectionDB := options.TonalityCorrectionDB

				if value, ok, err := featurePropertyFloat(feature, "industry_tonality_correction_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tonalityCorrectionDB = value
				}

				impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

				if value, ok, err := featurePropertyFloat(feature, "industry_impulsivity_correction_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					impulsivityCorrectionDB = value
				}

				operationDayFactor := options.OperationDayFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_day_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationDayFactor = value
				}

				operationEveningFactor := options.OperationEveningFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_evening_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationEveningFactor = value
				}

				operationNightFactor := options.OperationNightFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_night_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationNightFactor = value
				}

				sources = append(sources, cnossosindustry.IndustrySource{
					ID:                      sourceID,
					SourceType:              cnossosindustry.SourceTypePoint,
					SourceCategory:          sourceCategory,
					EnclosureState:          enclosureState,
					Point:                   point,
					SourceHeightM:           sourceHeightM,
					SoundPowerLevelDB:       soundPowerLevelDB,
					TonalityCorrectionDB:    tonalityCorrectionDB,
					ImpulsivityCorrectionDB: impulsivityCorrectionDB,
					OperationDay:            cnossosindustry.OperationPeriod{OperatingFactor: operationDayFactor},
					OperationEvening:        cnossosindustry.OperationPeriod{OperatingFactor: operationEveningFactor},
					OperationNight:          cnossosindustry.OperationPeriod{OperatingFactor: operationNightFactor},
				})
			}
		case cnossosindustry.SourceTypeArea:
			polygons, err := polygonsFromFeature(feature)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			for polygonIndex, polygon := range polygons {
				sourceID := baseID
				if len(polygons) > 1 {
					sourceID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
				}

				sourceHeightM := options.SourceHeightM

				if value, ok, err := featurePropertyFloat(feature, "industry_source_height_m"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceHeightM = value
				}

				soundPowerLevelDB := options.SoundPowerLevelDB

				if value, ok, err := featurePropertyFloat(feature, "industry_sound_power_level_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					soundPowerLevelDB = value
				}

				sourceCategory := options.SourceCategory

				if value, ok, err := featurePropertyString(feature, "industry_source_category"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceCategory = value
				}

				enclosureState := options.EnclosureState

				if value, ok, err := featurePropertyString(feature, "industry_enclosure_state"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					enclosureState = value
				}

				tonalityCorrectionDB := options.TonalityCorrectionDB

				if value, ok, err := featurePropertyFloat(feature, "industry_tonality_correction_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tonalityCorrectionDB = value
				}

				impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

				if value, ok, err := featurePropertyFloat(feature, "industry_impulsivity_correction_db"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					impulsivityCorrectionDB = value
				}

				operationDayFactor := options.OperationDayFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_day_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationDayFactor = value
				}

				operationEveningFactor := options.OperationEveningFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_evening_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationEveningFactor = value
				}

				operationNightFactor := options.OperationNightFactor

				if value, ok, err := featurePropertyFloat(feature, "operation_night_factor"); err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationNightFactor = value
				}

				sources = append(sources, cnossosindustry.IndustrySource{
					ID:                      sourceID,
					SourceType:              cnossosindustry.SourceTypeArea,
					SourceCategory:          sourceCategory,
					EnclosureState:          enclosureState,
					AreaPolygon:             polygon,
					SourceHeightM:           sourceHeightM,
					SoundPowerLevelDB:       soundPowerLevelDB,
					TonalityCorrectionDB:    tonalityCorrectionDB,
					ImpulsivityCorrectionDB: impulsivityCorrectionDB,
					OperationDay:            cnossosindustry.OperationPeriod{OperatingFactor: operationDayFactor},
					OperationEvening:        cnossosindustry.OperationPeriod{OperatingFactor: operationEveningFactor},
					OperationNight:          cnossosindustry.OperationPeriod{OperatingFactor: operationNightFactor},
				})
			}
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", "model does not contain any supported point/area source features", nil)
	}

	return sources, nil
}

func sourcePointsFromFeature(feature modelgeojson.Feature) ([]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Point":
		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return []geo.Point2D{point}, nil
	case "MultiPoint":
		rawPoints, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPoint coordinates must be an array")
		}

		points := make([]geo.Point2D, 0, len(rawPoints))
		for _, raw := range rawPoints {
			point, err := parsePointCoordinate(raw)
			if err != nil {
				return nil, err
			}

			points = append(points, point)
		}

		return points, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (dummy-freefield supports Point/MultiPoint only)", feature.GeometryType)
	}
}

func lineStringsFromFeature(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
	switch feature.GeometryType {
	case "LineString":
		line, err := parseLineStringCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point2D{line}, nil
	case "MultiLineString":
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point2D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseLineStringCoordinates(rawLine)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func polygonsFromFeature(feature modelgeojson.Feature) ([][][]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Polygon":
		polygon, err := parsePolygonCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][][]geo.Point2D{polygon}, nil
	case "MultiPolygon":
		rawPolygons, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPolygon coordinates must be an array")
		}

		polygons := make([][][]geo.Point2D, 0, len(rawPolygons))
		for _, rawPolygon := range rawPolygons {
			polygon, err := parsePolygonCoordinates(rawPolygon)
			if err != nil {
				return nil, err
			}

			polygons = append(polygons, polygon)
		}

		return polygons, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-industry supports Point/MultiPoint/Polygon/MultiPolygon only)", feature.GeometryType)
	}
}

func flightTracksFromFeature(feature modelgeojson.Feature, options cnossosAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case "LineString":
		line, err := parseFlightTrackCoordinates(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case "MultiLineString":
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinates(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func flightTracksFromFeatureBUF(feature modelgeojson.Feature, options bufAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case "LineString":
		line, err := parseFlightTrackCoordinatesBUF(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case "MultiLineString":
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinatesBUF(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (buf-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func parseFlightTrackCoordinates(value any, options cnossosAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

func parseFlightTrackCoordinatesBUF(value any, options bufAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

func parsePointCoordinate3D(value any) (geo.Point2D, float64, bool, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be [x,y] or [x,y,z]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be finite")
	}

	if len(raw) < 3 {
		return point, 0, false, nil
	}

	z, err := parseCoordinateNumber(raw[2])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	if math.IsNaN(z) || math.IsInf(z, 0) {
		return geo.Point2D{}, 0, false, errors.New("point z must be finite")
	}

	return point, z, true, nil
}

func parsePolygonCoordinates(value any) ([][]geo.Point2D, error) {
	rawRings, ok := value.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, errors.New("polygon coordinates must contain at least one ring")
	}

	rings := make([][]geo.Point2D, 0, len(rawRings))
	for _, rawRing := range rawRings {
		ring, err := parseRingCoordinates(rawRing)
		if err != nil {
			return nil, err
		}

		rings = append(rings, ring)
	}

	return rings, nil
}

func parseRingCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok || len(rawPoints) < 4 {
		return nil, errors.New("polygon ring must contain at least 4 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parseLineStringCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parsePointCoordinate(value any) (geo.Point2D, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, errors.New("point coordinates must be [x,y]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, errors.New("point coordinates must be finite")
	}

	return point, nil
}

func parseCoordinateNumber(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric coordinate %q: %w", typed, err)
		}

		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate type %T", value)
	}
}

func buildDummyReceivers(sources []freefield.Source, options dummyRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildDummyReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildDummyReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildCnossosRoadReceivers(sources []cnossosroad.RoadSource, options cnossosRoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildCnossosRoadReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildCnossosRoadReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildCnossosRailReceivers(sources []cnossosrail.RailSource, options cnossosRailRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRailReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRailReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRailReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildCnossosRailReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildCnossosRailReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildBUBRoadReceivers(sources []bubroad.RoadSource, options bubRoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUBRoadReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUBRoadReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUBRoadReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildBUBRoadReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildBUBRoadReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildRLS19RoadReceivers(sources []rls19road.RoadSource, options rls19RoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildRLS19RoadReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildRLS19RoadReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildRLS19RoadReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildRLS19RoadReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildRLS19RoadReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildSchall03Receivers(sources []schall03.RailSource, options schall03RunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.TrackCenterline...)
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildSchall03Receivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildSchall03Receivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildSchall03Receivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildSchall03Receivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildSchall03Receivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildCnossosAircraftReceivers(sources []cnossosaircraft.AircraftSource, options cnossosAircraftRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		for _, point := range source.FlightTrack {
			sourcePoints = append(sourcePoints, point.XY())
		}
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosAircraftReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosAircraftReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosAircraftReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildCnossosAircraftReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildCnossosAircraftReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildBUFAircraftReceivers(sources []bufaircraft.AircraftSource, options bufAircraftRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		for _, point := range source.FlightTrack {
			sourcePoints = append(sourcePoints, point.XY())
		}
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUFAircraftReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUFAircraftReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildBUFAircraftReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildBUFAircraftReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildBUFAircraftReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func buildCnossosIndustryReceivers(sources []cnossosindustry.IndustrySource, options cnossosIndustryRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)

	for _, source := range sources {
		switch source.SourceType {
		case cnossosindustry.SourceTypePoint:
			sourcePoints = append(sourcePoints, source.Point)
		case cnossosindustry.SourceTypeArea:
			for _, ring := range source.AreaPolygon {
				sourcePoints = append(sourcePoints, ring...)
			}
		}
	}

	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosIndustryReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosIndustryReceivers", "generate receiver grid", err)
	}

	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosIndustryReceivers", "receiver grid is empty", nil)
	}

	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildCnossosIndustryReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildCnossosIndustryReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func inferGridShape(receivers []geo.PointReceiver) (int, int, error) {
	if len(receivers) == 0 {
		return 0, 0, errors.New("receivers are empty")
	}

	firstY := receivers[0].Point.Y
	width := 0

	for _, receiver := range receivers {
		if math.Abs(receiver.Point.Y-firstY) > 1e-9 {
			break
		}

		width++
	}

	if width <= 0 {
		return 0, 0, errors.New("invalid grid width")
	}

	if len(receivers)%width != 0 {
		return 0, 0, fmt.Errorf("receiver count %d is not divisible by inferred width %d", len(receivers), width)
	}

	return width, len(receivers) / width, nil
}

func persistReceiverTableOnly(
	resultsDir string,
	table results.ReceiverTable,
	summary map[string]any,
) (persistedRunOutputs, error) {
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "create results directory "+resultsDir, err)
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	if err := results.SaveReceiverTableJSON(receiverJSONPath, table); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table json", err)
	}

	if err := results.SaveReceiverTableCSV(receiverCSVPath, table); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table csv", err)
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		SummaryPath:      summaryPath,
	}, nil
}

func persistDummyRunOutputs(
	runDir string,
	runOutput engine.RunOutput,
	receivers []geo.PointReceiver,
	gridWidth int,
	gridHeight int,
	indicator string,
) (persistedRunOutputs, error) {
	resultsDir := filepath.Join(runDir, "results")

	err := os.MkdirAll(resultsDir, 0o755)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "create results directory "+resultsDir, err)
	}

	levelByReceiver := make(map[string]float64, len(runOutput.Results))
	for _, receiverResult := range runOutput.Results {
		levelByReceiver[receiverResult.ReceiverID] = receiverResult.LevelDB
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{indicator},
		Unit:           dummyResultUnit,
		Records:        make([]results.ReceiverRecord, 0, len(receivers)),
	}
	for _, receiver := range receivers {
		level, ok := levelByReceiver[receiver.ID]
		if !ok {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "missing result for receiver "+receiver.ID, nil)
		}

		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      receiver.ID,
			X:       receiver.Point.X,
			Y:       receiver.Point.Y,
			HeightM: receiver.HeightM,
			Values: map[string]float64{
				indicator: level,
			},
		})
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table csv", err)
	}

	summary := map[string]any{
		"run_id":             runOutput.RunID,
		"status":             runOutput.Status,
		"output_hash":        runOutput.OutputHash,
		"total_chunks":       runOutput.TotalChunks,
		"used_cached_chunks": runOutput.UsedCachedChunks,
		"source_count":       runOutput.Metadata["source_count"],
		"receiver_count":     len(receivers),
		"receiver_mode":      receiverModeAutoGrid,
	}

	if gridWidth <= 0 || gridHeight <= 0 {
		summary["receiver_mode"] = receiverModeCustom
		summaryPath := filepath.Join(resultsDir, "run-summary.json")
		if err := writeJSONFile(summaryPath, summary); err != nil {
			return persistedRunOutputs{}, err
		}

		return persistedRunOutputs{
			ReceiverJSONPath: receiverJSONPath,
			ReceiverCSVPath:  receiverCSVPath,
			SummaryPath:      summaryPath,
		}, nil
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     1,
		NoData:    -9999,
		Unit:      dummyResultUnit,
		BandNames: []string{indicator},
	})
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "build raster", err)
	}

	for receiverIndex, receiver := range receivers {
		level := levelByReceiver[receiver.ID]
		x := receiverIndex % gridWidth

		y := receiverIndex / gridWidth

		err := raster.Set(x, y, 0, level)
		if err != nil {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "set raster value", err)
		}
	}

	rasterBasePath := filepath.Join(resultsDir, strings.ToLower(indicator))

	rasterPersistence, err := results.SaveRaster(rasterBasePath, raster)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save raster", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   receiverJSONPath,
		ReceiverCSVPath:    receiverCSVPath,
		RasterMetadataPath: rasterPersistence.MetadataPath,
		RasterDataPath:     rasterPersistence.DataPath,
		SummaryPath:        summaryPath,
	}, nil
}

func persistCnossosRoadRunOutputs(
	runDir string,
	outputs []cnossosroad.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosRoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "hash cnossos outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosindustry.BuiltinModelVersion,
		"reporting_precision_db": cnossosindustry.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosroad.IndicatorLden, cnossosroad.IndicatorLnight, cnossosroad.IndicatorLday, cnossosroad.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosroad.IndicatorLden: output.Indicators.Lden, cnossosroad.IndicatorLnight: output.Indicators.Lnight, cnossosroad.IndicatorLday: output.Indicators.Lday, cnossosroad.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosroad.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "export cnossos road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistBUBRoadRunOutputs(
	runDir string,
	outputs []bubroad.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashBUBRoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUBRoadRunOutputs", "hash BUB road outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          bubroad.BuiltinModelVersion,
		"reporting_precision_db": bubroad.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{bubroad.IndicatorLden, bubroad.IndicatorLnight, bubroad.IndicatorLday, bubroad.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{bubroad.IndicatorLden: output.Indicators.Lden, bubroad.IndicatorLnight: output.Indicators.Lnight, bubroad.IndicatorLday: output.Indicators.Lday, bubroad.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := bubroad.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUBRoadRunOutputs", "export BUB road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistRLS19RoadRunOutputs(
	runDir string,
	outputs []rls19road.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashRLS19RoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistRLS19RoadRunOutputs", "hash RLS-19 road outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"data_pack_version":      rls19road.BuiltinDataPackVersion,
		"reporting_precision_db": rls19road.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{rls19road.IndicatorLrDay: output.Indicators.LrDay, rls19road.IndicatorLrNight: output.Indicators.LrNight}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := rls19road.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistRLS19RoadRunOutputs", "export RLS-19 road results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistSchall03RunOutputs(
	runDir string,
	outputs []schall03.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashSchall03Outputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistSchall03RunOutputs", "hash Schall 03 outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          schall03.BuiltinModelVersion,
		"reporting_precision_db": schall03.ReportingPrecisionDB,
		"band_model":             "octave-63Hz-8000Hz",
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{schall03.IndicatorLrDay: output.Indicators.LrDay, schall03.IndicatorLrNight: output.Indicators.LrNight}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := schall03.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistSchall03RunOutputs", "export Schall 03 results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistCnossosAircraftRunOutputs(
	runDir string,
	outputs []cnossosaircraft.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosAircraftOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosAircraftRunOutputs", "hash cnossos aircraft outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosaircraft.BuiltinModelVersion,
		"reporting_precision_db": cnossosaircraft.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosaircraft.IndicatorLden, cnossosaircraft.IndicatorLnight, cnossosaircraft.IndicatorLday, cnossosaircraft.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosaircraft.IndicatorLden: output.Indicators.Lden, cnossosaircraft.IndicatorLnight: output.Indicators.Lnight, cnossosaircraft.IndicatorLday: output.Indicators.Lday, cnossosaircraft.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosaircraft.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosAircraftRunOutputs", "export cnossos aircraft results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistCnossosRailRunOutputs(
	runDir string,
	outputs []cnossosrail.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosRailOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRailRunOutputs", "hash cnossos rail outputs", err)
	}

	summary := map[string]any{
		"run_id":                 filepath.Base(runDir),
		"status":                 project.RunStatusCompleted,
		"output_hash":            outputHash,
		"source_count":           sourceCount,
		"receiver_count":         len(outputs),
		"model_version":          cnossosrail.BuiltinModelVersion,
		"reporting_precision_db": cnossosrail.ReportingPrecisionDB,
		"receiver_mode":          receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosrail.IndicatorLden, cnossosrail.IndicatorLnight, cnossosrail.IndicatorLday, cnossosrail.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosrail.IndicatorLden: output.Indicators.Lden, cnossosrail.IndicatorLnight: output.Indicators.Lnight, cnossosrail.IndicatorLday: output.Indicators.Lday, cnossosrail.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosrail.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRailRunOutputs", "export cnossos rail results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistBUFAircraftRunOutputs(
	runDir string,
	outputs []bufaircraft.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashBUFAircraftOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUFAircraftRunOutputs", "hash buf aircraft outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
		"receiver_mode":  receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{bufaircraft.IndicatorLden, bufaircraft.IndicatorLnight, bufaircraft.IndicatorLday, bufaircraft.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{bufaircraft.IndicatorLden: output.Indicators.Lden, bufaircraft.IndicatorLnight: output.Indicators.Lnight, bufaircraft.IndicatorLday: output.Indicators.Lday, bufaircraft.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := bufaircraft.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBUFAircraftRunOutputs", "export buf aircraft results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func persistBEBExposureRunOutputs(
	runDir string,
	outputs []bebexposure.BuildingExposureOutput,
	summary bebexposure.Summary,
	sourceCount int,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	exported, err := bebexposure.ExportResultBundle(resultsDir, outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "export BEB exposure results", err)
	}

	outputHash, err := hashBEBExposureOutputs(outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "hash BEB exposure outputs", err)
	}

	runSummary := map[string]any{
		"run_id":                    filepath.Base(runDir),
		"status":                    project.RunStatusCompleted,
		"output_hash":               outputHash,
		"source_count":              sourceCount,
		"building_count":            len(outputs),
		"estimated_dwellings":       summary.EstimatedDwellings,
		"estimated_persons":         summary.EstimatedPersons,
		"affected_dwellings_lden":   summary.AffectedDwellingsLden,
		"affected_persons_lden":     summary.AffectedPersonsLden,
		"affected_dwellings_lnight": summary.AffectedDwellingsLnight,
		"affected_persons_lnight":   summary.AffectedPersonsLnight,
		"model_version":             bebexposure.BuiltinModelVersion,
		"reporting_precision_db":    bebexposure.ReportingPrecisionCount,
		"occupancy_mode":            summary.OccupancyMode,
		"facade_evaluation_mode":    summary.FacadeEvaluationMode,
		"upstream_mapping_standard": summary.UpstreamMappingStandard,
		"lden_bands":                summary.LdenBands,
		"lnight_bands":              summary.LnightBands,
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, runSummary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   exported.ReceiverJSONPath,
		ReceiverCSVPath:    exported.ReceiverCSVPath,
		RasterMetadataPath: exported.RasterMetaPath,
		RasterDataPath:     exported.RasterDataPath,
		SummaryPath:        summaryPath,
	}, outputHash, nowUTC(), nil
}

func persistCnossosIndustryRunOutputs(
	runDir string,
	outputs []cnossosindustry.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashCnossosIndustryOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosIndustryRunOutputs", "hash cnossos industry outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
		"receiver_mode":  receiverMode,
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{IndicatorOrder: []string{cnossosindustry.IndicatorLden, cnossosindustry.IndicatorLnight, cnossosindustry.IndicatorLday, cnossosindustry.IndicatorLevening}, Unit: "dB", Records: make([]results.ReceiverRecord, 0, len(outputs))}
		for _, output := range outputs {
			table.Records = append(table.Records, results.ReceiverRecord{ID: output.Receiver.ID, X: output.Receiver.Point.X, Y: output.Receiver.Point.Y, HeightM: output.Receiver.HeightM, Values: map[string]float64{cnossosindustry.IndicatorLden: output.Indicators.Lden, cnossosindustry.IndicatorLnight: output.Indicators.Lnight, cnossosindustry.IndicatorLday: output.Indicators.Lday, cnossosindustry.IndicatorLevening: output.Indicators.Levening}})
		}
		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)
		return persisted, outputHash, nowUTC(), err
	}

	exported, err := cnossosindustry.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosIndustryRunOutputs", "export cnossos industry results", err)
	}

	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{ReceiverJSONPath: exported.ReceiverJSONPath, ReceiverCSVPath: exported.ReceiverCSVPath, RasterMetadataPath: exported.RasterMetaPath, RasterDataPath: exported.RasterDataPath, SummaryPath: summaryPath}, outputHash, nowUTC(), nil
}

func hashCnossosRoadOutputs(outputs []cnossosroad.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators cnossosroad.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosRailOutputs(outputs []cnossosrail.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators cnossosrail.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBUBRoadOutputs(outputs []bubroad.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                     `json:"receiver_id"`
		Indicators bubroad.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashRLS19RoadOutputs(outputs []rls19road.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                       `json:"receiver_id"`
		Indicators rls19road.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashSchall03Outputs(outputs []schall03.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                      `json:"receiver_id"`
		Indicators schall03.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosAircraftOutputs(outputs []cnossosaircraft.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                             `json:"receiver_id"`
		Indicators cnossosaircraft.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBUFAircraftOutputs(outputs []bufaircraft.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators bufaircraft.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashBEBExposureOutputs(outputs []bebexposure.BuildingExposureOutput, summary bebexposure.Summary) (string, error) {
	type record struct {
		BuildingID string                         `json:"building_id"`
		Indicators bebexposure.BuildingIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			BuildingID: output.Building.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(struct {
		Buildings []record            `json:"buildings"`
		Summary   bebexposure.Summary `json:"summary"`
	}{
		Buildings: records,
		Summary:   summary,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func hashCnossosIndustryOutputs(outputs []cnossosindustry.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                             `json:"receiver_id"`
		Indicators cnossosindustry.ReceiverIndicators `json:"indicators"`
	}

	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func buildRunArtifacts(projectRoot string, runID string, persisted persistedRunOutputs) []project.ArtifactRef {
	now := nowUTC()
	artifacts := make([]project.ArtifactRef, 0, 5)
	if persisted.ReceiverJSONPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-json", runID), RunID: runID, Kind: "run.result.receiver_table_json", Path: relativePath(projectRoot, persisted.ReceiverJSONPath), CreatedAt: now})
	}
	if persisted.ReceiverCSVPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-csv", runID), RunID: runID, Kind: "run.result.receiver_table_csv", Path: relativePath(projectRoot, persisted.ReceiverCSVPath), CreatedAt: now})
	}
	if persisted.RasterMetadataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-meta", runID), RunID: runID, Kind: "run.result.raster_metadata", Path: relativePath(projectRoot, persisted.RasterMetadataPath), CreatedAt: now})
	}
	if persisted.RasterDataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-data", runID), RunID: runID, Kind: "run.result.raster_binary", Path: relativePath(projectRoot, persisted.RasterDataPath), CreatedAt: now})
	}
	if persisted.SummaryPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-summary", runID), RunID: runID, Kind: "run.result.summary", Path: relativePath(projectRoot, persisted.SummaryPath), CreatedAt: now})
	}

	return artifacts
}

func finalizeRunFailure(store projectfs.Store, run project.Run, logLines []string, runErr error) error {
	finishedAt := nowUTC()

	logLines = append(logLines, finishedAt.Format(time.RFC3339)+" run failed")

	err := finalizeRun(store, run, project.RunStatusFailed, finishedAt, logLines, nil)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRunFailure", "finalize failed run", errors.Join(runErr, err))
	}

	return runErr
}

func finalizeRun(
	store projectfs.Store,
	run project.Run,
	status string,
	finishedAt time.Time,
	logLines []string,
	artifacts []project.ArtifactRef,
) error {
	if finishedAt.IsZero() {
		finishedAt = nowUTC()
	}

	proj, err := store.Load()
	if err != nil {
		return err
	}

	foundRun := false

	for i := range proj.Runs {
		if proj.Runs[i].ID != run.ID {
			continue
		}

		proj.Runs[i].Status = status
		proj.Runs[i].FinishedAt = finishedAt
		foundRun = true

		break
	}

	if !foundRun {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", fmt.Sprintf("run %s not found in project manifest", run.ID), nil)
	}

	for _, artifact := range artifacts {
		proj.Artifacts = upsertArtifact(proj.Artifacts, artifact)
	}

	err = store.Save(proj)
	if err != nil {
		return err
	}

	if len(logLines) == 0 {
		logLines = []string{fmt.Sprintf("%s run finalized with status=%s", finishedAt.Format(time.RFC3339), status)}
	}

	logContent := strings.Join(logLines, "\n") + "\n"

	logPath := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))

	err = os.WriteFile(logPath, []byte(logContent), 0o644)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", "write run log "+logPath, err)
	}

	return nil
}
