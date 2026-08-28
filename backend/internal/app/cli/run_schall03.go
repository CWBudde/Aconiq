package cli

import (
	"fmt"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
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
		return computeSchall03Normative(result, model, options, supportedSourceTypes, receiverMode)
	}

	return computeSchall03Preview(result, model, options, supportedSourceTypes, receiverMode)
}

func computeSchall03Normative(
	result schall03RunResult,
	model modelgeojson.Model,
	options schall03RunOptions,
	supportedSourceTypes []string,
	receiverMode string,
) (schall03RunResult, error) {
	scene, err := extractSchall03NormativeScene(model, supportedSourceTypes)
	if err != nil {
		return schall03RunResult{}, err
	}

	receivers, gridWidth, gridHeight, err := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
		return buildSchall03NormativeReceivers(scene.Segments, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(scene.Segments)
	result.GridWidth = gridWidth
	result.GridHeight = gridHeight

	result.logf("schall03_segments=%d walls=%d barriers=%d", len(scene.Segments), len(scene.Walls), len(scene.Barriers))
	result.logReceivers(receiverMode, len(receivers), gridWidth, gridHeight)

	result.Outputs, err = schall03.ComputeNormativeReceiverOutputs(receivers, scene.Segments, scene.Walls, scene.Barriers)
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

	receivers, gridWidth, gridHeight, err := resolveReceiverSet(receiverMode, model, func() ([]geo.PointReceiver, int, int, error) {
		return buildSchall03Receivers(railSources, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(railSources)
	result.GridWidth = gridWidth
	result.GridHeight = gridHeight

	result.logf("schall03_sources=%d", len(railSources))
	result.logReceivers(receiverMode, len(receivers), gridWidth, gridHeight)

	result.Outputs, err = schall03.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
	if err != nil {
		return schall03RunResult{}, fmt.Errorf("compute Schall 03 preview receiver levels: %w", err)
	}

	return result, nil
}

// buildSchall03NormativeReceivers derives the auto grid from the normative
// track centerlines.
func buildSchall03NormativeReceivers(segments []schall03.TrackSegment, options schall03RunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(segments)*2)
	for _, segment := range segments {
		sourcePoints = append(sourcePoints, segment.TrackCenterline...)
	}

	return buildReceiversFromPoints("cli.buildSchall03NormativeReceivers", sourcePoints, options.GridResolutionM, options.GridPaddingM, options.ReceiverHeightM)
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
