package rail

import "math"

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one rail source.
//
// This preview implementation combines rolling, traction, and braking noise
// terms with period train-flow weighting.
func ComputeEmission(source RailSource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		Lday:     emissionForPeriod(source, source.TrafficDay),
		Levening: emissionForPeriod(source, source.TrafficEvening),
		Lnight:   emissionForPeriod(source, source.TrafficNight),
	}, nil
}

func emissionForPeriod(source RailSource, traffic TrafficPeriod) float64 {
	baseFlow := 10 * math.Log10(traffic.TrainsPerHour+1)
	rolling := 43.0 + baseFlow + speedCorrection(source.AverageTrainSpeedKPH) + roughnessCorrection(source.TrackRoughnessClass)
	traction := 38.0 + baseFlow + tractionCorrection(source.TractionType)
	braking := 35.0 + baseFlow + brakingCorrection(source.BrakingShare)

	return energySumDB([]float64{rolling, traction, braking})
}

func speedCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 30 {
		clamped = 30
	}

	if clamped > 250 {
		clamped = 250
	}

	switch {
	case clamped < 80:
		return -1.0 + 6*math.Log10(clamped/80)
	case clamped <= 160:
		return 8 * math.Log10(clamped/100)
	default:
		return 3 + 5*math.Log10(clamped/160)
	}
}

func roughnessCorrection(class string) float64 {
	switch class {
	case RoughnessSmooth:
		return -1.5
	case RoughnessStandard:
		return 0
	case RoughnessRough:
		return 2.5
	default:
		return 0
	}
}

func tractionCorrection(traction string) float64 {
	switch traction {
	case TractionElectric:
		return -1.0
	case TractionDiesel:
		return 1.5
	case TractionMixed:
		return 0.5
	default:
		return 0
	}
}

func brakingCorrection(share float64) float64 {
	return 10 * math.Log10(1+4*share)
}

func energySumDB(levels []float64) float64 {
	sum := 0.0

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) {
			continue
		}

		sum += math.Pow(10, level/10)
	}

	if sum <= 0 {
		return -999.0
	}

	return 10 * math.Log10(sum)
}
