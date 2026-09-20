package road

import (
	"errors"
	"fmt"
	"math"

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

	// segmentCount is the total over all sources. It sizes the walk's
	// contribution slices in one allocation instead of letting them grow from
	// a guess, which on a 2 km road at the default 1 m Teilstück length was a
	// dozen reallocations per receiver. Those slices now live in pathScratch,
	// so that one allocation is taken once per walk rather than once per
	// receiver; a nil-scratch call still takes it per receiver.
	segmentCount int
}

// preparedSource is one road source reduced to what the propagation walk reads
// from it: its Teilstücke and, per Teilstück, the sound power level the path
// starts from.
//
// baseDayDB and baseNightDB are parallel to segments — index i belongs to
// segment i — and hold L_m,E + 10·lg(l_i/l_0) (RLS-19 Eq. 10). Both halves are
// receiver-independent: SplitLineIntoSegments fixes l_i here, and ComputeEmission
// fixes L_m,E here, so the length weight is the same number for every one of a
// grid's receivers. Computing it in the walk meant 10 000 receivers × 2 000
// Teilstücke math.Log10 calls for 2 000 distinct arguments.
//
// Holding the *sum* rather than the weight alone also removes the second of the
// two places the walk used to add it: the direct contribution and the mirrored
// one both start from this value.
//
// It is bit-for-bit what the walk computed. Go evaluates `a + b - c`
// left-to-right as `(a + b) - c`, so lifting `a + b` out keeps the same
// association and the same two roundings; and the lifted expression is written
// in the same shape it had in the walk — `10 * math.Log10(…)` into a variable,
// then added — so an architecture that contracts a multiply-add contracts the
// same one it contracted before.
//
// The field sits here and not on the exported Segment: it is a function of the
// source's emission as much as of the segment's length, and Segment is pure
// geometry that callers outside this package build and read.
type preparedSource struct {
	segments    []Segment
	baseDayDB   []float64
	baseNightDB []float64
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

	// Length weighting (RLS-19 Eq. 10): the sub-segment sound power level is the
	// length-related emission level L_m,E [dB(A)/m] plus 10·lg(l_i / l_0) with the
	// reference length l_0 = 1 m. L_m,E is already per-metre (emission.go converts
	// veh/h ÷ km/h to veh/m via the −30 dB term), so the weight must NOT be
	// normalised by the total road length: doing so would make the whole road
	// radiate the power of a single 1 m long section.
	baseDayDB := make([]float64, len(segments))
	baseNightDB := make([]float64, len(segments))

	for i, seg := range segments {
		lengthWeight := 10 * math.Log10(seg.LengthM/referenceLengthM)
		baseDayDB[i] = emission.LmEDay + lengthWeight
		baseNightDB[i] = emission.LmENight + lengthWeight
	}

	return preparedSource{segments: segments, baseDayDB: baseDayDB, baseNightDB: baseNightDB}, nil
}

// ValidateReceivers makes computeReceivers' per-receiver refusals over the
// whole list, in input order, and returns the first.
//
// A parallel driver needs this because its chunks refuse concurrently. The
// sequential walk checks and computes one receiver at a time, so it always
// reports the lowest-index bad receiver; a pool can have a high-index chunk
// fail, cancel its siblings, and report that refusal while a lower-index
// chunk was skipped before it reached its own bad receiver. Then the sentence
// a user reads depends on which goroutine got there first, which is precisely
// what the sequential-refusal contract promises it does not.
//
// Running these checks up front removes the ambiguity rather than papering
// over it. Every receiver-shaped refusal is found here, in order, before any
// chunk exists. What remains reachable inside the walk is receiver-
// *independent* — appendParkingContributions refuses a parking source, and
// ComputeParkingEmission reads only the source — so every chunk would produce
// that same sentence and it no longer matters which one reports it.
//
// It duplicates the checks in computeReceivers rather than replacing them.
// The sequential walk is the reference the parallel driver is tested against,
// and a reference that called out to its challenger for its own refusals
// would not be one.
func ValidateReceivers(receivers []geo.PointReceiver, cfg PropagationConfig) error {
	for _, receiver := range receivers {
		if receiver.ID == "" {
			return errors.New("receiver id is required")
		}

		if !receiver.Point.IsFinite() {
			return fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
		}

		receiverCfg := cfg
		receiverCfg.ReceiverHeightM = receiver.HeightM

		if err := receiverCfg.Validate(); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// PrepareSceneFor derives the receiver-independent half once, keeping
// ComputeReceiverOutputs' order of refusals.
//
// That order is load-bearing and not obvious. ComputeReceiverOutputs prepares
// the scene *lazily*, on the first receiver that has cleared its own checks, so
// a model that is wrong in both ways at once — a receiver with no ID and a
// source with no ID — reports the receiver. Preparing eagerly, which is exactly
// what this function does to get the saving it exists for, would swap the two
// around.
//
// So the eager preparation is speculative: when it succeeds nothing was
// reordered, because a scene that prepares cleanly has no refusal to lose the
// race. When it fails, the refusal the caller hears is not ours to choose, and
// ComputeReceiverOutputs is asked to word it. That costs a second
// PrepareScene, on a path that is returning an error and computing nothing.
//
// It lives here rather than in wasmkernel, where it was written, because both
// parallel drivers need it and neither can afford its own copy: the browser's
// shards and the CLI's goroutine pool have to refuse a bad model in the same
// sentence as each other and as the sequential walk.
func PrepareSceneFor(
	receivers []geo.PointReceiver,
	sources []RoadSource,
	barriers []Barrier,
	cfg PropagationConfig,
) (*Scene, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	// The first receiver's height, as computeReceivers would have used it:
	// ReceiverHeightM is the one config field a receiver overrides, and
	// PrepareScene reads it only to validate it.
	firstCfg := cfg
	firstCfg.ReceiverHeightM = receivers[0].HeightM

	scene, err := PrepareScene(sources, barriers, firstCfg)
	if err == nil {
		return scene, nil
	}

	_, refusal := ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if refusal != nil {
		return nil, fmt.Errorf("%w", refusal)
	}

	// Unreachable as the two are written today — ComputeReceiverOutputs
	// prepares from the same first receiver, so it cannot succeed where
	// PrepareScene failed. Returning the refusal we already have rather than a
	// nil scene keeps that an error instead of a panic if it ever stops being
	// true.
	return nil, fmt.Errorf("%w", err)
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
// made by the caller. It is what the grid loop runs, so with a scratch it
// allocates nothing at all.
//
// The two contribution slices come from the scratch when there is one. They are
// the largest per-receiver allocation in the walk — one float64 per Teilstück
// per period, so 32 KB per receiver at 2 000 Teilstücke, a third of a gigabyte
// over a 10 000-point grid — and nothing reads them after EnergySum has, so the
// backing array can be handed to the next receiver. Reusing it cannot move a
// result: the slices are truncated to zero length before the walk, appended to
// in exactly the order they were before, and EnergySum reads them in slice
// order, so both the contents and the order are the ones the fresh slices had.
//
// A nil scratch still allocates, because a nil scratch is the unindexed
// reference path pathScratch documents — spatial_test.go computes against it
// and requires the two to agree to the bit, which it could not do if the
// reference path shared state with the walk it is checking.
func (s *Scene) receiverLevelsOn(
	receiver geo.Point2D,
	cfg PropagationConfig,
	scratch *pathScratch,
) (PeriodLevels, error) {
	// Absolute receiver Z = terrain elevation + height above ground.
	receiverZ := cfg.ReceiverTerrainZ + cfg.ReceiverHeightM

	var dayContrib, nightContrib []float64

	if scratch != nil {
		dayContrib = scratch.dayContrib[:0]
		nightContrib = scratch.nightContrib[:0]
	} else {
		dayContrib = make([]float64, 0, s.segmentCount)
		nightContrib = make([]float64, 0, s.segmentCount)
	}

	for _, source := range s.sources {
		for i, seg := range source.segments {
			appendSegmentContributions(
				&dayContrib, &nightContrib,
				source.baseDayDB[i], source.baseNightDB[i],
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

	var err error

	if len(cfg.ParkingSources) > 0 {
		err = appendParkingContributions(
			&dayContrib, &nightContrib, cfg.ParkingSources,
			receiver, receiverZ, s.barriers, &s.reflectors, cfg, scratch,
		)
	}

	// Write back before the error check: the appends above may have grown the
	// slices past the scratch's backing array, and that growth is exactly what
	// the next receiver must inherit — the same pattern scratch.reflectedPaths
	// uses in appendReflectedContribs.
	if scratch != nil {
		scratch.dayContrib = dayContrib
		scratch.nightContrib = nightContrib
	}

	if err != nil {
		return PeriodLevels{}, err
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
