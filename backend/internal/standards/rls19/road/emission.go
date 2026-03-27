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
	sum := 0.0

	for _, vg := range AllVehicleGroups() {
		count := traffic.CountForGroup(vg)
		if count <= 0 {
			continue
		}

		speed := effectiveVehicleSpeed(source.Speeds, vg)

		// E6: per-vehicle sound power with corrections.
		lwA := computeVehicleSoundPower(source, vg)

		// E7 / Eq. 4: accumulate the per-group length-related power term
		// M_vg * 10^(0.1*L_WA,vg) / v_vg. This is algebraically equivalent
		// to the total-M + share_vg representation from RLS-19.
		sum += count * math.Pow(10, lwA/10) / speed
	}

	if sum <= 0 {
		return -999.0
	}

	// E7/EG: convert the Eq. 4 sum to dB(A)/m and add the
	// multiple-reflection surcharge (E5).
	total := 10*math.Log10(sum) - 30
	total += source.ReflectionSurchargeDB

	return total
}

func baseEmissionSpeed(speeds SpeedInput, vg VehicleGroup) float64 {
	if vg == Krad {
		// RLS-19 treats motorcycles with the Lkw2 base-emission curve, but at Pkw speed.
		return speeds.PkwKPH
	}

	return speeds.SpeedForGroup(vg)
}

func effectiveVehicleSpeed(speeds SpeedInput, vg VehicleGroup) float64 {
	return clampBaseEmissionSpeed(baseEmissionSpeed(speeds, vg), vg)
}

func clampBaseEmissionSpeed(speedKPH float64, vg VehicleGroup) float64 {
	base := baseEmissionTable[vg]
	v := speedKPH
	if v < base.VMin {
		v = base.VMin
	}

	if v > base.VMax {
		v = base.VMax
	}

	return v
}

// computeVehicleSoundPower computes the per-vehicle sound power level L_WA
// for a given vehicle group, including all corrections (E1-E4, E6).
func computeVehicleSoundPower(source RoadSource, vg VehicleGroup) float64 {
	// E1: Base emission (Grundwert) - speed-dependent.
	base := computeBaseEmission(effectiveVehicleSpeed(source.Speeds, vg), vg)

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
// given speed and vehicle group using the RLS-19 Eq. 6 coefficient model.
func computeBaseEmission(speedKPH float64, vg VehicleGroup) float64 {
	base := baseEmissionTable[vg]
	v := clampBaseEmissionSpeed(speedKPH, vg)
	return base.A + 10*math.Log10(1+math.Pow(v/base.B, base.C))
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
		speed := effectiveVehicleSpeed(source.Speeds, vg)
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
