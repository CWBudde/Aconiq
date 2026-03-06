package schall03

import "math"

type periodEmission struct {
	DaySpectrum   OctaveSpectrum
	NightSpectrum OctaveSpectrum
}

var (
	baseRollingSpectrum       = OctaveSpectrum{73, 76, 80, 84, 87, 85, 81, 76}
	tractionElectricSpectrum  = OctaveSpectrum{3, 3, 2, 1, 0, -1, -2, -3}
	tractionDieselSpectrum    = OctaveSpectrum{5, 5, 4, 3, 1, 0, -1, -2}
	tractionMixedSpectrum     = OctaveSpectrum{4, 4, 3, 2, 1, -0.5, -1.5, -2.5}
	roughnessLowNoiseSpectrum = OctaveSpectrum{0, -0.5, -1, -1.5, -2, -2, -2, -2}
	roughnessStandardSpectrum = OctaveSpectrum{0, 0, 0, 0, 0, 0, 0, 0}
	roughnessRoughSpectrum    = OctaveSpectrum{0.5, 1, 1.5, 2, 2.5, 2.5, 2, 1.5}
)

// ComputeEmission computes day/night source spectra for one rail source.
func ComputeEmission(source RailSource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		DaySpectrum:   emissionSpectrumForPeriod(source, source.TrafficDay),
		NightSpectrum: emissionSpectrumForPeriod(source, source.TrafficNight),
	}, nil
}

func emissionSpectrumForPeriod(source RailSource, traffic TrafficPeriod) OctaveSpectrum {
	flowCorrection := 10 * math.Log10(traffic.TrainsPerHour+1)
	speedCorrection := rollingSpeedCorrection(source.AverageSpeedKPH)

	lengthCorrection := 0.0
	if length := sourceSegmentLengthM(source.TrackCenterline); length > 0 {
		lengthCorrection = 10 * math.Log10(length/100.0)
	}

	traction := tractionSpectrum(source.Infrastructure.TractionType)
	roughness := roughnessSpectrum(source.Infrastructure.TrackRoughnessClass)

	var spectrum OctaveSpectrum
	for i := range spectrum {
		spectrum[i] = baseRollingSpectrum[i] + traction[i] + roughness[i] + flowCorrection + speedCorrection + lengthCorrection
	}

	return spectrum
}

func rollingSpeedCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 30 {
		clamped = 30
	}

	if clamped > 250 {
		clamped = 250
	}

	switch {
	case clamped < 80:
		return -1.5 + 8*math.Log10(clamped/80.0)
	case clamped <= 160:
		return 9 * math.Log10(clamped/100.0)
	default:
		return 1.8 + 6*math.Log10(clamped/160.0)
	}
}

func tractionSpectrum(kind string) OctaveSpectrum {
	switch kind {
	case TractionElectric:
		return tractionElectricSpectrum
	case TractionDiesel:
		return tractionDieselSpectrum
	case TractionMixed:
		return tractionMixedSpectrum
	default:
		return tractionMixedSpectrum
	}
}

func roughnessSpectrum(class string) OctaveSpectrum {
	switch class {
	case RoughnessLowNoise:
		return roughnessLowNoiseSpectrum
	case RoughnessRough:
		return roughnessRoughSpectrum
	case RoughnessStandard:
		return roughnessStandardSpectrum
	default:
		return roughnessStandardSpectrum
	}
}
