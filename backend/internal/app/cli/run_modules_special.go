package cli

import (
	"fmt"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// runRLS19RoadModule is the receiver shape plus what RLS-19 alone needs:
// barriers and buildings are separate feature kinds it propagates against, the
// count of sources carrying feature overrides is part of its run summary, and
// terrain elevation is applied at the receiver grid centre.
func runRLS19RoadModule(input runModuleInput) (runModuleResult, error) {
	options, err := parseRLS19RoadRunOptions(input.params)
	if err != nil {
		return runModuleResult{}, beforeRunError{err: err}
	}

	roadSources, sourceOverrideCount, err := extractRLS19RoadSources(input.model, options, input.standard.SupportedSourceTypes)
	if err != nil {
		input.log.addf("failed to extract RLS-19 road sources: %v", err)

		return runModuleResult{}, err
	}

	barriers, err := extractRLS19Barriers(input.model)
	if err != nil {
		input.log.addf("failed to extract RLS-19 barriers: %v", err)

		return runModuleResult{}, err
	}

	buildings, err := extractRLS19Buildings(input.model)
	if err != nil {
		input.log.addf("failed to extract RLS-19 buildings: %v", err)

		return runModuleResult{}, err
	}

	receivers, gridWidth, gridHeight, err := resolveReceiverSet(input.receiverMode, input.model, func() ([]geo.PointReceiver, int, int, error) {
		return buildRLS19RoadReceivers(roadSources, options)
	})
	if err != nil {
		input.log.addf("failed to build receivers: %v", err)

		return runModuleResult{}, err
	}

	input.log.addf("rls19_road_sources=%d", len(roadSources))
	input.log.addf("rls19_sources_with_feature_overrides=%d", sourceOverrideCount)
	input.log.addf("rls19_barriers=%d", len(barriers))
	input.log.addf("rls19_buildings=%d", len(buildings))
	input.log.addReceiverCount(input.receiverMode, len(receivers), gridWidth, gridHeight)

	propagationConfig := options.PropagationConfig()
	propagationConfig.Buildings = buildings

	if input.terrain != nil && len(receivers) > 0 {
		centerX, centerY := receiverGridCenter(receivers)
		propagationConfig.ReceiverTerrainZ = terrainElevationAt(input.terrain, centerX, centerY)
	}

	receiverOutputs, err := rls19road.ComputeReceiverOutputs(receivers, roadSources, barriers, propagationConfig)
	if err != nil {
		input.log.addf("rls19 compute failed: %v", err)

		return runModuleResult{}, fmt.Errorf("compute RLS-19 receiver outputs: %w", err)
	}

	persisted, outputHash, finishedAt, err := persistRLS19RoadRunOutputs(
		input.runDir, receiverOutputs, gridWidth, gridHeight, len(roadSources), sourceOverrideCount, input.receiverMode, input.standard.EvidenceTier,
	)
	if err != nil {
		input.log.addf("failed to persist outputs: %v", err)

		return runModuleResult{}, err
	}

	return runModuleResult{
		persisted:  persisted,
		outputHash: outputHash,
		finishedAt: finishedAt,
	}, nil
}

// runSchall03Module drives the normative Schall 03 chain, which resolves which
// engine it runs and reports its own log lines. The resolved engine is only
// knowable here, so the manifest is completed rather than guessed at
// CreateRun time — and before anything is persisted, so a failure to record it
// leaves no results behind.
func runSchall03Module(input runModuleInput) (runModuleResult, error) {
	options, err := parseSchall03RunOptions(input.params)
	if err != nil {
		return runModuleResult{}, beforeRunError{err: err}
	}

	result, computeErr := computeSchall03Run(input.model, options, input.standard.SupportedSourceTypes, input.receiverMode)
	input.log.addLines(result.LogLines)

	if computeErr != nil {
		input.log.addf("schall03 run failed: %v", computeErr)

		return runModuleResult{}, computeErr
	}

	err = input.mergeProvenance(schall03.ResolvedProvenanceMetadata(result.Engine))
	if err != nil {
		input.log.addf("failed to record resolved engine in provenance: %v", err)

		return runModuleResult{}, err
	}

	persisted, outputHash, finishedAt, err := persistSchall03RunOutputs(
		input.runDir,
		result.Outputs,
		result.GridWidth,
		result.GridHeight,
		result.SourceCount,
		input.receiverMode,
		result.Engine,
		input.standard.EvidenceTier,
	)
	if err != nil {
		input.log.addf("failed to persist outputs: %v", err)

		return runModuleResult{}, err
	}

	return runModuleResult{
		persisted:  persisted,
		outputHash: outputHash,
		finishedAt: finishedAt,
	}, nil
}

// runBEBExposureModule aggregates exposure per building rather than computing
// receiver levels, so its "receivers" are buildings and its sources come from
// whichever mapping standard it sits downstream of.
func runBEBExposureModule(input runModuleInput) (runModuleResult, error) {
	if input.receiverMode == receiverModeCustom {
		return runModuleResult{}, beforeRunError{err: domainerrors.New(
			domainerrors.KindUserInput, "cli.run", "custom receiver mode is not supported for building exposure runs", nil,
		)}
	}

	options, err := parseBEBExposureRunOptions(input.params)
	if err != nil {
		return runModuleResult{}, beforeRunError{err: err}
	}

	buildings, err := extractBEBBuildings(input.model, options)
	if err != nil {
		input.log.addf("failed to extract BEB buildings: %v", err)

		return runModuleResult{}, err
	}

	outputs, summary, sourceCount, err := computeBEBExposure(input, options, buildings)
	if err != nil {
		return runModuleResult{}, err
	}

	persisted, outputHash, finishedAt, err := persistBEBExposureRunOutputs(input.runDir, outputs, summary, sourceCount, input.standard.EvidenceTier)
	if err != nil {
		input.log.addf("failed to persist outputs: %v", err)

		return runModuleResult{}, err
	}

	return runModuleResult{
		persisted:  persisted,
		outputHash: outputHash,
		finishedAt: finishedAt,
	}, nil
}

// computeBEBExposure runs the upstream mapping standard the BEB options name
// and aggregates its sources onto the buildings.
func computeBEBExposure(
	input runModuleInput,
	options bebExposureRunOptions,
	buildings []bebexposure.BuildingUnit,
) ([]bebexposure.BuildingExposureOutput, bebexposure.Summary, int, error) {
	switch options.UpstreamMappingStandard {
	case bebexposure.UpstreamStandardBUBRoad:
		roadSources, err := extractBUBRoadSources(input.model, options.BUBRoadOptions(), []string{modelgeojson.SourceTypeLine})
		if err != nil {
			input.log.addf("failed to extract BEB upstream road sources: %v", err)

			return nil, bebexposure.Summary{}, 0, err
		}

		logBEBUpstream(input, options, len(roadSources), len(buildings))

		outputs, summary, err := bebexposure.ComputeOutputs(buildings, roadSources, options.ExposureConfig(), options.BUBRoadOptions().PropagationConfig(), options.FacadeReceiverHeightM)
		if err != nil {
			input.log.addf("beb exposure compute failed: %v", err)

			return nil, bebexposure.Summary{}, 0, fmt.Errorf("compute BEB exposure outputs: %w", err)
		}

		return outputs, summary, len(roadSources), nil
	case bebexposure.UpstreamStandardBUFAircraft:
		aircraftSources, err := extractBUFAircraftSources(input.model, options.BUFAircraftOptions(), []string{modelgeojson.SourceTypeLine})
		if err != nil {
			input.log.addf("failed to extract BEB upstream aircraft sources: %v", err)

			return nil, bebexposure.Summary{}, 0, err
		}

		logBEBUpstream(input, options, len(aircraftSources), len(buildings))

		outputs, summary, err := bebexposure.ComputeOutputsFromAircraft(buildings, aircraftSources, options.ExposureConfig(), options.BUFAircraftOptions().PropagationConfig(), options.FacadeReceiverHeightM)
		if err != nil {
			input.log.addf("beb exposure compute failed: %v", err)

			return nil, bebexposure.Summary{}, 0, fmt.Errorf("compute BEB exposure outputs: %w", err)
		}

		return outputs, summary, len(aircraftSources), nil
	default:
		err := domainerrors.New(
			domainerrors.KindUserInput, "cli.run",
			"unsupported upstream mapping standard "+options.UpstreamMappingStandard, nil,
		)
		input.log.addf("beb exposure compute failed: %v", err)

		return nil, bebexposure.Summary{}, 0, err
	}
}

func logBEBUpstream(input runModuleInput, options bebExposureRunOptions, sourceCount int, buildingCount int) {
	input.log.addf("beb_upstream_standard=%s", options.UpstreamMappingStandard)
	input.log.addf("beb_upstream_sources=%d", sourceCount)
	input.log.addf("beb_buildings=%d", buildingCount)
}
