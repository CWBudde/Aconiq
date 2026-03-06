package rail

import "math"

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one rail source.
//
// This preview implementation separates rolling, traction, braking, and track
// infrastructure components so the baseline is easier to reason about and test.
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
	flow := trainFlowCorrection(traffic.TrainsPerHour)

	return energySumDB([]float64{
		rollingEmission(source, flow),
		tractionEmission(source, flow),
		brakingEmission(source, flow),
		infrastructureEmission(source, flow),
	})
}

func trainFlowCorrection(trainsPerHour float64) float64 {
	return 10 * math.Log10(trainsPerHour+1)
}

func rollingEmission(source RailSource, flow float64) float64 {
	return 43.0 +
		flow +
		speedCorrection(source.AverageTrainSpeedKPH) +
		roughnessCorrection(source.TrackRoughnessClass) +
		trackTypeCorrection(source.TrackType)
}

func tractionEmission(source RailSource, flow float64) float64 {
	return 38.0 +
		flow +
		tractionCorrection(source.TractionType) +
		tractionSpeedCorrection(source.AverageTrainSpeedKPH)
}

func brakingEmission(source RailSource, flow float64) float64 {
	return 35.0 + flow + brakingCorrection(source.BrakingShare)
}

func infrastructureEmission(source RailSource, flow float64) float64 {
	return 30.0 +
		flow +
		bridgeEmissionCorrection(source.OnBridge) +
		curveEmissionCorrection(source.CurveRadiusM)
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

func tractionSpeedCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 30 {
		clamped = 30
	}

	if clamped > 250 {
		clamped = 250
	}

	switch {
	case clamped <= 120:
		return 2 * math.Log10(clamped/90)
	default:
		return 0.5 + 1.5*math.Log10(clamped/120)
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

func trackTypeCorrection(trackType string) float64 {
	switch trackType {
	case TrackTypeBallasted:
		return 0
	case TrackTypeSlab:
		return 1.2
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

func bridgeEmissionCorrection(onBridge bool) float64 {
	if onBridge {
		return 1.5
	}

	return 0
}

func curveEmissionCorrection(curveRadiusM float64) float64 {
	if curveRadiusM <= 0 || curveRadiusM >= 500 {
		return 0
	}

	return ((500 - curveRadiusM) / 500) * 2.0
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
