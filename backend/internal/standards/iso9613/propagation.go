package iso9613

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines the attenuation terms for ISO 9613-2 octave-band processing.
type PropagationConfig struct {
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	Barrier                 *BarrierGeometry
	C0                      float64
	MinDistanceM            float64
}

// DefaultPropagationConfig returns the default ISO 9613-2 propagation configuration.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		GroundFactor:            0.5,
		AirTemperatureC:         10,
		RelativeHumidityPercent: 70,
		MeteorologyAssumption:   MeteorologyDownwind,
		Barrier:                 nil,
		C0:                      0,
		MinDistanceM:            1,
	}
}

// Validate checks propagation inputs for sane ranges.
func (cfg PropagationConfig) Validate() error {
	// Each row carries its own predicate and message: the bounds differ per
	// field and so does the wording, which is user-visible.
	for _, check := range []struct {
		value   float64
		valid   func(float64) bool
		message string
	}{
		{cfg.GroundFactor, withinInclusive(0, 1), "ground_factor must be finite and within [0,1]"},
		{cfg.AirTemperatureC, withinInclusive(minAirTemperatureC, maxAirTemperatureC), fmt.Sprintf("air_temperature_c must be finite and within [%g,%g]", minAirTemperatureC, maxAirTemperatureC)},
		{cfg.RelativeHumidityPercent, withinInclusive(0, 100), "relative_humidity_percent must be finite and within [0,100]"},
	} {
		if !check.valid(check.value) {
			return errors.New(check.message)
		}
	}

	// Checked here, not with the rest of the numeric fields: this is where the
	// original sequence of ifs tested it, and the order decides which error a
	// config with several invalid fields reports.
	if cfg.MeteorologyAssumption != MeteorologyDownwind {
		return fmt.Errorf("meteorology_assumption must be %q", MeteorologyDownwind)
	}

	for _, check := range []struct {
		value   float64
		valid   func(float64) bool
		message string
	}{
		{cfg.C0, atLeast(0), "c0 must be finite and >= 0"},
		{cfg.MinDistanceM, greaterThan(0), "min_distance_m must be finite and > 0"},
	} {
		if !check.valid(check.value) {
			return errors.New(check.message)
		}
	}

	if cfg.Barrier != nil {
		err := cfg.Barrier.Validate()
		if err != nil {
			return fmt.Errorf("barrier: %w", err)
		}
	}

	return nil
}

// isFinite reports whether v is a real number: not NaN and not infinite.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func withinInclusive(minimum, maximum float64) func(float64) bool {
	return func(v float64) bool { return isFinite(v) && v >= minimum && v <= maximum }
}

func atLeast(minimum float64) func(float64) bool {
	return func(v float64) bool { return isFinite(v) && v >= minimum }
}

func greaterThan(minimum float64) func(float64) bool {
	return func(v float64) bool { return isFinite(v) && v > minimum }
}

func effectiveDistance(distanceM float64, cfg PropagationConfig) float64 {
	if distanceM < cfg.MinDistanceM {
		return cfg.MinDistanceM
	}

	return distanceM
}

func sourceDistance(receiver geo.PointReceiver, source PointSource) float64 {
	horizontal := geo.Distance(receiver.Point, source.Point)
	heightDelta := receiver.HeightM - source.SourceHeightM

	return math.Hypot(horizontal, heightDelta)
}

func geometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11
}

// BandAttenuation computes per-octave-band attenuation A(j) for one source-receiver path.
// Returns the 8-band attenuation and the effective source-receiver distance.
func BandAttenuation(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) (BandLevels, float64) {
	distance := effectiveDistance(sourceDistance(receiver, source), cfg)
	hs := source.SourceHeightM
	hr := receiver.HeightM
	dp := geo.Distance(receiver.Point, source.Point) // projected ground distance

	adiv := geometricDivergence(distance)
	aatm := AtmosphericAbsorptionBands(cfg.AirTemperatureC, cfg.RelativeHumidityPercent, distance)
	agr := GroundEffectBands(cfg.GroundFactor, cfg.GroundFactor, cfg.GroundFactor, hs, hr, dp)
	abar := BarrierAttenuationBands(cfg.Barrier, agr, 20)

	var totalAtten BandLevels
	for i := range NumBands {
		totalAtten[i] = adiv + aatm[i] + agr[i] + abar[i]
	}

	return totalAtten, distance
}

// sourcePressureRatio returns the A-weighted mean-square pressure ratio that
// one source contributes at the receiver, i.e. one summand of Eq. 5.
//
// A source with an explicit octave-band spectrum is evaluated band by band and
// A-weighted exactly once, as Eq. 5 prescribes. A source that carries only an
// A-weighted sound power level is evaluated with the 500 Hz attenuation terms
// per ISO 9613-2:1996 NOTE 1; its level is already A-weighted, so no further
// weighting is applied.
func sourcePressureRatio(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) float64 {
	atten, _ := BandAttenuation(receiver, source, cfg)

	bandLevels, hasSpectrum := EffectiveBandLevels(source)
	if !hasSpectrum {
		lat := EffectiveAWeightedPowerLevel(source) - atten[NoteOneBandIndex]

		return math.Pow(10, 0.1*lat)
	}

	sum := 0.0

	for j := range NumBands {
		lft := bandLevels[j] - atten[j]
		sum += math.Pow(10, 0.1*(lft+AWeighting[j]))
	}

	return sum
}

// levelFromPressureRatio converts a summed mean-square pressure ratio into a
// level, mapping a non-positive sum to the sentinel used for silent receivers.
func levelFromPressureRatio(sum float64) float64 {
	if sum <= 0 {
		return -999
	}

	return 10 * math.Log10(sum)
}

// ComputeDownwindLevel computes L_AT(DW) for one receiver from all sources (Eq. 5).
func ComputeDownwindLevel(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) float64 {
	dw, _ := ComputeDownwindAndLongTermLevels(receiver, sources, cfg)

	return dw
}

// ComputeDownwindAndLongTermLevels computes L_AT(DW) (Eq. 5) and L_AT(LT)
// (Eq. 6) for one receiver in a single pass over the sources.
//
// C_met is applied per source-receiver path before the energy summation.
// Clause 8 derives C_met from the source height h_s and the projected distance
// d_p of one point sound source (Eq. 21/22), so every path has its own value.
// Deriving a single C_met from the farthest source and subtracting it from the
// already summed level would over-correct every nearer source by up to C_0 dB.
//
// Sources are visited in slice order and every summand is independent of that
// order apart from float64 rounding, so the result is deterministic.
func ComputeDownwindAndLongTermLevels(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) (float64, float64) {
	dwSum := 0.0
	ltSum := 0.0

	for _, source := range sources {
		ratio := sourcePressureRatio(receiver, source, cfg)
		dwSum += ratio

		dp := geo.Distance(receiver.Point, source.Point)
		cmet := MeteorologicalCorrection(cfg.C0, source.SourceHeightM, receiver.HeightM, dp)
		ltSum += ratio * math.Pow(10, -0.1*cmet)
	}

	return levelFromPressureRatio(dwSum), levelFromPressureRatio(ltSum)
}

// MeteorologicalCorrection computes C_met from Eq. 21-22.
// c0 depends on local meteorological statistics; default 0 for pure downwind.
// hs is source height, hr is receiver height, dp is projected distance.
func MeteorologicalCorrection(c0, hs, hr, dp float64) float64 {
	limit := 10 * (hs + hr)
	if dp <= limit {
		return 0
	}

	return c0 * (1 - limit/dp)
}
