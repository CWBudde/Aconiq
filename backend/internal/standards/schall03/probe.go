package schall03

import (
	"context"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
)

// EstimateNormative says what computing these receivers on the normative
// Anlage-2 chain will cost, by timing a sample of the walk
// ComputeNormativeReceiverOutputsForSceneProgress would run over the same
// receivers and scene.
//
// It refuses what the run would refuse, in the run's words: the run's input
// checks, then what the run's first receiver would refuse — itself, a wall, a
// barrier panel, a segment, the speed-zone split or an emission — and then
// any later receiver. Everything but the receiver records is the same for
// every receiver, so whatever the scene fails on, the run fails on at its
// first receiver.
//
// It computes no level anyone reads. A unit is one line-source walk, run by
// addNormativeWalk — the function the run's walk calls — into a sum the probe
// then drops, so nothing about a run can differ for having been estimated
// first.
//
// The chain walks its receivers one after another on the calling goroutine,
// so the estimate is for one worker whatever opts.Workers says, and it
// prepares no scene shared between receivers: what it prepares, it prepares
// per receiver, and the probe times that as the receiver's one-off cost. So
// opts.Prepare is zero.
func EstimateNormative(
	ctx context.Context,
	receivers []ReceiverInput,
	scene NormativeScene,
	opts runestimate.Options,
) (runestimate.Estimate, error) {
	parts, err := refuseNormative(receivers, scene)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	opts.Workers = 1
	opts.Prepare = 0

	kernel := newNormativeKernel(receivers, scene, parts)

	estimate, err := runestimate.Run(ctx, kernel, normativePoints(receivers), opts)
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	estimate.StandardID = StandardID
	estimate.Inputs.Counts = kernel.estimateCounts()

	return estimate, nil
}

// EstimatePreview says what computing these receivers on the preview data-pack
// chain will cost, by timing a sample of the walk
// ComputeReceiverOutputsWithDataPackProgress would run over the same
// receivers, sources, configuration and data pack.
//
// It refuses what the run would refuse, in the run's words, in the order
// EstimateNormative describes: the first receiver, then the configuration,
// the data pack and every source's emission, then any later receiver.
//
// A unit is one source, run by previewSourceSpectra — the function the run's
// walk calls — and dropped. Like the normative chain, the preview chain walks
// its receivers on one goroutine and prepares nothing shared between them, so
// the estimate is for one worker and opts.Prepare is zero.
//
// The run also sums each receiver's spectra once its sources are walked
// (SumSpectra). That is a few additions per source next to a source's
// hundreds of subsegments, and the probe does not time it.
func EstimatePreview(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []RailSource,
	cfg PropagationConfig,
	pack DataPack,
	opts runestimate.Options,
) (runestimate.Estimate, error) {
	err := refusePreview(receivers, sources, cfg, pack)
	if err != nil {
		return runestimate.Estimate{}, err
	}

	opts.Workers = 1
	opts.Prepare = 0

	kernel := newPreviewKernel(receivers, sources, cfg, pack)

	estimate, err := runestimate.Run(ctx, kernel, previewPoints(receivers), opts)
	if err != nil {
		return runestimate.Estimate{}, fmt.Errorf("%w", err)
	}

	estimate.StandardID = StandardID
	estimate.Inputs.Counts = kernel.estimateCounts()

	return estimate, nil
}

// refuseNormative is what ComputeNormativeReceiverOutputsForSceneProgress
// refuses, with the run's error, and otherwise the speed-zone parts the run
// would split the segments into.
func refuseNormative(receivers []ReceiverInput, scene NormativeScene) ([]speedZonePart, error) {
	if err := checkNormativeInputs(len(receivers), scene); err != nil {
		return nil, err
	}

	first := receivers[0]

	parts, err := checkNormativeScene(first, scene)
	if err != nil {
		return nil, fmt.Errorf("receiver %q: %w", first.ID, err)
	}

	// A later receiver passes every check on the scene that the first one
	// passed, so only its own record can fail.
	for _, receiver := range receivers[1:] {
		err = receiver.Validate()
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}
	}

	return parts, nil
}

// checkNormativeScene is everything walkNormativeReceiver can refuse at
// receiver, in the order it would refuse it: the receiver and the scene, the
// speed-zone split, and the emission of every walk.
func checkNormativeScene(receiver ReceiverInput, scene NormativeScene) ([]speedZonePart, error) {
	err := validateNormativeReceiverScene(receiver, scene)
	if err != nil {
		return nil, err
	}

	parts, err := prepareSpeedZoneParts(scene.Segments)
	if err != nil {
		return nil, err
	}

	for _, part := range parts {
		for _, op := range part.segment.Operations {
			for _, period := range [2]normativePeriod{periodDay, periodNight} {
				_, err = normativeEmission(part, op, period)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return parts, nil
}

// refusePreview is what ComputeReceiverOutputsWithDataPackProgress refuses,
// with the run's error.
func refusePreview(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig, pack DataPack) error {
	if err := requireReceivers(len(receivers)); err != nil {
		return err
	}

	first := receivers[0]

	err := validatePreviewReceiver(first)
	if err != nil {
		return err
	}

	err = checkPreviewReceiver(first.Point, sources, cfg, pack)
	if err != nil {
		return err
	}

	// The emission is the only part of previewSourceSpectra that can fail,
	// and it does not depend on the receiver.
	for _, source := range sources {
		_, err = ComputeEmissionWithDataPack(source, pack)
		if err != nil {
			return err
		}
	}

	for _, receiver := range receivers[1:] {
		err = validatePreviewReceiver(receiver)
		if err != nil {
			return err
		}
	}

	return nil
}

func normativePoints(receivers []ReceiverInput) []geo.Point2D {
	points := make([]geo.Point2D, len(receivers))
	for i, r := range receivers {
		points[i] = r.Point
	}

	return points
}

func previewPoints(receivers []geo.PointReceiver) []geo.Point2D {
	points := make([]geo.Point2D, len(receivers))
	for i, r := range receivers {
		points[i] = r.Point
	}

	return points
}

// subsegmentCount is how many integration steps eachSubsegment cuts a
// centerline into — the walk's own count, taken by walking it.
func subsegmentCount(centerline []geo.Point2D) int {
	n := 0

	eachSubsegment(nil, centerline, func(geo.Point2D, float64, float64, float64, float64) { n++ })

	return n
}

// normativeKernel is runestimate.Kernel over the normative chain.
//
// A unit is one line-source walk: one operation on one speed-zone part for
// one period. Unit 2k is walk k by day and unit 2k+1 the same walk by night,
// and the walks are numbered part by part, operation by operation — the order
// walkNormativeReceiver runs them in. Every receiver walks all of them, so the
// count is the same for each.
//
// The unit is the whole line-source walk and not one of its 10 m
// subsegments: the emission is computed once per walk, and the walk adds the
// direct and the reflected path as two levels, each the logarithm of a sum
// over all the walk's subsegments, so no part of a walk is a step the run
// takes on its own.
type normativeKernel struct {
	receivers []ReceiverInput
	scene     NormativeScene
	parts     []speedZonePart
	walks     []normativeWalkRef
}

// normativeWalkRef names one line-source walk by its speed-zone part and the
// index of the operation in that part's segment.
type normativeWalkRef struct {
	part int
	op   int
}

func newNormativeKernel(receivers []ReceiverInput, scene NormativeScene, parts []speedZonePart) *normativeKernel {
	var walks []normativeWalkRef

	for p, part := range parts {
		for o := range part.segment.Operations {
			walks = append(walks, normativeWalkRef{part: p, op: o})
		}
	}

	return &normativeKernel{receivers: receivers, scene: scene, parts: parts, walks: walks}
}

// Units is every walk, once per period.
func (k *normativeKernel) Units(int) int {
	return len(periodSums{}) * len(k.walks)
}

func (k *normativeKernel) NewWalker() runestimate.Walker {
	return k.newWalker()
}

func (k *normativeKernel) newWalker() *normativeWalker {
	return &normativeWalker{kernel: k}
}

// estimateCounts names what the scene is made of, for the reader of an
// estimate who wants to know why it is large. subsegments is the integration
// steps of every part once; each walk of a part steps through all of them.
func (k *normativeKernel) estimateCounts() map[string]int {
	subsegments := 0
	for _, part := range k.parts {
		subsegments += subsegmentCount(part.segment.TrackCenterline)
	}

	return map[string]int{
		"track_segments":    len(k.scene.Segments),
		"speed_zone_parts":  len(k.parts),
		"line_source_walks": k.Units(0),
		"subsegments":       subsegments,
		"walls":             len(k.scene.Walls),
		"barrier_panels":    len(k.scene.Barriers),
	}
}

// normativeWalker runs single line-source walks of one receiver at a time.
type normativeWalker struct {
	kernel *normativeKernel

	// What the current receiver reads: its record, and the speed-zone parts
	// its Begin prepared, as the run prepares them for it.
	receiver ReceiverInput
	parts    []speedZonePart

	sums periodSums
}

// Begin does what walkNormativeReceiver does before a receiver's first walk:
// it validates the receiver and every wall and barrier panel of the scene,
// and splits the segments into speed-zone parts. The run repeats both for
// every receiver, so the probe does too.
func (w *normativeWalker) Begin(receiver int) {
	w.receiver = w.kernel.receivers[receiver]

	// EstimateNormative has already refused anything either could fail on.
	_ = validateNormativeReceiverScene(w.receiver, w.kernel.scene)
	w.parts, _ = prepareSpeedZoneParts(w.kernel.scene.Segments)
}

// Unit runs one walk of the receiver last begun and drops what it
// contributed.
func (w *normativeWalker) Unit(unit int) {
	w.sums = periodSums{}

	w.addUnit(unit)
}

// addUnit adds one walk's contribution to the walker's sum for its period.
func (w *normativeWalker) addUnit(unit int) {
	walk := w.kernel.walks[unit/len(w.sums)]
	period := normativePeriod(unit % len(w.sums))
	part := w.parts[walk.part]

	// EstimateNormative has already refused an emission this could fail on.
	_ = addNormativeWalk(part, part.segment.Operations[walk.op], period, w.receiver, w.kernel.scene, nil, &w.sums[period])
}

// previewKernel is runestimate.Kernel over the preview data-pack chain. A
// unit is one source, in source order: every receiver walks all of them.
type previewKernel struct {
	receivers []geo.PointReceiver
	sources   []RailSource
	cfg       PropagationConfig
	pack      DataPack
}

func newPreviewKernel(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig, pack DataPack) *previewKernel {
	return &previewKernel{receivers: receivers, sources: sources, cfg: cfg, pack: pack}
}

// Units is every source.
func (k *previewKernel) Units(int) int {
	return len(k.sources)
}

func (k *previewKernel) NewWalker() runestimate.Walker {
	return k.newWalker()
}

func (k *previewKernel) newWalker() *previewWalker {
	return &previewWalker{kernel: k}
}

// estimateCounts names what the scene is made of. subsegments is the
// integration steps of every source's centerline; every receiver walks all
// of them, by day and by night.
func (k *previewKernel) estimateCounts() map[string]int {
	subsegments := 0
	for _, source := range k.sources {
		subsegments += subsegmentCount(source.TrackCenterline)
	}

	return map[string]int{
		"rail_sources": len(k.sources),
		"subsegments":  subsegments,
	}
}

// previewWalker runs single sources of one receiver at a time.
type previewWalker struct {
	kernel *previewKernel
	point  geo.Point2D

	// day and night are the spectra the walker's units produced, in unit
	// order, as walkPreviewReceiver collects them.
	day, night []OctaveSpectrum
}

// Begin does what the run does for a receiver before its first source: it
// checks the receiver record, the configuration and the data pack, and sets
// aside room for one spectrum per source and period.
func (w *previewWalker) Begin(receiver int) {
	k := w.kernel
	w.point = k.receivers[receiver].Point

	// EstimatePreview has already refused anything these could fail on.
	_ = validatePreviewReceiver(k.receivers[receiver])
	_ = checkPreviewReceiver(w.point, k.sources, k.cfg, k.pack)

	w.day = make([]OctaveSpectrum, 0, len(k.sources))
	w.night = make([]OctaveSpectrum, 0, len(k.sources))
}

// Unit runs one source of the receiver last begun and drops its spectra.
func (w *previewWalker) Unit(unit int) {
	w.day, w.night = w.day[:0], w.night[:0]

	w.addUnit(unit)
}

// addUnit appends one source's day and night spectra to the walker's.
func (w *previewWalker) addUnit(unit int) {
	k := w.kernel

	// EstimatePreview has already refused an emission this could fail on.
	day, night, _ := previewSourceSpectra(w.point, k.sources[unit], k.cfg, k.pack)

	w.day = append(w.day, day)
	w.night = append(w.night, night)
}
