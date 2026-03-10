package schall03

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

const maxIntegrationStepM = 10.0

// PropagationConfig defines the baseline Schall 03 preview attenuation chain.
type PropagationConfig struct {
	AirAbsorptionDBPerKM  float64
	GroundAttenuationDB   float64
	SlabTrackCorrectionDB float64
	BridgeCorrectionDB    float64
	CurveCorrectionDB     float64
	MinDistanceM          float64
}

// DefaultPropagationConfig returns default baseline propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return BuiltinDataPack().Propagation.DefaultConfig
}

func (cfg PropagationConfig) Validate() error {
	for _, item := range []struct {
		name string
		v    float64
		min  float64
	}{
		{"air_absorption_db_per_km", cfg.AirAbsorptionDBPerKM, 0},
		{"ground_attenuation_db", cfg.GroundAttenuationDB, 0},
		{"slab_track_correction_db", cfg.SlabTrackCorrectionDB, 0},
		{"bridge_correction_db", cfg.BridgeCorrectionDB, 0},
		{"curve_correction_db", cfg.CurveCorrectionDB, 0},
		{"min_distance_m", cfg.MinDistanceM, 0.0000001},
	} {
		if math.IsNaN(item.v) || math.IsInf(item.v, 0) || item.v < item.min {
			return errors.New(item.name + " must be finite and >= 0")
		}
	}

	return nil
}

func attenuation(distanceM float64, bandIdx int, cfg PropagationConfig, pack DataPack) float64 {
	d := distanceM
	if d < cfg.MinDistanceM {
		d = cfg.MinDistanceM
	}

	geometric := 20*math.Log10(d) + 11.0
	air := cfg.AirAbsorptionDBPerKM * pack.Propagation.AirAbsorptionBandFactor[bandIdx] * (d / 1000.0)

	return geometric + air + cfg.GroundAttenuationDB
}

func sourceAdjustment(source RailSource, cfg PropagationConfig) float64 {
	adjustment := 0.0
	if source.Infrastructure.TrackType == TrackTypeSlab {
		adjustment += cfg.SlabTrackCorrectionDB
	}

	if source.Infrastructure.OnBridge {
		adjustment += cfg.BridgeCorrectionDB
	}

	if source.Infrastructure.CurveRadiusM > 0 && source.Infrastructure.CurveRadiusM < 500 {
		severity := (500 - source.Infrastructure.CurveRadiusM) / 500
		adjustment += severity * cfg.CurveCorrectionDB
	}

	return adjustment
}

func lineSourceSpectrumAtReceiver(sourceSpectrum OctaveSpectrum, receiver geo.Point2D, source RailSource, cfg PropagationConfig, pack DataPack) OctaveSpectrum {
	var bandContribs [8][]float64
	adjustment := sourceAdjustment(source, cfg)

	for i := range len(source.TrackCenterline) - 1 {
		a := source.TrackCenterline[i]
		b := source.TrackCenterline[i+1]

		length := geo.Distance(a, b)
		if math.IsNaN(length) || math.IsInf(length, 0) || length <= 0 {
			continue
		}

		subsegments := max(int(math.Ceil(length/maxIntegrationStepM)), 1)
		stepLength := length / float64(subsegments)

		for j := range subsegments {
			fraction := (float64(j) + 0.5) / float64(subsegments)
			point := geo.Point2D{
				X: a.X + (b.X-a.X)*fraction,
				Y: a.Y + (b.Y-a.Y)*fraction,
			}
			distance := geo.Distance(receiver, point)

			for bandIdx := range sourceSpectrum {
				a := attenuation(distance, bandIdx, cfg, pack)
				level := sourceSpectrum[bandIdx] + 10*math.Log10(stepLength) - a + adjustment
				bandContribs[bandIdx] = append(bandContribs[bandIdx], level)
			}
		}
	}

	var result OctaveSpectrum
	for bandIdx := range result {
		result[bandIdx] = EnergeticSumLevels(bandContribs[bandIdx]...)
	}

	return result
}

// ComputeReceiverPeriodLevels computes day/night levels at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig) (PeriodLevels, error) {
	return ComputeReceiverPeriodLevelsWithDataPack(receiver, sources, cfg, BuiltinDataPack())
}

// ComputeReceiverPeriodLevelsWithDataPack computes day/night levels using an
// explicit preview or external Schall 03 data pack.
func ComputeReceiverPeriodLevelsWithDataPack(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig, pack DataPack) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if err := pack.Validate(); err != nil {
		return PeriodLevels{}, err
	}

	if !receiver.IsFinite() {
		return PeriodLevels{}, errors.New("receiver is not finite")
	}

	if len(sources) == 0 {
		return PeriodLevels{}, errors.New("at least one source is required")
	}

	daySpectra := make([]OctaveSpectrum, 0, len(sources))
	nightSpectra := make([]OctaveSpectrum, 0, len(sources))

	for _, source := range sources {
		emission, err := ComputeEmissionWithDataPack(source, pack)
		if err != nil {
			return PeriodLevels{}, err
		}

		daySpectra = append(daySpectra, lineSourceSpectrumAtReceiver(emission.DaySpectrum, receiver, source, cfg, pack))
		nightSpectra = append(nightSpectra, lineSourceSpectrumAtReceiver(emission.NightSpectrum, receiver, source, cfg, pack))
	}

	day := SumSpectra(daySpectra).EnergeticTotal()
	night := SumSpectra(nightSpectra).EnergeticTotal()

	return PeriodLevels{LrDay: day, LrNight: night}, nil
}
