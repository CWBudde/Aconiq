package road

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
)

// Estimate says what computing these receivers will cost, by preparing the
// scene the run would prepare and timing a sample of its walk.
//
// It refuses what the run would refuse, in the run's words: the scene is
// prepared by PrepareSceneForContext, the receivers are checked by
// ValidateReceivers — the two calls ComputeReceiverOutputsParallel makes
// before its first receiver — and a Parkplatz whose emission cannot be
// computed is refused as the walk would refuse it on its first receiver.
//
// It computes no level anyone reads. The probe runs the walk's own per-pair
// step — appendSegmentContributions, and appendParkingContributions for a
// Parkplatz — on a Walker's scratch, into a sum it then drops, so nothing
// about a run can differ for having been estimated first.
//
// A run in fixed segment_length_mode also gets the distance-scaled
// alternative costed, because that is the switch most likely to take a run
// from hours to minutes. Its sources are prepared again in that mode (see
// withSegmentLengthMode) and probed on their own, because which pairs survive
// the ladder changes what the average pair costs — but only if the budget
// has time left once the run's own probe is done. Its wall time carries its
// own preparation (see alternativePrepare), because building the ladders is
// work the fixed-mode scene never did.
func Estimate(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []RoadSource,
	barriers []Barrier,
	cfg PropagationConfig,
	opts runestimate.Options,
) (runestimate.Estimate, error) {
	startedAt := time.Now()

	scene, err := PrepareSceneForContext(ctx, receivers, sources, barriers, cfg)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	err = ValidateReceivers(receivers, cfg)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	for _, parking := range cfg.ParkingSources {
		_, err = ComputeParkingEmission(parking)
		if err != nil {
			return runestimate.Estimate{}, err
		}
	}

	prepare := time.Since(startedAt)
	opts.Prepare = prepare

	if cfg.SegmentLengthMode.resolved() == SegmentLengthFixed {
		opts.Alternatives = append(opts.Alternatives, distanceScaledAlternative(scene, receivers, sources, cfg, prepare))
	}

	points := make([]geo.Point2D, len(receivers))
	for i, r := range receivers {
		points[i] = r.Point
	}

	estimate, err := runestimate.Run(ctx, newEstimateKernel(scene, receivers, cfg), points, opts)
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	estimate.StandardID = StandardID
	estimate.Inputs.Counts = scene.estimateCounts(len(barriers), cfg)

	return estimate, nil
}

// distanceScaledAlternative is the distance-scaled run a fixed-mode estimate
// also costs, over scene, which the estimate prepared in prepare.
//
// Its kernel is built only if the budget leaves time for it, and priced with
// the preparation a distance-scaled run would spend: its own ladders in place
// of the fixed Teilstücke, everything else as measured for scene.
func distanceScaledAlternative(
	scene *Scene,
	receivers []geo.PointReceiver,
	sources []RoadSource,
	cfg PropagationConfig,
	prepare time.Duration,
) runestimate.AlternativeKernel {
	scaledCfg := cfg
	scaledCfg.SegmentLengthMode = SegmentLengthDistanceScaled

	return runestimate.AlternativeKernel{
		Params: map[string]string{"segment_length_mode": string(SegmentLengthDistanceScaled)},
		NewKernel: func() (runestimate.Kernel, time.Duration, error) {
			scaled, err := scene.withSegmentLengthMode(sources, scaledCfg)
			if err != nil {
				return nil, 0, err
			}

			return newEstimateKernel(scaled, receivers, scaledCfg), alternativePrepare(prepare, scene, scaled), nil
		},
	}
}

// withSegmentLengthMode is the scene a run in cfg's segment_length_mode would
// prepare, sharing everything of s that the mode does not touch.
//
// Only the sources read SegmentLengthMode; the barrier set, the reflector
// field and both indices are functions of the model alone and immutable once
// prepared, so they are shared rather than rebuilt — on a city extract the
// reflector index is the expensive half of a scene. The sources come from
// prepareSources, the function prepareScene uses, so the ladder is the one a
// run in that mode would build.
func (s *Scene) withSegmentLengthMode(sources []RoadSource, cfg PropagationConfig) (*Scene, error) {
	prepared, took, err := prepareSourcesTimed(sources, cfg)
	if err != nil {
		return nil, err
	}

	scene := *s
	scene.sources = prepared
	scene.sourcesPrepare = took

	return &scene, nil
}

// alternativePrepare is how long a run would spend preparing the scene
// `alternative` stands for, given that the estimate spent `prepare` on
// `scene`, from which `alternative` was derived by withSegmentLengthMode.
//
// A run in the alternative's mode prepares the whole scene from scratch, but
// only its sources differ from what the estimate prepared: the barrier set,
// the reflector field, both indices, the receiver and Parkplatz checks are
// the same work in either mode. So it is the measured preparation with the
// one set of sources swapped for the other — exact up to the clock, and
// without preparing anything twice. Both terms are wall-clock, as prepare
// is, and scene.sourcesPrepare is a part of prepare, so the result is never
// negative.
func alternativePrepare(prepare time.Duration, scene, alternative *Scene) time.Duration {
	return prepare - scene.sourcesPrepare + alternative.sourcesPrepare
}

// estimateCounts names what the scene is made of, for the reader of an
// estimate who wants to know why it is large.
func (s *Scene) estimateCounts(explicitBarriers int, cfg PropagationConfig) map[string]int {
	teilstuecke := 0
	for i := range s.sources {
		teilstuecke += len(s.sources[i].segments)
	}

	return map[string]int{
		"road_sources":    len(s.sources),
		"teilstuecke":     teilstuecke,
		"buildings":       len(cfg.Buildings),
		"reflector_walls": len(s.reflectors.walls),
		"barriers":        explicitBarriers,
		"parking_sources": len(cfg.ParkingSources),
	}
}

// estimateKernel is runestimate.Kernel over a prepared scene.
//
// A unit is one (source, Teilstück) pair, in the order receiverLevelsOn walks
// them, followed by one unit per Parkplatz, which is where the walk puts
// them.
type estimateKernel struct {
	scene     *Scene
	receivers []geo.PointReceiver
	cfg       PropagationConfig
}

func newEstimateKernel(scene *Scene, receivers []geo.PointReceiver, cfg PropagationConfig) *estimateKernel {
	return &estimateKernel{scene: scene, receivers: receivers, cfg: cfg}
}

// Units is how many Teilstücke the walk visits for the receiver — the length
// of what levelFor hands it, per source, which is exactly the slice
// receiverLevelsOn ranges over — plus the Parkplätze.
func (k *estimateKernel) Units(i int) int {
	n := len(k.cfg.ParkingSources)

	point := k.receivers[i].Point
	for j := range k.scene.sources {
		segments, _, _ := k.scene.sources[j].levelFor(point)
		n += len(segments)
	}

	return n
}

func (k *estimateKernel) NewWalker() runestimate.Walker {
	return k.newWalker()
}

func (k *estimateKernel) newWalker() *estimateWalker {
	return &estimateWalker{
		kernel:   k,
		walk:     k.scene.NewWalker(),
		receiver: -1,
	}
}

// estimateWalker runs single units of one receiver at a time on a Walker —
// the type a run's worker walks with — so the probe times the run's code on
// the run's scratch: the index cursors and the receiver-side reflection
// table carry from unit to unit as they carry from Teilstück to Teilstück.
type estimateWalker struct {
	kernel *estimateKernel
	walk   *Walker

	// What the current receiver reads: its config, its absolute height, and
	// per source the Teilstücke levelFor chose for it, with offsets[i] the
	// first unit of source i.
	receiver  int
	cfg       PropagationConfig
	receiverZ float64
	levels    []sourceLevel
	offsets   []int

	sums periodSums
}

// Begin does a receiver's one-off work: the rung levelFor picks per source,
// and the receiver-side reflection table, which the walk builds on its first
// mirrored path and keys on the receiver.
func (w *estimateWalker) Begin(receiver int) {
	w.selectReceiver(receiver)

	if reflectors := &w.kernel.scene.reflectors; len(reflectors.walls) > 0 {
		reflectors.mirrorsFor(w.kernel.receivers[receiver].Point, w.walk.scratch)
	}
}

// Unit runs one unit of the receiver last begun and drops what it
// contributed.
func (w *estimateWalker) Unit(unit int) {
	w.sums = periodSums{}

	w.addUnit(w.receiver, unit)
}

func (w *estimateWalker) selectReceiver(receiver int) {
	if receiver == w.receiver {
		return
	}

	r := w.kernel.receivers[receiver]
	sources := w.kernel.scene.sources

	w.receiver = receiver
	w.cfg = w.kernel.cfg
	w.cfg.ReceiverHeightM = r.HeightM
	w.receiverZ = w.cfg.ReceiverTerrainZ + w.cfg.ReceiverHeightM
	w.levels = w.levels[:0]
	w.offsets = w.offsets[:0]

	offset := 0

	for i := range sources {
		segments, baseDay, baseNight := sources[i].levelFor(r.Point)
		w.levels = append(w.levels, sourceLevel{segments: segments, baseDayDB: baseDay, baseNightDB: baseNight})
		w.offsets = append(w.offsets, offset)
		offset += len(segments)
	}

	w.offsets = append(w.offsets, offset)
}

// addUnit adds one unit's contributions, direct and mirrored, to the
// walker's sum.
func (w *estimateWalker) addUnit(receiver, unit int) {
	w.selectReceiver(receiver)

	point := w.kernel.receivers[receiver].Point
	scene := w.kernel.scene
	roadUnits := w.offsets[len(w.offsets)-1]

	if unit >= roadUnits {
		lot := unit - roadUnits

		// Estimate has already refused a Parkplatz this could fail on.
		_ = appendParkingContributions(
			&w.sums, w.cfg.ParkingSources[lot:lot+1],
			point, w.receiverZ, scene.barriers, &scene.reflectors, w.cfg, w.walk.scratch,
		)

		return
	}

	// The source whose units contain unit: the last offset at or below it.
	source := sort.Search(len(w.offsets), func(i int) bool { return w.offsets[i] > unit }) - 1
	level := &w.levels[source]
	j := unit - w.offsets[source]

	appendSegmentContributions(
		&w.sums,
		level.baseDayDB[j], level.baseNightDB[j],
		level.segments[j],
		point,
		w.receiverZ,
		pointSourceHeightM,
		scene.barriers,
		&scene.reflectors,
		w.cfg,
		w.cfg.Terrain,
		w.walk.scratch,
	)
}
