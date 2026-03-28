package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
)

type runCommandRequest struct {
	scenarioID      string
	standardID      string
	standardVersion string
	standardProfile string
	modelPath       string
	receiverMode    string
	rawParams       []string
	inputPaths      []string
}

//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx // This preserves the existing per-standard run orchestration while keeping newRunCommand thin.
func executeRunCommand(cmd *cobra.Command, req runCommandRequest) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.run", "command state unavailable", nil)
	}

	params, err := parseKeyValueFlags(req.rawParams)
	if err != nil {
		return err
	}

	err = validateReceiverMode(req.receiverMode)
	if err != nil {
		return err
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.run", "initialize standards registry", err)
	}

	resolvedStandard, err := registry.Resolve(req.standardID, req.standardVersion, req.standardProfile)
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

	resolvedModelPath := resolvePath(store.Root(), req.modelPath)
	relModelPath := relativePath(store.Root(), resolvedModelPath)
	combinedInputs := mergeInputPaths(append([]string{relModelPath}, req.inputPaths...))

	run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: req.scenarioID,
		Standard: project.StandardRef{
			Context: resolvedStandard.Context,
			ID:      resolvedStandard.StandardID,
			Version: resolvedStandard.Version,
			Profile: resolvedStandard.Profile,
		},
		ReceiverMode:  req.receiverMode,
		ReceiverSetID: receiverSetID(req.receiverMode),
		Parameters:    resolvedParams,
		Metadata:      buildRunProvenanceMetadata(resolvedStandard.StandardID, resolvedParams, req.receiverMode),
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
		fmt.Sprintf("%s receiver_mode=%s", run.StartedAt.Format(time.RFC3339), req.receiverMode),
	}

	model, err := loadValidatedModel(resolvedModelPath, proj.CRS, relModelPath)
	if err != nil {
		logLines = append(logLines, fmt.Sprintf("%s failed to load model: %v", nowUTC().Format(time.RFC3339), err))
		return finalizeRunFailure(store, run, logLines, err)
	}

	var terrainModel terrain.Model

	if terrainArtifactPath := findArtifactPath(proj, "artifact-terrain"); terrainArtifactPath != "" {
		tm, terrainErr := terrain.Load(filepath.Join(store.Root(), terrainArtifactPath))
		if terrainErr != nil {
			state.Logger.Warn("terrain DTM load failed, continuing without terrain", "error", terrainErr)
			logLines = append(logLines, fmt.Sprintf("%s terrain load warning: %v", nowUTC().Format(time.RFC3339), terrainErr))
		} else {
			terrainModel = tm

			logLines = append(logLines, fmt.Sprintf("%s terrain loaded from %s", nowUTC().Format(time.RFC3339), terrainArtifactPath))
		}
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildDummyReceivers(sources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(sources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildCnossosRoadReceivers(roadSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(roadSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := cnossosroad.ComputeReceiverOutputs(receivers, roadSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s cnossos compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistCnossosRoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildCnossosRailReceivers(railSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(railSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s rail_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := cnossosrail.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s cnossos rail compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistCnossosRailRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildBUBRoadReceivers(roadSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(roadSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s bub_road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := bubroad.ComputeReceiverOutputs(receivers, roadSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s bub road compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistBUBRoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
		if err != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
			return finalizeRunFailure(store, run, logLines, err)
		}
	case rls19road.StandardID:
		options, parseErr := parseRLS19RoadRunOptions(resolvedParams)
		if parseErr != nil {
			return parseErr
		}

		roadSources, sourceOverrideCount, extractErr := extractRLS19RoadSources(model, options, resolvedStandard.SupportedSourceTypes)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
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
			fmt.Sprintf("%s rls19_sources_with_feature_overrides=%d", nowUTC().Format(time.RFC3339), sourceOverrideCount),
			fmt.Sprintf("%s rls19_barriers=%d", nowUTC().Format(time.RFC3339), len(barriers)),
			fmt.Sprintf("%s rls19_buildings=%d", nowUTC().Format(time.RFC3339), len(buildings)),
		)
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		propagationConfig := options.PropagationConfig()
		propagationConfig.Buildings = buildings

		if terrainModel != nil && len(receivers) > 0 {
			centerX, centerY := receiverGridCenter(receivers)
			propagationConfig.ReceiverTerrainZ = terrainElevationAt(terrainModel, centerX, centerY)
		}

		receiverOutputs, computeErr := rls19road.ComputeReceiverOutputs(receivers, roadSources, barriers, propagationConfig)
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s rls19 compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistRLS19RoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, sourceOverrideCount, req.receiverMode)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildSchall03Receivers(railSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(railSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s schall03_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := schall03.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s schall03 compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistSchall03RunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
		if err != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
			return finalizeRunFailure(store, run, logLines, err)
		}
	case bebexposure.StandardID:
		if req.receiverMode == receiverModeCustom {
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildBUFAircraftReceivers(aircraftSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(aircraftSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s buf_aircraft_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := bufaircraft.ComputeReceiverOutputs(receivers, aircraftSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s buf aircraft compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistBUFAircraftRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildCnossosAircraftReceivers(aircraftSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(aircraftSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s aircraft_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := cnossosaircraft.ComputeReceiverOutputs(receivers, aircraftSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s cnossos aircraft compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistCnossosAircraftRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
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

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildCnossosIndustryReceivers(industrySources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(industrySources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s industry_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := cnossosindustry.ComputeReceiverOutputs(receivers, industrySources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s cnossos industry compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistCnossosIndustryRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
		if err != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
			return finalizeRunFailure(store, run, logLines, err)
		}
	case iso9613.StandardID:
		options, parseErr := parseISO9613RunOptions(resolvedParams)
		if parseErr != nil {
			return parseErr
		}

		pointSources, extractErr := extractISO9613Sources(model, options, resolvedStandard.SupportedSourceTypes)
		if extractErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to extract ISO 9613 point sources: %v", nowUTC().Format(time.RFC3339), extractErr))
			return finalizeRunFailure(store, run, logLines, extractErr)
		}

		receivers, gridWidth, gridHeight, receiverErr := resolveReceiverSet(req.receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
			return buildISO9613Receivers(pointSources, options)
		})
		if receiverErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
			return finalizeRunFailure(store, run, logLines, receiverErr)
		}

		sourceCount = len(pointSources)
		receiverCount = len(receivers)

		logLines = append(logLines, fmt.Sprintf("%s iso9613_sources=%d", nowUTC().Format(time.RFC3339), sourceCount))
		if req.receiverMode == receiverModeCustom {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d set=%s", nowUTC().Format(time.RFC3339), receiverCount, explicitReceiverSetID))
		} else {
			logLines = append(logLines, fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight))
		}

		receiverOutputs, computeErr := iso9613.ComputeReceiverOutputs(receivers, pointSources, options.PropagationConfig())
		if computeErr != nil {
			logLines = append(logLines, fmt.Sprintf("%s iso9613 compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
			return finalizeRunFailure(store, run, logLines, computeErr)
		}

		persisted, outputHash, finishedAt, err = persistISO9613RunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount, req.receiverMode)
		if err != nil {
			logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
			return finalizeRunFailure(store, run, logLines, err)
		}
	default:
		runErr := domainerrors.New(
			domainerrors.KindUserInput,
			"cli.run",
			fmt.Sprintf("standard %q is registered but not wired in run pipeline yet", resolvedStandard.StandardID),
			nil,
		)

		logLines = append(logLines, fmt.Sprintf("%s run wiring missing: %v", nowUTC().Format(time.RFC3339), runErr))

		return finalizeRunFailure(store, run, logLines, runErr)
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

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":          "run",
			"run_id":           run.ID,
			"status":           string(project.RunStatusCompleted),
			"scenario":         run.ScenarioID,
			"standard":         run.Standard.ID,
			"standard_version": run.Standard.Version,
			"standard_profile": run.Standard.Profile,
			"provenance_path":  provenance.ManifestPath,
			"results_path":     relativePath(store.Root(), filepath.Join(runDir, "results")),
		})
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Completed run %s (%s)\n", run.ID, project.RunStatusCompleted)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provenance: %s\n", provenance.ManifestPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Results: %s\n", relativePath(store.Root(), filepath.Join(runDir, "results")))

	return nil
}
