package schall03

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/numeric"
)

// heightAboveSO maps Teilquelle height index h to metres above Schienenoberkante.
// h=1 → 0 m (rail level), h=2 → 4 m (pantograph), h=3 → 5 m (above pantograph).
var heightAboveSO = map[int]float64{1: 0, 2: 4, 3: 5}

// teilquelleHeightIndices lists the Teilquelle height indices in a fixed order.
//
// Emission spectra are stored in a map[int]BeiblattSpectrum.  Ranging over that
// map and accumulating floats would make the summation order depend on Go's
// randomised map iteration, which docs/policies/determinism.md forbids ("Map
// iteration order must never influence numeric results").  Every accumulation
// over PerHeight therefore walks this slice instead.
var teilquelleHeightIndices = [3]int{1, 2, 3}

// buildVehicleInputs converts a TrainOperation into VehicleInput records for
// one planning period using the provided trains-per-hour value.
//
// It takes a speedZonePart rather than a bare TrackSegment so that a stretch
// lying between two Nr. 5.3.2 zones carries its substitution suppression into
// the emission computation.
func buildVehicleInputs(part speedZonePart, op TrainOperation, trainsPerHour float64) StreckeEmissionInput {
	seg := part.segment

	// The mode is derived from the first Fz of the composition, the same
	// convention ComputeStreckeEmission uses to pick between Tabelle 7 and
	// Tabelle 15.  It decides which speed rule applies: Nr. 4.3 for
	// Eisenbahnen, Nr. 5.3.2 for Straßenbahnen.
	isStrassenbahn := len(op.FzComposition) > 0 && IsStrassenbahnFz(op.FzComposition[0].Fz)
	effectiveSpeed := resolveEffectiveSpeed(seg.StreckeMaxKPH, op.SpeedKPH, seg.IsStation, isStrassenbahn)

	vehicles := make([]VehicleInput, 0, len(op.FzComposition))
	for _, fc := range op.FzComposition {
		vehicles = append(vehicles, VehicleInput{
			Fz:       fc.Fz,
			NPerHour: trainsPerHour * float64(fc.Count),
		})
	}

	return StreckeEmissionInput{
		Vehicles:        vehicles,
		SpeedKPH:        effectiveSpeed,
		Fahrbahn:        seg.Fahrbahn,
		SFahrbahn:       seg.SFahrbahn,
		Surface:         seg.Surface,
		BridgeType:      seg.BridgeType,
		BridgeMitig:     seg.BridgeMitig,
		CurveRadiusM:    seg.CurveRadiusM,
		PermanentlySlow: seg.PermanentlySlow,

		SubstitutionSuppressed: part.substitutionSuppressed,
	}
}

// prepareSpeedZoneParts validates every segment against its own index and then
// expands it into the homogeneous stretches the Nr. 5.3.2 speed substitution
// needs.  Validation runs over the caller's list so error messages keep naming
// the segment the caller passed, not the part it was cut into.
func prepareSpeedZoneParts(segments []TrackSegment) ([]speedZonePart, error) {
	for si := range segments {
		err := segments[si].Validate()
		if err != nil {
			return nil, fmt.Errorf("segment[%d]: %w", si, err)
		}
	}

	return splitSegmentsForSpeedZones(segments)
}

// normativeSinDelta2 computes sin²(δ) where δ is the angle between the
// source→receiver vector and the track axis (for Gl. 8 directivity).
func normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen float64) float64 {
	if tvLen <= 0 {
		return 1 // degenerate segment: treat as perpendicular
	}

	cosTheta := (rvX*tvX + rvY*tvY) / (dp * tvLen)
	cosTheta = math.Max(-1, math.Min(1, cosTheta))

	return math.Max(0, 1-cosTheta*cosTheta)
}

// subsegmentContrib is the single normative propagation kernel behind all four
// Schall 03 subsegment entry points.  It returns the total linear acoustic
// power contribution from one track subsegment step to a receiver, summed over
// all height levels h and octave bands f (Gl. 6, 8-16).
//
// The four entry points differ in exactly two additive dB terms:
//
//   - dRho, the cumulative absorption loss of a reflected path — Gl. 28: the
//     image source level includes D_ρ.  It is 0 for a direct path.
//   - abarBands[f], the barrier attenuation along the ray, computed only when
//     barriers is non-empty.  With no barriers it contributes nothing.
//
// rayOrigin is where the diffraction ray starts: the real subsegment source
// point for a direct path, the fully unfolded image source for a reflected one.
// It is read only when barriers is non-empty.
//
// waterFractionW is the fraction [0, 1] of the horizontal source–receiver path
// that crosses water bodies (Wasserflächen). It splits dp into a land portion
// and a water portion to compute Gl. 13: A_gr = A_gr,B + A_gr,W.
//
// groundOffsetM is the mean elevation of the ground along the path above the
// receiver's ground plane (Gl. 15, resolvePathGroundOffset).  It is resolved by
// the caller, once per subsegment→receiver path, because it does not depend on
// the Teilquelle height h and the loop below runs three times.
//
// The per-band accumulation is a plain float64 sum by design.  It runs over at
// most 3 Teilquelle heights × NumBeiblattOctaveBands terms — the fixed-length
// reduction docs/policies/determinism.md §3 exempts from compensated
// summation — while the per-subsegment sum above it, whose term count grows
// with the model, uses numeric.CompensatedSum.
func subsegmentContrib(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	rayOrigin geo.Point2D,
	dp, stepLen, sinDelta2, waterFractionW, dRho, groundOffsetM float64,
	barriers []BarrierSegment,
) float64 {
	dI := directivityDI(sinDelta2)
	log10Step := math.Log10(stepLen)

	// Gl. 13: A_gr = A_gr,B + A_gr,W
	// A_gr,B (Gl. 14) depends only on h_m and d; it is applied whenever the path
	// has a land portion.  A_gr,W (Gl. 16) uses d_w/d_p, the water share.
	dLand := (1.0 - waterFractionW) * dp
	dWater := waterFractionW * dp

	var contrib float64

	for _, h := range teilquelleHeightIndices {
		spectrum, present := emission.PerHeight[h]
		if !present {
			continue
		}

		path := newPathGeometry(elevationM+heightAboveSO[h], receiver.TerrainZ, receiver.HeightM, dp, groundOffsetM)
		adivVal := adiv(path.SlantDistanceM)

		// Gl. 13: A_gr = A_gr,B + A_gr,W.
		// A_gr,B applies only when there is a land path (d_land > 0).
		// A_gr,W is negative and applies only when there is a water path.
		agrVal := agrW(dWater, dp)
		if dLand > 0 {
			agrVal += agrB(path.MeanHeightM, path.SlantDistanceM)
		}

		// Barrier attenuation for this height level, along rayOrigin → receiver.
		// Stays zero — and costs no diffraction check — when the scene carries
		// no barriers.
		var abarBands BeiblattSpectrum

		if len(barriers) > 0 {
			var agrBands BeiblattSpectrum

			for f := range NumBeiblattOctaveBands {
				agrBands[f] = agrVal
			}

			// BarrierSegment.TopHeightM is a height above ground, so the path
			// ends go over in that same datum.
			abarBands = ComputePathBarrierAttenuation(
				rayOrigin, receiver.Point, path.SourceHeightM, path.ReceiverHeightM, barriers, agrBands,
			)
		}

		for f := range NumBeiblattOctaveBands {
			aatmVal := aatm(AirAbsorptionAlpha[f], path.SlantDistanceM)
			lW := spectrum[f] + 10*log10Step
			lpF := lW + dRho + dI + path.DOmega - adivVal - aatmVal - agrVal - abarBands[f]
			contrib += math.Pow(10, 0.1*lpF)
		}
	}

	return contrib
}

// normativeSubsegmentContrib returns the total linear acoustic power
// contribution from one track subsegment step to a receiver along the
// unshielded direct path: subsegmentContrib with no reflection loss (D_ρ = 0)
// and no barriers.
//
// waterFractionW is the fraction [0, 1] of the horizontal source–receiver path
// that crosses water bodies (Wasserflächen), for Gl. 13.  groundOffsetM is the
// ground under the path, per Gl. 15.
func normativeSubsegmentContrib(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	dp, stepLen, sinDelta2, waterFractionW, groundOffsetM float64,
) float64 {
	return subsegmentContrib(
		emission, elevationM, receiver, geo.Point2D{},
		dp, stepLen, sinDelta2, waterFractionW, 0, groundOffsetM,
		nil,
	)
}

// eachSubsegment splits every leg of a track centerline into integration steps
// of at most maxIntegrationStepM and calls visit once per step — in centerline
// order — with the step midpoint, the length it represents and the leg's axis
// vector, which Gl. 8 needs for the directivity angle δ.
//
// Degenerate legs (zero length, NaN or infinite) are skipped.  The visit order
// depends on the centerline alone, which is what lets every caller feed its
// numeric.CompensatedSum in a reproducible sequence.
//
// Once done is closed it visits no further step.  A subsegment is the finest
// unit the Anlage-2 chain owns, and on a long track with many facades one
// receiver is thousands of them, so this is where a cancelled run stops.  The
// sum the caller was feeding is then a partial one and must not become a
// level; computeNormativeReceiverLevels checks done after every walk and
// abandons the receiver instead.  A nil done never closes, so a walk that is
// not cancellable visits the same steps in the same order.
func eachSubsegment(done <-chan struct{}, centerline []geo.Point2D, visit func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64)) {
	for i := range len(centerline) - 1 {
		a := centerline[i]
		b := centerline[i+1]

		segLen := geo.Distance(a, b)
		if math.IsNaN(segLen) || math.IsInf(segLen, 0) || segLen <= 0 {
			continue
		}

		nsubs := max(int(math.Ceil(segLen/maxIntegrationStepM)), 1)
		stepLen := segLen / float64(nsubs)

		tvX := b.X - a.X
		tvY := b.Y - a.Y
		tvLen := math.Sqrt(tvX*tvX + tvY*tvY)

		for j := range nsubs {
			select {
			case <-done:
				return
			default:
			}

			frac := (float64(j) + 0.5) / float64(nsubs)
			pt := geo.Point2D{X: a.X + (b.X-a.X)*frac, Y: a.Y + (b.Y-a.Y)*frac}

			visit(pt, stepLen, tvX, tvY, tvLen)
		}
	}
}

// directRayTerms returns the horizontal source–receiver distance d_p, clamped
// to the 1 m near-field floor, and sin²(δ) for the direct ray leaving one
// subsegment midpoint.
func directRayTerms(receiver ReceiverInput, pt geo.Point2D, tvX, tvY, tvLen float64) (float64, float64) {
	rvX := receiver.Point.X - pt.X
	rvY := receiver.Point.Y - pt.Y
	dp := math.Sqrt(rvX*rvX + rvY*rvY)

	if dp < 1 {
		dp = 1
	}

	return dp, normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen)
}

// lineSourceLevel converts an accumulated linear acoustic power to a level in
// dB, mapping "collected no energy at all" to -Inf.
func lineSourceLevel(total float64) float64 {
	if total <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(total)
}

// normativeLineSourceLpAeq integrates the emission spectrum along a track
// centerline and returns the A-weighted equivalent continuous level L_pAeq at
// the receiver per the normative propagation chain (Gl. 6, 8-16).
//
// waterFractionW is forwarded to normativeSubsegmentContrib for Gl. 13.  dtm is
// the terrain model the run computes over, or nil; it is sampled once per
// subsegment for Gl. 15's ground term.  done stops the walk; see
// eachSubsegment.
func normativeLineSourceLpAeq(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	dtm terrain.Model,
	done <-chan struct{},
) float64 {
	var total numeric.CompensatedSum

	eachSubsegment(done, centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
		dp, sd2 := directRayTerms(receiver, pt, tvX, tvY, tvLen)
		groundOffsetM := resolvePathGroundOffset(dtm, pt, receiver)
		total.Add(normativeSubsegmentContrib(emission, elevationM, receiver, dp, stepLen, sd2, waterFractionW, groundOffsetM))
	})

	return lineSourceLevel(total.Sum())
}

// ComputeNormativeReceiverLevels computes L_pAeq and L_r for one receiver over
// all TrackSegments on the free-field path: no walls, no barriers, no terrain.
//
// K_S = 0 dB (Schienenbonus abolished since 2015 for Eisenbahnen), as for every
// other entry point here.
func ComputeNormativeReceiverLevels(
	receiver ReceiverInput,
	segments []TrackSegment,
) (NormativeReceiverLevels, error) {
	return ComputeNormativeReceiverLevelsForScene(receiver, NormativeScene{Segments: segments})
}

// normativeSubsegmentContribWithBarriers is like normativeSubsegmentContrib but
// includes barrier attenuation.  The barriers slice and subsegment source point
// are needed to compute the path-specific barrier geometry.
func normativeSubsegmentContribWithBarriers(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	sourcePoint geo.Point2D,
	dp, stepLen, sinDelta2, waterFractionW, groundOffsetM float64,
	barriers []BarrierSegment,
) float64 {
	if len(barriers) == 0 {
		return normativeSubsegmentContrib(emission, elevationM, receiver, dp, stepLen, sinDelta2, waterFractionW, groundOffsetM)
	}

	return subsegmentContrib(
		emission, elevationM, receiver, sourcePoint,
		dp, stepLen, sinDelta2, waterFractionW, 0, groundOffsetM,
		barriers,
	)
}

// normativeLineSourceLpAeqWithBarriers integrates emission along a track
// centerline with barrier attenuation on each subsegment's direct path.
func normativeLineSourceLpAeqWithBarriers(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	barriers []BarrierSegment,
	dtm terrain.Model,
	done <-chan struct{},
) float64 {
	if len(barriers) == 0 {
		return normativeLineSourceLpAeq(emission, centerline, elevationM, receiver, waterFractionW, dtm, done)
	}

	var total numeric.CompensatedSum

	eachSubsegment(done, centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
		dp, sd2 := directRayTerms(receiver, pt, tvX, tvY, tvLen)
		groundOffsetM := resolvePathGroundOffset(dtm, pt, receiver)
		total.Add(normativeSubsegmentContribWithBarriers(
			emission, elevationM, receiver, pt, dp, stepLen, sd2, waterFractionW, groundOffsetM, barriers,
		))
	})

	return lineSourceLevel(total.Sum())
}

// addDirectWithBarriersAndReflected computes the direct line-source contribution
// with barrier attenuation plus reflected contributions, and adds the linear
// power to *sum.
func addDirectWithBarriersAndReflected(
	emission *StreckeEmissionResult,
	seg TrackSegment,
	receiver ReceiverInput,
	walls []ReflectingWall,
	barriers []BarrierSegment,
	dtm terrain.Model,
	done <-chan struct{},
	sum *numeric.CompensatedSum,
) {
	lp := normativeLineSourceLpAeqWithBarriers(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, barriers, dtm, done,
	)
	if !math.IsInf(lp, -1) {
		sum.Add(math.Pow(10, 0.1*lp))
	}

	// Reflected paths (walls) with barrier attenuation on reflected paths.
	if len(walls) == 0 {
		return
	}

	reflLp := reflectedLineSourceLpAeqWithBarriers(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, walls, barriers, dtm, done,
	)
	if !math.IsInf(reflLp, -1) {
		sum.Add(math.Pow(10, 0.1*reflLp))
	}
}

// NormativeScene is everything the Anlage-2 chain computes over, beyond the
// receiver itself: the track segments that emit, the walls that reflect, the
// barrier panels that shield, and the ground it all stands on.
//
// It exists so that the ground can reach the propagation kernel at all.  The
// four older entry points take positional arguments and are called from about
// forty places, so growing each of them a terrain parameter would have been a
// change to every call site to say "no terrain" — and would have had to be made
// again for the next scene-wide input.  They are now thin wrappers that build a
// NormativeScene with no Terrain, which is exactly the flat-ground case they
// already computed.
//
// Terrain is optional and may be nil.  It is the imported DTM, in the CRS the
// run computes in; where it is absent, or does not reach a path, the ground
// falls back to the receiver's own plane (resolvePathGroundOffset).
type NormativeScene struct {
	Segments []TrackSegment
	Walls    []ReflectingWall
	Barriers []BarrierSegment
	Terrain  terrain.Model
}

// ComputeNormativeReceiverLevelsForScene computes L_pAeq and L_r for one
// receiver over a whole scene: direct paths, reflected paths (walls), barrier
// diffraction (barriers) and the ground beneath all of them (Gl. 15).
//
// It is the single normative propagation entry point; every other one in this
// file delegates to it.
func ComputeNormativeReceiverLevelsForScene(
	receiver ReceiverInput,
	scene NormativeScene,
) (NormativeReceiverLevels, error) {
	return computeNormativeReceiverLevels(context.Background(), receiver, scene)
}

// computeNormativeReceiverLevels is ComputeNormativeReceiverLevelsForScene
// under a context, checked inside the receiver: after every line-source walk,
// and at every subsegment of it (eachSubsegment).
//
// # Why inside a receiver and not only between receivers
//
// One receiver over a large scene — a long line, many train types, many
// facades to search for reflections — is seconds to minutes of CPU, and a
// check only between receivers left a cancelled run computing all of it.
// `aconiq serve` gives an interrupted run a grace period to record its own
// stop and kills it after that, so the run then read as killed, not stopped.
//
// # Why it cannot move a level
//
// The check is a non-blocking receive on ctx.Done() and reads nothing the walk
// computes. It either lets the next subsegment run exactly as before, or
// abandons the receiver, in which case no level is produced at all: a walk
// stopped part-way has fed daySum or nightSum a partial sum, and the check
// after that walk returns before either becomes a level. A context that can
// never be cancelled has a nil Done(), so the select is free and the walk is
// the same floating-point operations in the same order.
func computeNormativeReceiverLevels(
	ctx context.Context,
	receiver ReceiverInput,
	scene NormativeScene,
) (NormativeReceiverLevels, error) {
	daySum, nightSum, err := walkNormativeReceiver(ctx, receiver, scene)
	if err != nil {
		return NormativeReceiverLevels{}, err
	}

	lpAeqDay := math.Inf(-1)
	if daySum > 0 {
		lpAeqDay = 10 * math.Log10(daySum)
	}

	lpAeqNight := math.Inf(-1)
	if nightSum > 0 {
		lpAeqNight = 10 * math.Log10(nightSum)
	}

	const ks = 0.0

	return NormativeReceiverLevels{
		LpAeqDay:   lpAeqDay,
		LpAeqNight: lpAeqNight,
		LrDay:      beurteilungspegel(lpAeqDay, ks),
		LrNight:    beurteilungspegel(lpAeqNight, ks),
	}, nil
}

// walkNormativeReceiver is the walk behind computeNormativeReceiverLevels: it
// validates the receiver and the scene, splits the segments into speed-zone
// parts, and runs one line-source walk per (part, operation, period), day and
// night interleaved per operation, into one compensated sum per period. It
// returns the two linear sums, which computeNormativeReceiverLevels turns
// into levels.
//
// The validation and the split are repeated for every receiver; the
// estimate's probe times them as the receiver's one-off cost
// (normativeWalker.Begin), and its units are the line-source walks
// (addNormativeWalk), in this order.
func walkNormativeReceiver(ctx context.Context, receiver ReceiverInput, scene NormativeScene) (float64, float64, error) {
	done := ctx.Done()

	err := validateNormativeReceiverScene(receiver, scene)
	if err != nil {
		return 0, 0, err
	}

	var sums periodSums

	parts, err := prepareSpeedZoneParts(scene.Segments)
	if err != nil {
		return 0, 0, err
	}

	for _, part := range parts {
		for _, op := range part.segment.Operations {
			for _, period := range [2]normativePeriod{periodDay, periodNight} {
				err = addNormativeWalk(part, op, period, receiver, scene, done, &sums[period])
				if err != nil {
					return 0, 0, err
				}

				err = stopped(ctx, done)
				if err != nil {
					return 0, 0, err
				}
			}
		}
	}

	return sums[periodDay].Sum(), sums[periodNight].Sum(), nil
}

// normativePeriod is the planning period a line-source walk is run for. It
// indexes periodSums.
type normativePeriod int

const (
	periodDay normativePeriod = iota
	periodNight
)

// periodSums are a normative walk's two running sums of linear acoustic
// power, indexed by normativePeriod.
type periodSums [2]numeric.CompensatedSum

// normativeEmission is the Strecke emission of one operation on one
// speed-zone part for one period, refused in the walk's words.
func normativeEmission(part speedZonePart, op TrainOperation, period normativePeriod) (*StreckeEmissionResult, error) {
	trainsPerHour, name := op.TrainsPerHourDay, "day"
	if period == periodNight {
		trainsPerHour, name = op.TrainsPerHourNight, "night"
	}

	emission, err := ComputeStreckeEmission(buildVehicleInputs(part, op, trainsPerHour))
	if err != nil {
		return nil, fmt.Errorf("segment %q %s emission: %w", part.segment.ID, name, err)
	}

	return emission, nil
}

// addNormativeWalk is one line-source walk of the normative chain: the
// emission of one operation on one speed-zone part for one period, propagated
// to the receiver along the direct path and every reflected one, and added to
// *sum. It is the walk's unit of work, and the estimate's: both call it.
func addNormativeWalk(
	part speedZonePart,
	op TrainOperation,
	period normativePeriod,
	receiver ReceiverInput,
	scene NormativeScene,
	done <-chan struct{},
	sum *numeric.CompensatedSum,
) error {
	emission, err := normativeEmission(part, op, period)
	if err != nil {
		return err
	}

	addDirectWithBarriersAndReflected(emission, part.segment, receiver, scene.Walls, scene.Barriers, scene.Terrain, done, sum)

	return nil
}

// validateNormativeReceiverScene refuses a receiver or a scene the Anlage-2
// chain cannot compute over, before any of it is walked.
func validateNormativeReceiverScene(receiver ReceiverInput, scene NormativeScene) error {
	if len(scene.Segments) == 0 {
		return errors.New("at least one TrackSegment is required")
	}

	err := receiver.Validate()
	if err != nil {
		return err
	}

	for i, w := range scene.Walls {
		wallErr := w.Validate()
		if wallErr != nil {
			return fmt.Errorf("wall[%d]: %w", i, wallErr)
		}
	}

	for i, b := range scene.Barriers {
		barrierErr := b.Validate()
		if barrierErr != nil {
			return fmt.Errorf("barrier[%d]: %w", i, barrierErr)
		}
	}

	return nil
}

// stopped is the cancellation check of a walk inside one receiver: ctx's error
// once done is closed, nil otherwise, without blocking. It reads no number.
func stopped(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return fmt.Errorf("%w", ctx.Err())
	default:
		return nil
	}
}

// ComputeNormativeReceiverLevelsWithWalls computes L_pAeq and L_r including
// reflected path contributions from the given walls, over flat ground.
func ComputeNormativeReceiverLevelsWithWalls(
	receiver ReceiverInput,
	segments []TrackSegment,
	walls []ReflectingWall,
) (NormativeReceiverLevels, error) {
	return ComputeNormativeReceiverLevelsForScene(receiver, NormativeScene{Segments: segments, Walls: walls})
}

// ComputeNormativeReceiverLevelsWithScene computes L_pAeq and L_r including
// both reflected paths (walls) and barrier diffraction (barriers), over flat
// ground.  Pass a NormativeScene to ComputeNormativeReceiverLevelsForScene to
// put the path on real terrain.
func ComputeNormativeReceiverLevelsWithScene(
	receiver ReceiverInput,
	segments []TrackSegment,
	walls []ReflectingWall,
	barriers []BarrierSegment,
) (NormativeReceiverLevels, error) {
	return ComputeNormativeReceiverLevelsForScene(receiver, NormativeScene{
		Segments: segments,
		Walls:    walls,
		Barriers: barriers,
	})
}

// ReceiverOutput stores one computed receiver record.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsContext(context.Background(), receivers, sources, cfg)
}

// ComputeReceiverOutputsContext is ComputeReceiverOutputs under a context: once
// ctx is done it stops before the next receiver and returns ctx.Err().
func ComputeReceiverOutputsContext(ctx context.Context, receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsWithDataPackContext(ctx, receivers, sources, cfg, BuiltinDataPack())
}

// ComputeReceiverOutputsWithDataPack computes indicators using an explicit
// preview or external Schall 03 data pack.
func ComputeReceiverOutputsWithDataPack(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig, pack DataPack) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsWithDataPackContext(context.Background(), receivers, sources, cfg, pack)
}

// ComputeReceiverOutputsWithDataPackContext is ComputeReceiverOutputsWithDataPack
// under a context: once ctx is done it stops before the next source, inside
// the receiver it is computing, and returns ctx.Err().
func ComputeReceiverOutputsWithDataPackContext(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []RailSource,
	cfg PropagationConfig,
	pack DataPack,
) ([]ReceiverOutput, error) {
	return ComputeReceiverOutputsWithDataPackProgress(ctx, receivers, sources, cfg, pack, nil)
}

// ComputeReceiverOutputsWithDataPackProgress is
// ComputeReceiverOutputsWithDataPackContext that also reports (receivers done,
// receivers in total) after each receiver, the shape road.ProgressFunc
// documents. A nil progress reports nothing. The report is read by nothing in
// the walk, so it cannot move a level.
func ComputeReceiverOutputsWithDataPackProgress(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []RailSource,
	cfg PropagationConfig,
	pack DataPack,
	progress func(done, total int),
) ([]ReceiverOutput, error) {
	if err := requireReceivers(len(receivers)); err != nil {
		return nil, err
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		err = validatePreviewReceiver(receiver)
		if err != nil {
			return nil, err
		}

		levels, err := computeReceiverPeriodLevels(ctx, receiver.Point, sources, cfg, pack)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: levels.ToReceiverIndicators(),
		})

		if progress != nil {
			progress(len(outputs), len(receivers))
		}
	}

	return outputs, nil
}

// requireReceivers is both runs' refusal of an empty receiver list.
func requireReceivers(count int) error {
	if count == 0 {
		return errors.New("at least one receiver is required")
	}

	return nil
}

// checkNormativeInputs is the normative run's refusal of an empty receiver
// list or a scene without track, made before any receiver is walked.
func checkNormativeInputs(receivers int, scene NormativeScene) error {
	err := requireReceivers(receivers)
	if err != nil {
		return err
	}

	if len(scene.Segments) == 0 {
		return errors.New("at least one TrackSegment is required")
	}

	return nil
}

// validatePreviewReceiver is the preview run's check of one receiver record,
// made before the receiver is walked.
func validatePreviewReceiver(receiver geo.PointReceiver) error {
	if receiver.ID == "" {
		return errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
		return fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
	}

	return nil
}

// ComputeNormativeReceiverOutputsForScene runs the normative Anlage-2 chain for every
// receiver in order and returns the same ReceiverOutput shape the preview
// data-pack path produces, so both paths share one persistence layer.
//
// A receiver that collects no acoustic energy at all — every segment silent
// for that planning period — yields math.Inf(-1) from the level chain.  That
// value cannot be marshalled to JSON, so it is mapped to the silenceDB
// sentinel the rest of the tree already uses for "no contribution".
//
// Receivers arrive as ReceiverInput rather than geo.PointReceiver because each
// one carries the elevation of the ground it stands on, which geo has no field
// for and which every vertical propagation term is measured from.
func ComputeNormativeReceiverOutputsForScene(
	receivers []ReceiverInput,
	scene NormativeScene,
) ([]ReceiverOutput, error) {
	return ComputeNormativeReceiverOutputsForSceneContext(context.Background(), receivers, scene)
}

// ComputeNormativeReceiverOutputsForSceneContext is
// ComputeNormativeReceiverOutputsForScene under a context: once ctx is done it
// stops at the next subsegment, inside the receiver it is computing, and
// returns ctx.Err(). See computeNormativeReceiverLevels.
func ComputeNormativeReceiverOutputsForSceneContext(
	ctx context.Context,
	receivers []ReceiverInput,
	scene NormativeScene,
) ([]ReceiverOutput, error) {
	return ComputeNormativeReceiverOutputsForSceneProgress(ctx, receivers, scene, nil)
}

// ComputeNormativeReceiverOutputsForSceneProgress is
// ComputeNormativeReceiverOutputsForSceneContext that also reports (receivers
// done, receivers in total) after each receiver, the shape road.ProgressFunc
// documents. A nil progress reports nothing. The report is read by nothing in
// the walk, so it cannot move a level.
func ComputeNormativeReceiverOutputsForSceneProgress(
	ctx context.Context,
	receivers []ReceiverInput,
	scene NormativeScene,
	progress func(done, total int),
) ([]ReceiverOutput, error) {
	if err := checkNormativeInputs(len(receivers), scene); err != nil {
		return nil, err
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))

	for _, receiver := range receivers {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		levels, err := computeNormativeReceiverLevels(ctx, receiver, scene)
		if err != nil {
			return nil, fmt.Errorf("receiver %q: %w", receiver.ID, err)
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver: geo.PointReceiver{ID: receiver.ID, Point: receiver.Point, HeightM: receiver.HeightM},
			Indicators: ReceiverIndicators{
				LrDay:   finiteOrSilence(levels.LrDay),
				LrNight: finiteOrSilence(levels.LrNight),
			},
		})

		if progress != nil {
			progress(len(outputs), len(receivers))
		}
	}

	return outputs, nil
}

// ComputeNormativeReceiverOutputs is ComputeNormativeReceiverOutputsForScene
// over a scene with no terrain: every path is measured against the flat ground
// plane each receiver carries.
func ComputeNormativeReceiverOutputs(
	receivers []ReceiverInput,
	segments []TrackSegment,
	walls []ReflectingWall,
	barriers []BarrierSegment,
) ([]ReceiverOutput, error) {
	return ComputeNormativeReceiverOutputsForScene(receivers, NormativeScene{
		Segments: segments,
		Walls:    walls,
		Barriers: barriers,
	})
}

// finiteOrSilence replaces a -Inf level with the silenceDB sentinel.
func finiteOrSilence(level float64) float64 {
	if math.IsInf(level, -1) {
		return silenceDB
	}

	return level
}
