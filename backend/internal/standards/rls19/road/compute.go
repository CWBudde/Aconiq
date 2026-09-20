package road

import (
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
)

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// Scene is the receiver-independent half of an RLS-19 road calculation: every
// source already validated, its emission computed and its line already split
// into Teilstücke, plus the barrier and reflector sets the buildings have been
// folded into.
//
// Nothing in it reads the receiver. Deriving it once and walking a grid over it
// is therefore not an approximation of the per-receiver path — it is the same
// arithmetic, done once instead of once per receiver. The saving is not
// marginal: SplitLineIntoSegments is O(segments × vertices), because
// interpolateAlongPolyline walks the polyline from vertex 0 again for every
// segment it places, so a 10 000-point grid used to re-split every road 10 000
// times before computing a single level.
//
// A Scene also owns the spatial indices over those two sets. An index belongs
// here for the same reason the Teilstücke do: it is derived from the model and
// not from the receiver, so building it per receiver would recreate the cost
// it exists to remove. It is immutable once prepared; the mutable half of a
// query lives in pathScratch, one per receiver walk.
type Scene struct {
	sources    []preparedSource
	barriers   []Barrier      // explicit barriers plus the ones the buildings imply
	reflectors reflectorField // likewise, already flattened into walls and indexed

	// barrierGrid answers which barriers a source→receiver ray can reach. It
	// is nil when the model has no barrier or when one of them has no usable
	// bounding box, and a nil index means "test them all" — the walk the code
	// did before the index existed.
	barrierGrid *geo.BBoxGrid

	// segmentCount is the total over all sources. It sizes the per-receiver
	// contribution slices in one allocation instead of letting them grow from
	// a guess, which on a 2 km road at the default 1 m Teilstück length was a
	// dozen reallocations per receiver.
	segmentCount int
}

// preparedSource is one road source reduced to what the propagation walk reads
// from it: its emission levels and its Teilstücke.
type preparedSource struct {
	emission EmissionResult
	segments []Segment
}

// PrepareScene derives the receiver-independent state of a calculation.
//
// It makes the refusals the per-receiver path used to make on its first
// iteration — an invalid configuration, a model with nothing in it that makes
// sound, an invalid source — and makes them in that order.
func PrepareScene(sources []RoadSource, barriers []Barrier, cfg PropagationConfig) (*Scene, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	return prepareScene(sources, barriers, cfg)
}

// prepareScene is PrepareScene without the configuration check, for the callers
// that have already made it against the per-receiver configuration. Only
// ReceiverHeightM differs between that configuration and this one, and nothing
// below reads it.
func prepareScene(sources []RoadSource, barriers []Barrier, cfg PropagationConfig) (*Scene, error) {
	if len(sources) == 0 && len(cfg.ParkingSources) == 0 {
		return nil, errors.New("at least one road source or parking source is required")
	}

	effectiveBarriers := buildingBarriers(barriers, cfg.Buildings)

	scene := &Scene{
		sources:      make([]preparedSource, 0, len(sources)),
		barriers:     effectiveBarriers,
		reflectors:   newReflectorField(buildingReflectors(cfg.Reflectors, cfg.Buildings)),
		barrierGrid:  newBarrierGrid(effectiveBarriers),
		segmentCount: 0,
	}

	for _, source := range sources {
		prepared, err := prepareSource(source, cfg.SegmentLengthM)
		if err != nil {
			return nil, err
		}

		// A source that yields no Teilstück contributes nothing. The
		// per-receiver path dropped it without a word and so does this.
		if len(prepared.segments) == 0 {
			continue
		}

		scene.sources = append(scene.sources, prepared)
		scene.segmentCount += len(prepared.segments)
	}

	return scene, nil
}

// prepareSource validates one source, computes its emission and splits its
// source line into Teilstücke. A prepared source with no segments is a source
// whose geometry is degenerate; that is not an error, it is silence.
func prepareSource(source RoadSource, segmentLengthM float64) (preparedSource, error) {
	err := source.Validate()
	if err != nil {
		return preparedSource{}, err
	}

	emission, err := ComputeEmission(source)
	if err != nil {
		return preparedSource{}, err
	}

	// Prepare per-vertex elevations: use CenterlineElevations if provided,
	// otherwise fill from the uniform ElevationM field.
	elevations := source.CenterlineElevations
	if len(elevations) != len(source.Centerline) {
		elevations = make([]float64, len(source.Centerline))
		for i := range elevations {
			elevations[i] = source.ElevationM
		}
	}

	sourceLine := source.EffectiveCenterline()

	// Split the source line into sub-segments (Teilstueckverfahren).
	segments := SplitLineIntoSegments(sourceLine, elevations, segmentLengthM)
	if len(segments) == 0 {
		return preparedSource{}, nil
	}

	if polylineLength(sourceLine) <= 0 {
		return preparedSource{}, nil
	}

	return preparedSource{emission: emission, segments: segments}, nil
}

// ComputeReceiverLevels computes LrDay/LrNight at one receiver against a scene
// that was prepared once.
func (s *Scene) ComputeReceiverLevels(receiver geo.Point2D, cfg PropagationConfig) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if !receiver.IsFinite() {
		return PeriodLevels{}, errors.New("receiver is not finite")
	}

	return s.receiverLevels(receiver, cfg)
}

// ComputeReceivers computes indicators for all receivers in order against a
// scene the caller prepared.
func (s *Scene) ComputeReceivers(receivers []geo.PointReceiver, cfg PropagationConfig) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	return computeReceivers(receivers, cfg, func(PropagationConfig) (*Scene, error) {
		return s, nil
	})
}

// receiverLevels is the propagation walk for one receiver, on scratch of its
// own. Use receiverLevelsOn from a loop, so that the scratch — and with it the
// index cursors — survives from one receiver to the next.
func (s *Scene) receiverLevels(receiver geo.Point2D, cfg PropagationConfig) (PeriodLevels, error) {
	return s.receiverLevelsOn(receiver, cfg, s.newScratch())
}

// receiverLevelsOn is the propagation walk itself, with every check already
// made by the caller. It is what the grid loop runs, so it allocates nothing
// beyond the two contribution slices.
func (s *Scene) receiverLevelsOn(
	receiver geo.Point2D,
	cfg PropagationConfig,
	scratch *pathScratch,
) (PeriodLevels, error) {
	// Absolute receiver Z = terrain elevation + height above ground.
	receiverZ := cfg.ReceiverTerrainZ + cfg.ReceiverHeightM

	dayContrib := make([]float64, 0, s.segmentCount)
	nightContrib := make([]float64, 0, s.segmentCount)

	for _, source := range s.sources {
		for _, seg := range source.segments {
			appendSegmentContributions(
				&dayContrib, &nightContrib,
				source.emission,
				seg,
				receiver,
				receiverZ,
				pointSourceHeightM,
				s.barriers,
				&s.reflectors,
				cfg,
				cfg.Terrain,
				scratch,
			)
		}
	}

	if len(cfg.ParkingSources) > 0 {
		err := appendParkingContributions(
			&dayContrib, &nightContrib, cfg.ParkingSources,
			receiver, receiverZ, s.barriers, &s.reflectors, cfg, scratch,
		)
		if err != nil {
			return PeriodLevels{}, err
		}
	}

	return PeriodLevels{
		LrDay:   acoustics.EnergySum(dayContrib),
		LrNight: acoustics.EnergySum(nightContrib),
	}, nil
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
// This is the top-level entry point for RLS-19 road calculations.
// Barriers are optional; pass nil for free-field calculation.
//
// It prepares the scene itself. A caller that computes several grids over the
// same model should prepare it once with PrepareScene and call ComputeReceivers.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RoadSource, barriers []Barrier, cfg PropagationConfig) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	return computeReceivers(receivers, cfg, func(first PropagationConfig) (*Scene, error) {
		return prepareScene(sources, barriers, first)
	})
}

// computeReceivers walks the receivers in the order given.
//
// prepare is called at most once, on the first receiver that has passed its own
// checks rather than before the loop, because that is where the source refusals
// used to happen: every receiver re-prepared the scene, so a malformed receiver
// was reported ahead of a malformed source. Preparing eagerly would swap the
// two errors around for a model that is wrong in both ways at once.
func computeReceivers(
	receivers []geo.PointReceiver,
	cfg PropagationConfig,
	prepare func(PropagationConfig) (*Scene, error),
) ([]ReceiverOutput, error) {
	var (
		scene   *Scene
		scratch *pathScratch
	)

	outputs := make([]ReceiverOutput, 0, len(receivers))

	for _, receiver := range receivers {
		if receiver.ID == "" {
			return nil, errors.New("receiver id is required")
		}

		if !receiver.Point.IsFinite() {
			return nil, fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
		}

		receiverCfg := cfg
		receiverCfg.ReceiverHeightM = receiver.HeightM

		err := receiverCfg.Validate()
		if err != nil {
			return nil, err
		}

		if scene == nil {
			scene, err = prepare(receiverCfg)
			if err != nil {
				return nil, err
			}

			// One scratch for the whole walk: it carries the index cursors,
			// whose per-item stamps are the only thing standing between a
			// query and a freshly allocated map.
			scratch = scene.newScratch()
		}

		periodLevels, err := scene.receiverLevelsOn(receiver.Point, receiverCfg, scratch)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: periodLevels.ToReceiverIndicators(),
		})
	}

	return outputs, nil
}
