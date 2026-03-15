package schall03

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// heightAboveSO maps Teilquelle height index h to metres above Schienenoberkante.
// h=1 → 0 m (rail level), h=2 → 4 m (pantograph), h=3 → 5 m (above pantograph).
var heightAboveSO = map[int]float64{1: 0, 2: 4, 3: 5}

// buildVehicleInputs converts a TrainOperation into VehicleInput records for
// one planning period using the provided trains-per-hour value.
func buildVehicleInputs(seg TrackSegment, op TrainOperation, trainsPerHour float64) StreckeEmissionInput {
	effectiveSpeed := resolveEffectiveSpeed(seg.StreckeMaxKPH, op.SpeedKPH, seg.IsStation)

	vehicles := make([]VehicleInput, 0, len(op.FzComposition))
	for _, fc := range op.FzComposition {
		vehicles = append(vehicles, VehicleInput{
			Fz:       fc.Fz,
			NPerHour: trainsPerHour * float64(fc.Count),
		})
	}

	return StreckeEmissionInput{
		Vehicles:     vehicles,
		SpeedKPH:     effectiveSpeed,
		Fahrbahn:     seg.Fahrbahn,
		SFahrbahn:    seg.SFahrbahn,
		Surface:      seg.Surface,
		BridgeType:   seg.BridgeType,
		BridgeMitig:  seg.BridgeMitig,
		CurveRadiusM: seg.CurveRadiusM,
	}
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

// normativeSubsegmentContrib returns the total linear acoustic power
// contribution from one track subsegment step to a receiver, summed over all
// height levels h and octave bands f (Gl. 6, 8-16).
//
// waterFractionW is the fraction [0, 1] of the horizontal source–receiver path
// that crosses water bodies (Wasserflächen). It splits dp into a land portion
// and a water portion to compute Gl. 13: A_gr = A_gr,B + A_gr,W.
func normativeSubsegmentContrib(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	dp, stepLen, sinDelta2, waterFractionW float64,
) float64 {
	dI := 10.0 * math.Log10(0.22+1.27*sinDelta2)
	log10Step := math.Log10(stepLen)

	// Gl. 13: A_gr = A_gr,B + A_gr,W
	// A_gr,B uses only the land portion of the path (Gl. 14: d_p = land path length).
	// A_gr,W uses the water portion and total path (Gl. 16).
	dLand := (1.0 - waterFractionW) * dp
	dWater := waterFractionW * dp

	var contrib float64

	for h, spectrum := range emission.PerHeight {
		hg := elevationM + heightAboveSO[h]
		hr := receiver.HeightM

		hm := (hg + hr) / 2
		if hm < 0 {
			hm = 0
		}

		dSlant := math.Sqrt(dp*dp + (hg-hr)*(hg-hr))
		if dSlant < 1 {
			dSlant = 1
		}

		dOmega := solidAngleDOmega(dp, hg, hr)
		adivVal := adiv(dSlant)

		// Gl. 13: A_gr = A_gr,B + A_gr,W.
		// A_gr,B applies only when there is a land path (d_land > 0).
		// A_gr,W is negative and applies only when there is a water path.
		agrVal := agrW(dWater, dp)
		if dLand > 0 {
			agrVal += agrB(hm, dSlant, dLand)
		}

		for f := range NumBeiblattOctaveBands {
			aatmVal := aatm(AirAbsorptionAlpha[f], dSlant)
			lW := spectrum[f] + 10*log10Step
			lpF := lW + dI + dOmega - adivVal - aatmVal - agrVal
			contrib += math.Pow(10, 0.1*lpF)
		}
	}

	return contrib
}

// normativeLineSourceLpAeq integrates the emission spectrum along a track
// centerline and returns the A-weighted equivalent continuous level L_pAeq at
// the receiver per the normative propagation chain (Gl. 6, 8-16).
//
// waterFractionW is forwarded to normativeSubsegmentContrib for Gl. 13.
func normativeLineSourceLpAeq(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
) float64 {
	var total float64

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

			rvX := receiver.Point.X - pt.X
			rvY := receiver.Point.Y - pt.Y
			dp := math.Sqrt(rvX*rvX + rvY*rvY)

			if dp < 1 {
				dp = 1
			}

			sd2 := normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen)
			total += normativeSubsegmentContrib(emission, elevationM, receiver, dp, stepLen, sd2, waterFractionW)
		}
	}

	if total <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(total)
}

// ComputeNormativeReceiverLevels computes L_pAeq and L_r for one receiver
// over all TrackSegments using the normative Gl. 1-2 + Gl. 8-16 + Gl. 33-34
// pipeline.  K_S = 0 dB (Schienenbonus abolished since 2015 for Eisenbahnen).
func ComputeNormativeReceiverLevels(
	receiver ReceiverInput,
	segments []TrackSegment,
) (NormativeReceiverLevels, error) {
	if len(segments) == 0 {
		return NormativeReceiverLevels{}, errors.New("at least one TrackSegment is required")
	}

	err := receiver.Validate()
	if err != nil {
		return NormativeReceiverLevels{}, err
	}

	var daySum, nightSum float64

	for si, seg := range segments {
		err = seg.Validate()
		if err != nil {
			return NormativeReceiverLevels{}, fmt.Errorf("segment[%d]: %w", si, err)
		}

		for _, op := range seg.Operations {
			dayEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourDay))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q day emission: %w", seg.ID, emitErr)
			}

			dayLp := normativeLineSourceLpAeq(dayEmission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW)

			if !math.IsInf(dayLp, -1) {
				daySum += math.Pow(10, 0.1*dayLp)
			}

			nightEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourNight))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q night emission: %w", seg.ID, emitErr)
			}

			nightLp := normativeLineSourceLpAeq(nightEmission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW)

			if !math.IsInf(nightLp, -1) {
				nightSum += math.Pow(10, 0.1*nightLp)
			}
		}
	}

	lpAeqDay := math.Inf(-1)
	if daySum > 0 {
		lpAeqDay = 10 * math.Log10(daySum)
	}

	lpAeqNight := math.Inf(-1)
	if nightSum > 0 {
		lpAeqNight = 10 * math.Log10(nightSum)
	}

	const ks = 0.0 // K_S abolished for Eisenbahnen since 2015

	return NormativeReceiverLevels{
		LpAeqDay:   lpAeqDay,
		LpAeqNight: lpAeqNight,
		LrDay:      beurteilungspegel(lpAeqDay, ks),
		LrNight:    beurteilungspegel(lpAeqNight, ks),
	}, nil
}

// normativeSubsegmentContribWithBarriers is like normativeSubsegmentContrib but
// includes barrier attenuation.  The barriers slice and subsegment source point
// are needed to compute the path-specific barrier geometry.
func normativeSubsegmentContribWithBarriers(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	sourcePoint geo.Point2D,
	dp, stepLen, sinDelta2, waterFractionW float64,
	barriers []BarrierSegment,
) float64 {
	if len(barriers) == 0 {
		return normativeSubsegmentContrib(emission, elevationM, receiver, dp, stepLen, sinDelta2, waterFractionW)
	}

	dI := 10.0 * math.Log10(0.22+1.27*sinDelta2)
	log10Step := math.Log10(stepLen)

	dLand := (1.0 - waterFractionW) * dp
	dWater := waterFractionW * dp

	var contrib float64

	for h, spectrum := range emission.PerHeight {
		hg := elevationM + heightAboveSO[h]
		hr := receiver.HeightM

		hm := (hg + hr) / 2
		if hm < 0 {
			hm = 0
		}

		dSlant := math.Sqrt(dp*dp + (hg-hr)*(hg-hr))
		if dSlant < 1 {
			dSlant = 1
		}

		dOmega := solidAngleDOmega(dp, hg, hr)
		adivVal := adiv(dSlant)

		agrVal := agrW(dWater, dp)
		if dLand > 0 {
			agrVal += agrB(hm, dSlant, dLand)
		}

		// Compute barrier attenuation for this height level.
		var agrBands BeiblattSpectrum
		for f := range NumBeiblattOctaveBands {
			agrBands[f] = agrVal
		}

		abarBands := ComputePathBarrierAttenuation(
			sourcePoint, receiver.Point, hg, hr, barriers, agrBands,
		)

		for f := range NumBeiblattOctaveBands {
			aatmVal := aatm(AirAbsorptionAlpha[f], dSlant)
			lW := spectrum[f] + 10*log10Step
			lpF := lW + dI + dOmega - adivVal - aatmVal - agrVal - abarBands[f]
			contrib += math.Pow(10, 0.1*lpF)
		}
	}

	return contrib
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
) float64 {
	if len(barriers) == 0 {
		return normativeLineSourceLpAeq(emission, centerline, elevationM, receiver, waterFractionW)
	}

	var total float64

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

			rvX := receiver.Point.X - pt.X
			rvY := receiver.Point.Y - pt.Y
			dp := math.Sqrt(rvX*rvX + rvY*rvY)

			if dp < 1 {
				dp = 1
			}

			sd2 := normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen)
			total += normativeSubsegmentContribWithBarriers(
				emission, elevationM, receiver, pt, dp, stepLen, sd2, waterFractionW, barriers,
			)
		}
	}

	if total <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(total)
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
	sum *float64,
) {
	lp := normativeLineSourceLpAeqWithBarriers(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, barriers,
	)
	if !math.IsInf(lp, -1) {
		*sum += math.Pow(10, 0.1*lp)
	}

	// Reflected paths (walls) — barriers on reflected paths deferred to Step 8.
	if len(walls) == 0 {
		return
	}

	reflLp := ComputeReflectedLineSourceLpAeq(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, walls,
	)
	if !math.IsInf(reflLp, -1) {
		*sum += math.Pow(10, 0.1*reflLp)
	}
}

// ComputeNormativeReceiverLevelsWithScene computes L_pAeq and L_r including
// both reflected paths (walls) and barrier diffraction (barriers).
func ComputeNormativeReceiverLevelsWithScene(
	receiver ReceiverInput,
	segments []TrackSegment,
	walls []ReflectingWall,
	barriers []BarrierSegment,
) (NormativeReceiverLevels, error) {
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

	var daySum, nightSum float64

	for si, seg := range segments {
		err = seg.Validate()
		if err != nil {
			return NormativeReceiverLevels{}, fmt.Errorf("segment[%d]: %w", si, err)
		}

		for _, op := range seg.Operations {
			dayEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourDay))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q day emission: %w", seg.ID, emitErr)
			}

			addDirectWithBarriersAndReflected(dayEmission, seg, receiver, walls, barriers, &daySum)

			nightEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourNight))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q night emission: %w", seg.ID, emitErr)
			}

			addDirectWithBarriersAndReflected(nightEmission, seg, receiver, walls, barriers, &nightSum)
		}
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

// addDirectAndReflected computes the direct and reflected line-source
// contributions for one emission result and adds the linear power to *sum.
func addDirectAndReflected(
	emission *StreckeEmissionResult,
	seg TrackSegment,
	receiver ReceiverInput,
	walls []ReflectingWall,
	sum *float64,
) {
	lp := normativeLineSourceLpAeq(emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW)
	if !math.IsInf(lp, -1) {
		*sum += math.Pow(10, 0.1*lp)
	}

	if len(walls) == 0 {
		return
	}

	reflLp := ComputeReflectedLineSourceLpAeq(
		emission, seg.TrackCenterline, seg.ElevationM, receiver, seg.WaterBodyFractionW, walls,
	)
	if !math.IsInf(reflLp, -1) {
		*sum += math.Pow(10, 0.1*reflLp)
	}
}

// ComputeNormativeReceiverLevelsWithWalls computes L_pAeq and L_r including
// reflected path contributions from the given walls.
func ComputeNormativeReceiverLevelsWithWalls(
	receiver ReceiverInput,
	segments []TrackSegment,
	walls []ReflectingWall,
) (NormativeReceiverLevels, error) {
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

	var daySum, nightSum float64

	for si, seg := range segments {
		err = seg.Validate()
		if err != nil {
			return NormativeReceiverLevels{}, fmt.Errorf("segment[%d]: %w", si, err)
		}

		for _, op := range seg.Operations {
			dayEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourDay))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q day emission: %w", seg.ID, emitErr)
			}

			addDirectAndReflected(dayEmission, seg, receiver, walls, &daySum)

			nightEmission, emitErr := ComputeStreckeEmission(buildVehicleInputs(seg, op, op.TrainsPerHourNight))
			if emitErr != nil {
				return NormativeReceiverLevels{}, fmt.Errorf("segment %q night emission: %w", seg.ID, emitErr)
			}

			addDirectAndReflected(nightEmission, seg, receiver, walls, &nightSum)
		}
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
