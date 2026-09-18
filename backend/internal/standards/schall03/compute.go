package schall03

import (
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
func eachSubsegment(centerline []geo.Point2D, visit func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64)) {
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
// subsegment for Gl. 15's ground term.
func normativeLineSourceLpAeq(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	dtm terrain.Model,
) float64 {
	var total numeric.CompensatedSum

	eachSubsegment(centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
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
) float64 {
	if len(barriers) == 0 {
		return normativeLineSourceLpAeq(emission, centerline, elevationM, receiver, waterFractionW, dtm)
	}

	var total numeric.CompensatedSum

	eachSubsegment(centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
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
	sum *numeric.CompensatedSum,
) {
	lp := normativeLineSourceLpAeqWithBarriers(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, barriers, dtm,
	)
	if !math.IsInf(lp, -1) {
		sum.Add(math.Pow(10, 0.1*lp))
	}

	// Reflected paths (walls) with barrier attenuation on reflected paths.
	if len(walls) == 0 {
		return
	}

	reflLp := reflectedLineSourceLpAeqWithBarriers(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, walls, barriers, dtm,
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
	segments, walls, barriers := scene.Segments, scene.Walls, scene.Barriers

	if len(segments) == 0 {
		return NormativeReceiverLevels{}, errors.New("at least one TrackSegment is required")
	}

	err := receiver.Validate()
	if err != nil {
		return NormativeReceiverLevels{}, err
	}

	for i, w := range walls {
		wallErr := w.Validate()
		if wallErr != nil {
			return NormativeReceiverLevels{}, fmt.Errorf("wall[%d]: %w", i, wallErr)
		}
	}

	for i, b := range barriers {
		barrierErr := b.Validate()
		if barrierErr != nil {
			return NormativeReceiverLevels{}, fmt.Errorf("barrier[%d]: %w", i, barrierErr)
		}
	}

	var daySum, nightSum numeric.CompensatedSum

	parts, err := prepareSpeedZoneParts(segments)
	if err != nil {
		return NormativeReceiverLevels{}, err
	}

	for _, part := range parts {
		seg := part.segment

		for _, op := range seg.Operations {
			dayEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(part, op, op.TrainsPerHourDay))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q day emission: %w", seg.ID, emitErr)
			}

			addDirectWithBarriersAndReflected(dayEmission, seg, receiver, walls, barriers, scene.Terrain, &daySum)

			nightEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(part, op, op.TrainsPerHourNight))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q night emission: %w", seg.ID, emitErr)
			}

			addDirectWithBarriersAndReflected(nightEmission, seg, receiver, walls, barriers, scene.Terrain, &nightSum)
		}
	}

	lpAeqDay := math.Inf(-1)
	if daySum.Sum() > 0 {
		lpAeqDay = 10 * math.Log10(daySum.Sum())
	}

	lpAeqNight := math.Inf(-1)
	if nightSum.Sum() > 0 {
		lpAeqNight = 10 * math.Log10(nightSum.Sum())
	}

	const ks = 0.0

	return NormativeReceiverLevels{
		LpAeqDay:   lpAeqDay,
		LpAeqNight: lpAeqNight,
		LrDay:      beurteilungspegel(lpAeqDay, ks),
		LrNight:    beurteilungspegel(lpAeqNight, ks),
	}, nil
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
	return ComputeReceiverOutputsWithDataPack(receivers, sources, cfg, BuiltinDataPack())
}

// ComputeReceiverOutputsWithDataPack computes indicators using an explicit
// preview or external Schall 03 data pack.
func ComputeReceiverOutputsWithDataPack(receivers []geo.PointReceiver, sources []RailSource, cfg PropagationConfig, pack DataPack) ([]ReceiverOutput, error) {
	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))
	for _, receiver := range receivers {
		if receiver.ID == "" {
			return nil, errors.New("receiver id is required")
		}

		if !receiver.Point.IsFinite() {
			return nil, fmt.Errorf("receiver %q coordinates are not finite", receiver.ID)
		}

		levels, err := ComputeReceiverPeriodLevelsWithDataPack(receiver.Point, sources, cfg, pack)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, ReceiverOutput{
			Receiver:   receiver,
			Indicators: levels.ToReceiverIndicators(),
		})
	}

	return outputs, nil
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
	segments := scene.Segments

	if len(receivers) == 0 {
		return nil, errors.New("at least one receiver is required")
	}

	if len(segments) == 0 {
		return nil, errors.New("at least one TrackSegment is required")
	}

	outputs := make([]ReceiverOutput, 0, len(receivers))

	for _, receiver := range receivers {
		levels, err := ComputeNormativeReceiverLevelsForScene(receiver, scene)
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
