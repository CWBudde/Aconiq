package talaerm

import "math"

// ComputeLr computes the Beurteilungspegel (rating level) Lr according to
// TA Lärm Anhang equation G2:
//
//	Lr = 10 · lg[ (1/Tr) · Σ(j=1..N) Tj · 10^(0.1 · (LAeq,j - Cmet + KT,j + KI,j + KR,j)) ]
//
// where Tr is the total reference duration (sum of all Tj), and cmet is the
// meteorological correction.
func ComputeLr(period AssessmentPeriod, teilzeiten []Teilzeit, cmet float64) (float64, error) {
	err := ValidateTeilzeiten(period, teilzeiten)
	if err != nil {
		return 0, err
	}

	tr := 0.0

	for _, tz := range teilzeiten {
		tr += tz.DurationH
	}

	sum := 0.0

	for _, tz := range teilzeiten {
		exponent := 0.1 * (tz.LAeq - cmet + tz.KT + tz.KI + tz.KR)
		sum += tz.DurationH * math.Pow(10, exponent)
	}

	lr := 10 * math.Log10(sum/tr)

	return lr, nil
}

// ComputeLrSimple computes the Beurteilungspegel for the common single-Teilzeit
// case where the entire assessment period has uniform emissions. When there is
// only one Teilzeit covering the full period, equation G2 simplifies to:
//
//	Lr = LAeq - Cmet + KT + KI + KR
func ComputeLrSimple(lAeq, cmet, kt, ki, kr float64) float64 {
	return lAeq - cmet + kt + ki + kr
}

// ComputeImpulsSurcharge computes the Impulshaltigkeit surcharge KI per
// TA Lärm equation G6 and Nr. A.2.5.3 / A.3.3.6:
//
//	KI = LAFTeq - LAeq
//
// The result is discretized into steps of 0, 3, or 6 dB:
//   - difference < 1.5:  KI = 0
//   - 1.5 <= difference < 4.5: KI = 3
//   - difference >= 4.5: KI = 6
func ComputeImpulsSurcharge(lAFTeq, lAeq float64) float64 {
	diff := lAFTeq - lAeq

	switch {
	case diff < 1.5:
		return 0
	case diff < 4.5:
		return 3
	default:
		return 6
	}
}
