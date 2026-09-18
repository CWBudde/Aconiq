package cli

import (
	"fmt"
	"math"
	"slices"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/numeric"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// schall03RunResult carries everything the run pipeline needs from a Schall 03
// computation, whichever engine produced it.
type schall03RunResult struct {
	Engine      string
	Outputs     []schall03.ReceiverOutput
	SourceCount int
	Layout      results.GridLayout
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

	receivers, layout, calcArea, err := resolveGridReceivers(model, receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, results.GridLayout, error) {
		return buildSchall03NormativeReceivers(scene.Segments, calcArea, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(scene.Segments)
	result.Layout = layout

	// Barrier panels, not obstacles: one building footprint contributes one
	// panel per outer-ring edge, so this count is no longer the number of
	// barrier features in the model.
	result.logf("schall03_segments=%d walls=%d barrier_panels=%d", len(scene.Segments), len(scene.Walls), len(scene.Barriers))
	result.logReceivers(receiverMode, len(receivers), layout.Width, layout.Height)
	result.logGridExtent(receiverMode, calcArea)

	receiverInputs, sampled, err := schall03ReceiverInputs(receivers, terrainModel)
	if err != nil {
		return schall03RunResult{}, err
	}

	if terrainModel != nil {
		result.logf("schall03_receiver_terrain_samples=%d/%d", sampled, len(receivers))
	}

	result.warnOnUngroundedElevation(terrainModel, scene.Segments)

	// The DTM goes into the scene, not just into the receivers' datum: the
	// propagation chain samples it along every subsegment→receiver path for
	// Gl. 15's h_m.
	result.Outputs, err = schall03.ComputeNormativeReceiverOutputsForScene(receiverInputs, schall03.NormativeScene{
		Segments: scene.Segments,
		Walls:    scene.Walls,
		Barriers: scene.Barriers,
		Terrain:  terrainModel,
	})
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

	receivers, layout, calcArea, err := resolveGridReceivers(model, receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, results.GridLayout, error) {
		return buildSchall03Receivers(railSources, calcArea, options)
	})
	if err != nil {
		return schall03RunResult{}, err
	}

	result.SourceCount = len(railSources)
	result.Layout = layout

	result.logf("schall03_sources=%d", len(railSources))
	result.logReceivers(receiverMode, len(receivers), layout.Width, layout.Height)
	result.logGridExtent(receiverMode, calcArea)

	result.Outputs, err = schall03.ComputeReceiverOutputs(receivers, railSources, options.PropagationConfig())
	if err != nil {
		return schall03RunResult{}, fmt.Errorf("compute Schall 03 preview receiver levels: %w", err)
	}

	return result, nil
}

// warnOnUngroundedElevation says out loud which of the two readings of
// elevation_m the run just used, and what else the run gave up with the DTM.
//
// With no DTM every receiver's ground sits at Z = 0, so a track's elevation_m
// is read as a height above that ground — the right reading for a hand-written
// scene where it means a bridge deck or an embankment crest, and that scene
// must keep working, so this is a warning and not a refusal.
//
// It is the wrong reading for an imported project, where elevation_m is an
// absolute Z: the SoundPLAN import writes the rail's ZTrack into it, and such a
// project carries no DTM today because that import creates no terrain
// artifact. There the levels are computed as though the whole site stood
// hundreds of metres above its own ground.
//
// The DTM now does more than set that datum. It is also the terrain profile
// Gl. 15 measures h_m against: with one, the ground under each
// subsegment→receiver path is sampled and the Bodendämpfung of Gl. 14 follows
// the hillside; without one, every path runs over one flat plane, which is the
// special case S/d = (h_g + h_r)/2 and deviation 4 of the conformance
// declaration. Both facts have the same remedy, so they are said together.
func (r *schall03RunResult) warnOnUngroundedElevation(terrainModel terrain.Model, segments []schall03.TrackSegment) {
	if terrainModel != nil {
		return
	}

	if !slices.ContainsFunc(segments, func(segment schall03.TrackSegment) bool { return segment.ElevationM != 0 }) {
		return
	}

	r.logf(
		"WARNING no terrain model: elevation_m is read as a height above the ground under each receiver, and " +
			"h_m (Gl. 15) falls back to the flat-ground case (h_g + h_r)/2 because there is no terrain profile to " +
			"measure the propagation path against. The first is correct where elevation_m means a bridge or " +
			"embankment height, and wrong where it is an absolute Z — a SoundPLAN-imported project is the case in " +
			"point, because its elevation_m is the rail's ZTrack and the SoundPLAN import does not yet produce a DTM. " +
			"Import one with `aconiq import --terrain` to put the path on real ground.",
	)
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
//
// A DTM that covers not one receiver has no mean to fall back to, and taking
// Z = 0 there would reinstate exactly the sea-level datum this function exists
// to remove — silently, on a project that went to the trouble of importing
// terrain. That is refused rather than computed.
func schall03ReceiverInputs(receivers []geo.PointReceiver, terrainModel terrain.Model) ([]schall03.ReceiverInput, int, error) {
	groundZ, sampled, err := schall03ReceiverGroundZ(receivers, terrainModel)
	if err != nil {
		return nil, 0, err
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

	return inputs, sampled, nil
}

// schall03ReceiverGroundZ samples the DTM once per receiver and fills the
// misses, applying the rules schall03ReceiverInputs documents.
func schall03ReceiverGroundZ(receivers []geo.PointReceiver, terrainModel terrain.Model) ([]float64, int, error) {
	groundZ := make([]float64, len(receivers))

	if terrainModel == nil {
		return groundZ, 0, nil
	}

	var hits numeric.CompensatedSum

	covered := make([]bool, len(receivers))
	sampled := 0

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

	if sampled == 0 {
		if len(receivers) == 0 {
			return groundZ, 0, nil
		}

		return nil, 0, schall03TerrainCoversNoReceiverError(receivers, terrainModel)
	}

	fallback := hits.Sum() / float64(sampled)

	for i := range groundZ {
		if !covered[i] {
			groundZ[i] = fallback
		}
	}

	return groundZ, sampled, nil
}

// schall03TerrainCoversNoReceiverError reports a DTM whose extent and the
// receivers' have nothing in common. Both extents are quoted because the
// overwhelmingly likely cause is that they are expressed in different CRS, and
// two disjoint number ranges make that visible at a glance.
func schall03TerrainCoversNoReceiverError(receivers []geo.PointReceiver, terrainModel terrain.Model) error {
	terrainBounds := terrainModel.Bounds()

	minX, minY := receivers[0].Point.X, receivers[0].Point.Y
	maxX, maxY := minX, minY

	for _, receiver := range receivers[1:] {
		minX = math.Min(minX, receiver.Point.X)
		minY = math.Min(minY, receiver.Point.Y)
		maxX = math.Max(maxX, receiver.Point.X)
		maxY = math.Max(maxY, receiver.Point.Y)
	}

	return domainerrors.New(
		domainerrors.KindUserInput,
		"cli.schall03ReceiverInputs",
		fmt.Sprintf(
			"the imported terrain DTM covers none of the %d receivers, so Schall 03 has no ground elevation to measure the propagation path against. "+
				"Terrain bounds are [%.2f, %.2f, %.2f, %.2f]; the receivers span [%.2f, %.2f, %.2f, %.2f]. "+
				"Check that the DTM is in the project's CRS — a CRS mismatch is the usual cause of two extents this far apart — "+
				"and that its extent reaches the site; then re-import it with `aconiq import --terrain`",
			len(receivers),
			terrainBounds[0], terrainBounds[1], terrainBounds[2], terrainBounds[3],
			minX, minY, maxX, maxY,
		),
		nil,
	)
}

// buildSchall03NormativeReceivers derives the auto grid from the normative
// track centerlines.
func buildSchall03NormativeReceivers(segments []schall03.TrackSegment, calcArea *geo.BBox, options schall03RunOptions) ([]geo.PointReceiver, results.GridLayout, error) {
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
