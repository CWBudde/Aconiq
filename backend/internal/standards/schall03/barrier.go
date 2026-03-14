package schall03

import "math"

// speedOfSound is the reference speed of sound in m/s used for wavelength
// calculation in the barrier diffraction formulas.
const speedOfSound = 340.0

// c2Strecke is the constant C₂ = 40 for Eisenbahn Strecke (Gl. 21).
const c2Strecke = 40.0

// DzCapSingle is the maximum allowed D_z for a single diffraction edge (dB).
const DzCapSingle = 20.0

// DzCapDouble is the maximum allowed D_z for double diffraction (dB).
const DzCapDouble = 25.0

// BarrierGeometry holds the geometry parameters for one source–barrier–receiver
// propagation path, sufficient to compute Gl. 18-26.
type BarrierGeometry struct {
	// Ds is the source-to-barrier-top distance [m].
	Ds float64
	// Dr is the barrier-top-to-receiver distance [m].
	Dr float64
	// D is the direct (unobstructed) source–receiver distance [m].
	D float64
	// Z is the path length difference (ds+dr) - d [m].
	// Positive Z means the barrier adds path length.
	Z float64
	// E is the barrier thickness (distance between two edges for a wide
	// barrier / double barrier) [m].  0 for a thin single barrier.
	E float64
	// DPar is the component of path difference parallel to the track axis [m].
	// Used only for Gl. 25 (parallel barrier edges).
	DPar float64
	// Habs is the height of the absorbing surface at the barrier base [m].
	// Used for D_refl correction (Gl. 20).
	Habs float64
	// IsDouble is true when two separate barrier edges are present.
	IsDouble bool
	// TopDiffraction is true when the path goes over the barrier top rather
	// than around its lateral edge.
	TopDiffraction bool
}

// wavelength returns the acoustic wavelength for an octave-band centre
// frequency in Hz.
func wavelength(fm float64) float64 {
	return speedOfSound / fm
}

// barrierDz computes the screening attenuation per Gl. 21 using C₂=40
// (Eisenbahn Strecke).
//
//	D_z = 10·lg(3 + C₂/λ · C₃ · z · K_met)
//
// lambda: acoustic wavelength [m]
// c3:   multiple-diffraction factor C₃ (1.0 for single barrier)
// z:    path length difference [m]
// kmet: meteorological correction factor
//
// Returns 0 when z ≤ 0.
func barrierDz(lambda, c3, z, kmet float64) float64 {
	if z <= 0 {
		return 0
	}

	return 10.0 * math.Log10(3.0+c2Strecke/lambda*c3*z*kmet)
}

// kmet computes the meteorological correction factor per Gl. 23-24.
//
//	K_met = exp(-1/2000 · sqrt(ds·dr·d / (2·z)))  for z > 0
//	K_met = 1                                       for z ≤ 0
func kmet(ds, dr, d, z float64) float64 {
	if z <= 0 {
		return 1.0
	}

	return math.Exp(-1.0 / 2000.0 * math.Sqrt(ds*dr*d/(2.0*z)))
}

// pathDifferenceParallel computes z per Gl. 25 for parallel barrier edges.
//
//	z = sqrt((d_s + d_r + e)² + d_∥²) - d
//
// ds:   source-to-edge distance [m]
// dr:   edge-to-receiver distance [m]
// e:    barrier thickness [m]
// dPar: lateral (parallel-to-track) component of offset [m]
// d:    direct source–receiver distance [m]
func pathDifferenceParallel(ds, dr, e, dPar, d float64) float64 {
	total := ds + dr + e
	return math.Sqrt(total*total+dPar*dPar) - d
}

// pathDifferenceNonParallel computes z per Gl. 26 for non-parallel barrier
// edges (no lateral offset).
//
//	z = (d_s + d_r + e) - d
func pathDifferenceNonParallel(ds, dr, e, d float64) float64 {
	return ds + dr + e - d
}

// c3Multiple computes the additional screening factor C₃ for wide barriers
// with two diffraction edges per Gl. 22.
//
//	C₃ = (1 + (5λ/e)²) / (1/3 + (5λ/e)²)
func c3Multiple(lambda, e float64) float64 {
	if e <= 0 {
		return 1.0
	}

	ratio := 5.0 * lambda / e

	return (1.0 + ratio*ratio) / (1.0/3.0 + ratio*ratio)
}

// drefl computes the correction for reflective barriers with an absorbing base
// per Gl. 20.
//
//	D_refl = max(3 - h_abs/1m, 0) dB
func drefl(habs float64) float64 {
	return math.Max(3.0-habs, 0.0)
}

// ComputeAbar computes the barrier attenuation A_bar for a single propagation
// path per Gl. 18-19, returning frequency-dependent values across all 8
// Beiblatt octave bands.
//
// Gl. 18 (lateral diffraction):  A_bar = D_z              ≥ 0 dB
// Gl. 19 (top diffraction):      A_bar = D_z - D_refl - A_gr ≥ 0 dB
//
// agrBandValues supplies A_gr per octave band (for top diffraction paths).
// If geom.TopDiffraction is false, agrBandValues is ignored.
func ComputeAbar(geom BarrierGeometry, agrBandValues BeiblattSpectrum) BeiblattSpectrum {
	dzCap := DzCapSingle
	if geom.IsDouble {
		dzCap = DzCapDouble
	}

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		fm := BeiblattOctaveBandFrequencies[f]
		lam := wavelength(fm)

		c3 := 1.0
		if geom.IsDouble && geom.E > 0 {
			c3 = c3Multiple(lam, geom.E)
		}

		km := kmet(geom.Ds, geom.Dr, geom.D, geom.Z)
		dz := barrierDz(lam, c3, geom.Z, km)

		if dz > dzCap {
			dz = dzCap
		}

		var abar float64

		if geom.TopDiffraction {
			// Gl. 19: A_bar = D_z - D_refl - A_gr ≥ 0
			dr := drefl(geom.Habs)
			abar = math.Max(dz-dr-agrBandValues[f], 0)
		} else {
			// Gl. 18: A_bar = D_z ≥ 0
			abar = math.Max(dz, 0)
		}

		result[f] = abar
	}

	return result
}
