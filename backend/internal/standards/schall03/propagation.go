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

// BeiblattOctaveBandFrequencies are the octave-band centre frequencies (Hz)
// in the canonical order 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
// Used by the barrier diffraction module.
var BeiblattOctaveBandFrequencies = [NumBeiblattOctaveBands]float64{
	63, 125, 250, 500, 1000, 2000, 4000, 8000,
}

// ---------------------------------------------------------------------------
// Normative propagation functions (Anlage 2 zu §4 der 16. BImSchV)
// ---------------------------------------------------------------------------

// adiv computes geometric divergence per Gl. 11.
//
//	A_div = 10·lg(4π·d²/d₀²) with d₀ = 1 m.
func adiv(d float64) float64 {
	return 10.0 * math.Log10(4.0*math.Pi*d*d)
}

// aatm computes air absorption per Gl. 12 for a single octave band.
//
//	A_atm = α·d/1000
func aatm(alpha, d float64) float64 {
	return alpha * d / 1000.0
}

// agrB computes ground absorption over land per Gl. 14.
//
//	A_gr,B = [4.8 - (2·h_m/d)·(17 + 300·d_p/d)] ≥ 0 dB
//
// h_m: mean height of propagation path above ground [m],
// d:   source–receiver distance [m],
// d_p: length of propagation path over land [m].
func agrB(hm, d, dp float64) float64 {
	val := 4.8 - (2.0*hm/d)*(17.0+300.0*dp/d)
	return math.Max(val, 0.0)
}

// agrW computes the water-body ground correction per Gl. 16.
//
//	A_gr,W = -3·d_w/d_p
//
// d_w: length of propagation path over water [m],
// d_p: total horizontal source–receiver distance [m].
func agrW(dw, dp float64) float64 {
	if dp == 0 {
		return 0
	}

	return -3.0 * dw / dp
}

// directivityDI computes the directivity correction per Gl. 8.
//
//	D_I = 10·lg(0.22 + 1.27·sin²(δ))
//
// δ is the angle between the perpendicular to the track axis and the
// source-to-receiver direction.
func directivityDI(delta float64) float64 {
	sinD := math.Sin(delta)
	return 10.0 * math.Log10(0.22+1.27*sinD*sinD)
}

// solidAngleDOmega computes the solid-angle correction per Gl. 9.
//
//	D_Ω = 10·lg(1 + (d_p² + (h_g-h_r)²) / (d_p² + (h_g+h_r)²))
//
// d_p: horizontal source–receiver distance [m],
// h_g: source height above ground [m],
// h_r: receiver height above ground [m].
func solidAngleDOmega(dp, hg, hr float64) float64 {
	num := dp*dp + (hg-hr)*(hg-hr)
	den := dp*dp + (hg+hr)*(hg+hr)

	if den == 0 {
		return 0
	}

	return 10.0 * math.Log10(1.0+num/den)
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

	err = pack.Validate()
	if err != nil {
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
