package schall03

import "math"

const (
	// silenceDB is the sentinel level used for "no acoustic contribution".
	// It matches the convention used by cnossos/*, bub/road and rls19.
	silenceDB = -999.0

	// silenceThresholdDB is the cut-off below which a level is treated as the
	// silence sentinel.
	silenceThresholdDB = -900.0
)

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
	// A period without trains emits nothing.  Without this branch the flow term
	// collapses to 10 lg(0 + 1) = 0 dB and the base, traction, roughness, train
	// class and track form spectra are still summed, so an empty period would
	// report the full bare spectrum.
	if traffic.TrainsPerHour <= 0 {
		return silentSpectrum()
	}

	flowCorrection := trainFlowCorrection(traffic.TrainsPerHour)
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

// silentSpectrum returns the per-band silence sentinel.  EnergeticSumLevels
// keeps it out of any sum in practice: 10^(-999/10) is 1e-100, a hundred orders
// of magnitude below any real contribution.
func silentSpectrum() OctaveSpectrum {
	var spectrum OctaveSpectrum
	for i := range spectrum {
		spectrum[i] = silenceDB
	}

	return spectrum
}

// trainFlowCorrection converts an hourly train flow into the 10 lg(n) energy
// term of Gl. 2.
//
// The zero-flow case is handled by an explicit branch in the caller rather than
// by the 10 lg(n + 1) shift that was used before: the shift adds a spurious
// +3.0 dB at n = 1 train/h and still misstates every other flow. This is the
// same correction that was applied to cnossos/road, cnossos/rail and bub/road.
func trainFlowCorrection(trainsPerHour float64) float64 {
	return 10 * math.Log10(trainsPerHour)
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
