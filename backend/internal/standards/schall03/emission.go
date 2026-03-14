package schall03

import "math"

type periodEmission struct {
	DaySpectrum   OctaveSpectrum
	NightSpectrum OctaveSpectrum
}

// ComputeEmission computes day/night source spectra for one rail source.
func ComputeEmission(source RailSource) (periodEmission, error) {
	return ComputeEmissionWithDataPack(source, BuiltinDataPack())
}

// ComputeEmissionWithDataPack computes day/night spectra using an explicit
// preview or external Schall 03 data pack.
func ComputeEmissionWithDataPack(source RailSource, pack DataPack) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	err = pack.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		DaySpectrum:   emissionSpectrumForPeriod(source, source.TrafficDay, pack),
		NightSpectrum: emissionSpectrumForPeriod(source, source.TrafficNight, pack),
	}, nil
}

func emissionSpectrumForPeriod(source RailSource, traffic TrafficPeriod, pack DataPack) OctaveSpectrum {
	flowCorrection := 10 * math.Log10(traffic.TrainsPerHour+1)
	speedCorrection := rollingSpeedCorrection(source.AverageSpeedKPH, pack.Emission.SpeedModel)

	lengthCorrection := 0.0
	if length := sourceSegmentLengthM(source.TrackCenterline); length > 0 {
		lengthCorrection = 10 * math.Log10(length/100.0)
	}

	traction := pack.Emission.TractionSpectra[source.Infrastructure.TractionType]
	roughness := pack.Emission.RoughnessSpectra[source.Infrastructure.TrackRoughnessClass]
	trainClass := pack.Emission.TrainClassSpectra[source.TrainClass]
	trackForm := pack.Emission.TrackFormSpectra[source.Infrastructure.TrackForm]

	var spectrum OctaveSpectrum
	for i := range spectrum {
		spectrum[i] = pack.Emission.BaseRollingSpectrum[i] + traction[i] + roughness[i] + trainClass[i] + trackForm[i] + flowCorrection + speedCorrection + lengthCorrection
	}

	return spectrum
}

func rollingSpeedCorrection(speedKPH float64, model SpeedModel) float64 {
	clamped := speedKPH
	if clamped < model.MinSpeedKPH {
		clamped = model.MinSpeedKPH
	}

	if clamped > model.MaxSpeedKPH {
		clamped = model.MaxSpeedKPH
	}

	switch {
	case clamped < model.LowSpeedThresholdKPH:
		return model.LowOffsetDB + model.LowSlope*math.Log10(clamped/model.LowSpeedThresholdKPH)
	case clamped <= model.HighSpeedThresholdKPH:
		return model.MidSlope * math.Log10(clamped/model.MidReferenceKPH)
	default:
		return model.HighOffsetDB + model.HighSlope*math.Log10(clamped/model.HighSpeedThresholdKPH)
	}
}
