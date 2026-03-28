package iso9613

import "math"

// BarrierGeometry holds pre-computed diffraction path geometry.
type BarrierGeometry struct {
	Dss float64 // distance from source to first diffraction edge (m)
	Dsr float64 // distance from last diffraction edge to receiver (m)
	E   float64 // distance between first and last diffraction edge, 0 for single (m)
	A   float64 // component distance parallel to barrier edge (m)
	D   float64 // direct source-to-receiver distance (m)
}

// IsDouble returns true if this represents double diffraction (e > 0).
func (g BarrierGeometry) IsDouble() bool {
	return g.E > 0
}

// pathDifference computes z from Eq. 16 (single) or Eq. 17 (double).
func pathDifference(g BarrierGeometry) float64 {
	pathSum := g.Dss + g.Dsr + g.E
	return math.Sqrt(pathSum*pathSum+g.A*g.A) - g.D
}

// c3Factor computes C_3 from Eq. 15.
// For single diffraction (e=0), C_3 = 1.
// For double diffraction, C_3 = [1+(5λ/e)²] / [(1/3)+(5λ/e)²].
func c3Factor(e, freqHz float64) float64 {
	if e <= 0 {
		return 1
	}

	lambda := Wavelength(freqHz)
	ratio := 5 * lambda / e
	r2 := ratio * ratio

	return (1 + r2) / (1.0/3.0 + r2)
}

// kMet computes K_met from Eq. 18.
func kMet(g BarrierGeometry, z float64) float64 {
	if z <= 0 {
		return 1
	}

	return math.Exp(-(1.0 / 2000.0) * math.Sqrt(g.Dss*g.Dsr*g.D/(2*z)))
}

// BarrierDz computes the barrier attenuation D_z (Eq. 14) for one octave band.
// c2 is 20 when ground reflections are included, 40 when handled by image sources.
func BarrierDz(g BarrierGeometry, z, freqHz, c2 float64) float64 {
	if z <= 0 {
		return 0
	}

	lambda := Wavelength(freqHz)
	c3 := c3Factor(g.E, freqHz)
	km := kMet(g, z)

	dz := 10 * math.Log10(3+(c2/lambda)*c3*z*km)

	maxDz := 20.0
	if g.IsDouble() {
		maxDz = 25.0
	}

	if dz > maxDz {
		return maxDz
	}

	return dz
}

// BarrierAttenuationBands computes A_bar per octave band (Eq. 12).
// groundAtten is A_gr for the unscreened path (subtracted per Eq. 12).
// Returns zero bands if geometry is nil (no barrier).
func BarrierAttenuationBands(g *BarrierGeometry, groundAtten BandLevels, c2 float64) BandLevels {
	var result BandLevels
	if g == nil {
		return result
	}

	z := pathDifference(*g)

	for i := range NumBands {
		dz := BarrierDz(*g, z, OctaveBandFrequencies[i], c2)

		abar := dz - groundAtten[i]
		if abar < 0 {
			abar = 0
		}

		result[i] = abar
	}

	return result
}
