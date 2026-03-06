package road

import "math"

// EmissionResult holds the emission level per time period for one source.
type EmissionResult struct {
	LmEDay   float64 // length-related sound power level, day [dB(A)/m]
	LmENight float64 // length-related sound power level, night [dB(A)/m]
}

// VehicleGroupEmission holds intermediate emission results for one vehicle group.
type VehicleGroupEmission struct {
	Group           VehicleGroup
	BaseLevel       float64 // E1: Grundwert L_m,E
	SurfaceCorr     float64 // E2: DStrO correction
	GradientCorr    float64 // E3: Gradient correction D_Stg
	JunctionCorr    float64 // E4: Junction correction D_KP
	SoundPowerLevel float64 // E6: Per-vehicle L_WA with all corrections
}

// ComputeEmission computes period emissions (E1-E7) for one RLS-19 road source.
//
// Steps:
//
//	E1 - Base emission (speed-dependent Grundwert per vehicle group)
//	E2 - Surface correction (DStrO)
//	E3 - Gradient correction (vehicle-group dependent)
//	E4 - Junction correction (type + distance dependent)
//	E5 - Multiple-reflection surcharge (input as pre-computed value)
//	E6 - Per-vehicle sound power with all additive corrections
//	E7 - Length-related sound power level per period (day/night)
func ComputeEmission(source RoadSource) (EmissionResult, error) {
	err := source.Validate()
	if err != nil {
		return EmissionResult{}, err
	}

	return EmissionResult{
		LmEDay:   emissionForPeriod(source, source.TrafficDay),
		LmENight: emissionForPeriod(source, source.TrafficNight),
	}, nil
}

func emissionForPeriod(source RoadSource, traffic TrafficInput) float64 {
	groups := AllVehicleGroups()
	contributions := make([]float64, 0, len(groups))

	for _, vg := range groups {
		count := traffic.CountForGroup(vg)
		if count <= 0 {
			continue
		}

		// E6: per-vehicle sound power with corrections
		lwA := computeVehicleSoundPower(source, vg)

		// E7: length-related sound power level
		// L_m,E,vg = L_WA,vg + 10*lg(M_vg) - 30  [for hourly count M]
		// The -30 converts from per-vehicle to per-meter-per-hour
		// (accounting for reference speed normalization).
		lmE := lwA + 10*math.Log10(count)

		contributions = append(contributions, lmE)
	}

	if len(contributions) == 0 {
		return -999.0
	}

	// E7/EG: energetic sum of all vehicle group contributions,
	// plus the multiple-reflection surcharge (E5).
	total := energySumDB(contributions)
	total += source.ReflectionSurchargeDB

	return total
}

// computeVehicleSoundPower computes the per-vehicle sound power level L_WA
// for a given vehicle group, including all corrections (E1-E4, E6).
func computeVehicleSoundPower(source RoadSource, vg VehicleGroup) float64 {
	// E1: Base emission (Grundwert) - speed-dependent.
	base := computeBaseEmission(source.Speeds.SpeedForGroup(vg), vg)

	// E2: Surface correction (DStrO).
	surfCorr := SurfaceCorrection(source.SurfaceType, vg)

	// E3: Gradient correction.
	gradCorr := GradientCorrection(source.GradientPercent, vg)

	// E4: Junction correction.
	juncCorr := JunctionCorrection(source.JunctionType, source.JunctionDistanceM)

	// E6: Sound power = base + surface + gradient + junction.
	return base + surfCorr + gradCorr + juncCorr
}

// computeBaseEmission computes the E1 base emission (Grundwert) for a
// given speed and vehicle group using the rolling + propulsion noise model.
func computeBaseEmission(speedKPH float64, vg VehicleGroup) float64 {
	roll := rollingNoiseTable[vg]
	prop := propulsionNoiseTable[vg]
	base := baseEmissionTable[vg]

	// Clamp speed to valid range.
	v := speedKPH
	if v < base.VMin {
		v = base.VMin
	}

	if v > base.VMax {
		v = base.VMax
	}

	lgVRoll := math.Log10(v / roll.VRef)
	lgVProp := math.Log10(v / prop.VRef)

	rollLevel := roll.A + roll.B*lgVRoll
	propLevel := prop.A + prop.B*lgVProp

	// Energetic sum of rolling and propulsion noise.
	return energySumDB([]float64{rollLevel, propLevel})
}

// energySumDB performs an energetic summation of dB(A) values.
func energySumDB(levels []float64) float64 {
	sum := 0.0

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) || level <= -900 {
			continue
		}

		sum += math.Pow(10, level/10)
	}

	if sum <= 0 {
		return -999.0
	}

	return 10 * math.Log10(sum)
}

// ComputeVehicleGroupEmissions returns detailed per-vehicle-group emission
// breakdown for diagnostic/reporting purposes.
func ComputeVehicleGroupEmissions(source RoadSource) ([]VehicleGroupEmission, error) {
	err := source.Validate()
	if err != nil {
		return nil, err
	}

	groups := AllVehicleGroups()
	results := make([]VehicleGroupEmission, 0, len(groups))

	for _, vg := range groups {
		speed := source.Speeds.SpeedForGroup(vg)
		base := computeBaseEmission(speed, vg)
		surfCorr := SurfaceCorrection(source.SurfaceType, vg)
		gradCorr := GradientCorrection(source.GradientPercent, vg)
		juncCorr := JunctionCorrection(source.JunctionType, source.JunctionDistanceM)

		results = append(results, VehicleGroupEmission{
			Group:           vg,
			BaseLevel:       base,
			SurfaceCorr:     surfCorr,
			GradientCorr:    gradCorr,
			JunctionCorr:    juncCorr,
			SoundPowerLevel: base + surfCorr + gradCorr + juncCorr,
		})
	}

	return results, nil
}
