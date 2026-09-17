package cli

import (
	"fmt"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/numeric"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// schall03RunResult carries everything the run pipeline needs from a Schall 03
// computation, whichever engine produced it.
type schall03RunResult struct {
	Engine      string
	Outputs     []schall03.ReceiverOutput
	SourceCount int
	GridWidth   int
	GridHeight  int
	LogLines    []string
}

// resolveSchall03Engine turns the configured engine setting into the chain that
// will actually run.
//
// `auto` never falls back on its own: a model without normative track data has
// no Zugart and no Fz composition, so there is nothing for Beiblatt 1/2 to
// compute from, and the alternative — quietly running invented spectra under a
// normative standard ID — is the failure mode this resolution exists to
// prevent. Reaching the preview chain takes an explicit opt-in.
func resolveSchall03Engine(configured string, model modelgeojson.Model) (string, error) {
	switch configured {
	case schall03.EngineNormative:
		if !modelHasSchall03NormativeTracks(model) {
			return "", schall03MissingNormativeInputsError(schall03.EngineNormative)
		}

		return schall03.EngineNormative, nil
	case schall03.EnginePreview:
		return schall03.EnginePreview, nil
	case schall03.EngineAuto:
		if modelHasSchall03NormativeTracks(model) {
			return schall03.EngineNormative, nil
		}

		return "", schall03MissingNormativeInputsError(schall03.EngineAuto)
	default:
		return "", domainerrors.New(
			domainerrors.KindUserInput,
			"cli.resolveSchall03Engine",
			fmt.Sprintf("invalid schall03_engine=%q, expected one of %s, %s, %s", configured, schall03.EngineAuto, schall03.EngineNormative, schall03.EnginePreview),
			nil,
		)
	}
}

func schall03MissingNormativeInputsError(configured string) error {
	return domainerrors.New(
		domainerrors.KindUserInput,
		"cli.resolveSchall03Engine",
		fmt.Sprintf(
			"schall03_engine=%s requires normative track data: no source feature carries %q. "+
				"Add it (see docs/geojson-schema-v1.md), or opt into the non-normative preview data pack with --param schall03_engine=%s",
			configured, propSchall03Operations, schall03.EnginePreview,
		),
		nil,
	)
}

// computeSchall03Run resolves the engine, extracts its inputs from the model
// and computes receiver levels.
func computeSchall03Run(
	model modelgeojson.Model,
	terrainModel terrain.Model,
	options schall03RunOptions,
	supportedSourceTypes []string,
	receiverMode string,
) (schall03RunResult, error) {
	engine, err := resolveSchall03Engine(options.Engine, model)
	if err != nil {
		return schall03RunResult{}, err
	}

	result := schall03RunResult{Engine: engine}
	result.logf("schall03_engine=%s", engine)

	if engine == schall03.EnginePreview {
		result.logf(
			"WARNING schall03_engine=preview: levels come from %s, whose spectra are invented placeholders. Output is not a Schall 03 result.",
			schall03.BuiltinDataPackVersion,
		)
	}

	if engine == schall03.EngineNormative {
		return computeSchall03Normative(result, model, terrainModel, options, supportedSourceTypes, receiverMode)
	}

	return computeSchall03Preview(result, model, options, supportedSourceTypes, receiverMode)
}

func computeSchall03Normative(
	result schall03RunResult,
	model modelgeojson.Model,
	terrainModel terrain.Model,
	options schall03RunOptions,
	supportedSourceTypes []string,
	receiverMode string,
) (schall03RunResult, error) {
	scene, err := extractSchall03NormativeScene(model, supportedSourceTypes)
	if err != nil {
		return schall03RunResult{}, err
	}

	receivers, gridWidth, gridHeight, calcArea, err := resolveGridReceivers(model, receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, int, int, error) {
		return buildSchall03NormativeReceivers(scene.Segments, calcArea, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(scene.Segments)
	result.GridWidth = gridWidth
	result.GridHeight = gridHeight

	// Barrier panels, not obstacles: one building footprint contributes one
	// panel per outer-ring edge, so this count is no longer the number of
	// barrier features in the model.
	result.logf("schall03_segments=%d walls=%d barrier_panels=%d", len(scene.Segments), len(scene.Walls), len(scene.Barriers))
	result.logReceivers(receiverMode, len(receivers), gridWidth, gridHeight)
	result.logGridExtent(receiverMode, calcArea)

	receiverInputs, sampled := schall03ReceiverInputs(receivers, terrainModel)
	if terrainModel != nil {
		result.logf("schall03_receiver_terrain_samples=%d/%d", sampled, len(receivers))
	}

	result.Outputs, err = schall03.ComputeNormativeReceiverOutputs(receiverInputs, scene.Segments, scene.Walls, scene.Barriers)
	if err != nil {
		return schall03RunResult{}, fmt.Errorf("compute Schall 03 normative receiver levels: %w", err)
	}

	return result, nil
}

func computeSchall03Preview(
	result schall03RunResult,
	model modelgeojson.Model,
	options schall03RunOptions,
	supportedSourceTypes []string,
	receiverMode string,
) (schall03RunResult, error) {
	railSources, err := extractSchall03Sources(model, options, supportedSourceTypes)
	if err != nil {
		return schall03RunResult{}, err
	}

	receivers, gridWidth, gridHeight, calcArea, err := resolveGridReceivers(model, receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, int, int, error) {
		return buildSchall03Receivers(railSources, calcArea, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(railSources)
	result.GridWidth = gridWidth
	result.GridHeight = gridHeight

	result.logf("schall03_sources=%d", len(railSources))
	result.logReceivers(receiverMode, len(receivers), gridWidth, gridHeight)
	result.logGridExtent(receiverMode, calcArea)

	result.Outputs, err = schall03.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
	if err != nil {
		return schall03RunResult{}, fmt.Errorf("compute Schall 03 preview receiver levels: %w", err)
	}

	return result, nil
}

// schall03ReceiverInputs pairs every receiver with the absolute elevation of
// the ground it stands on, which the Anlage-2 chain uses as the datum for the
// whole propagation path: a track's elevation_m is an absolute Z, so without
// it h_g would be measured from sea level while h_r was measured from the
// ground, and Gl. 14's h_m would come out several hundred metres too high.
//
// A run with no terrain keeps every receiver at Z = 0, which is the reading a
// scene that never mentions terrain already had — there elevation_m is itself
// a height above ground, and the two agree.
//
// A receiver the terrain does not cover is a miss, not an elevation of 0; that
// is the trap terrainInComputeCRS's doc comment names, and 0 is the worst
// possible guess here, because it would drop one receiver of a hillside grid
// back to sea level while its neighbours stayed on the hill. Such receivers
// inherit the mean of the ones the DTM does reach, and the run log records how
// many were actually sampled. Second return value is that count.
func schall03ReceiverInputs(receivers []geo.PointReceiver, terrainModel terrain.Model) ([]schall03.ReceiverInput, int) {
	groundZ := make([]float64, len(receivers))
	sampled := 0

	if terrainModel != nil {
		var hits numeric.CompensatedSum

		covered := make([]bool, len(receivers))

		for i, receiver := range receivers {
			elevation, ok := terrainModel.ElevationAt(receiver.Point.X, receiver.Point.Y)
			if !ok {
				continue
			}

			groundZ[i] = elevation
			covered[i] = true
			sampled++

			hits.Add(elevation)
		}

		if sampled > 0 && sampled < len(receivers) {
			fallback := hits.Sum() / float64(sampled)

			for i := range groundZ {
				if !covered[i] {
					groundZ[i] = fallback
				}
			}
		}
	}

	inputs := make([]schall03.ReceiverInput, len(receivers))
	for i, receiver := range receivers {
		inputs[i] = schall03.ReceiverInput{
			ID:       receiver.ID,
			Point:    receiver.Point,
			HeightM:  receiver.HeightM,
			TerrainZ: groundZ[i],
		}
	}

	return inputs, sampled
}

// buildSchall03NormativeReceivers derives the auto grid from the normative
// track centerlines.
func buildSchall03NormativeReceivers(segments []schall03.TrackSegment, calcArea *geo.BBox, options schall03RunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(segments)*2)
	for _, segment := range segments {
		sourcePoints = append(sourcePoints, segment.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildSchall03NormativeReceivers", sourcePoints, calcArea, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
}

func (r *schall03RunResult) logf(format string, args ...any) {
	r.LogLines = append(r.LogLines, nowUTC().Format(time.RFC3339)+" "+fmt.Sprintf(format, args...))
}

func (r *schall03RunResult) logReceivers(receiverMode string, receiverCount, gridWidth, gridHeight int) {
	if receiverMode == receiverModeCustom {
		r.logf("receivers=%d set=%s", receiverCount, explicitReceiverSetID)

		return
	}

	r.logf("receivers=%d grid=%dx%d", receiverCount, gridWidth, gridHeight)
}

// logGridExtent mirrors runLog.addGridExtent for the Schall 03 chain, which
// carries its own log lines.
func (r *schall03RunResult) logGridExtent(receiverMode string, calcArea *geo.BBox) {
	if receiverMode == receiverModeCustom {
		return
	}

	r.logf("grid_extent=%s", gridExtentLabel(calcArea))
}
