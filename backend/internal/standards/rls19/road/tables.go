package road

import "math"

// Normative coefficient tables for RLS-19 road emission.
//
// These are structured as data so they can be replaced by an external
// "standards data pack" without code changes.

// BaseEmissionCoeffs holds the speed-dependent base emission formula
// coefficients (Grundwert) per vehicle group.
// Formula (RLS-19 Eq. 6): L_W0,FzG(v) = A + 10*lg(1 + (v/B)^C).
type BaseEmissionCoeffs struct {
	A    float64 // constant term [dB(A)]
	B    float64 // speed scale [km/h]
	C    float64 // speed exponent
	VMin float64 // minimum clamped speed [km/h]
	VMax float64 // maximum clamped speed [km/h]
}

// baseEmissionTable holds coefficients per vehicle group.
// Index: VehicleGroup (Pkw=0, Lkw1=1, Lkw2=2, Krad=3).
var baseEmissionTable = [4]BaseEmissionCoeffs{
	Pkw:  {A: 88.0, B: 20.0, C: 3.06, VMin: 30, VMax: 130},
	Lkw1: {A: 100.3, B: 40.0, C: 4.33, VMin: 30, VMax: 130},
	Lkw2: {A: 105.4, B: 50.0, C: 4.88, VMin: 30, VMax: 130},
	Krad: {A: 105.4, B: 50.0, C: 4.88, VMin: 30, VMax: 130},
}

// SurfaceCorrectionEntry holds the Table 4 surface correction values.
type SurfaceCorrectionEntry struct {
	PkwLow  float64
	PkwHigh float64
	LkwLow  float64
	LkwHigh float64

	Paving30 float64
	Paving40 float64
	Paving50 float64

	UsesPavingThresholds bool
	LegacyPerVehicle     [4]float64
	UsesLegacyPerVehicle bool
}

func notApplicableSurfaceCorrection() float64 {
	return math.NaN()
}

func bandedSurfaceCorrection(pkwLow, pkwHigh, lkwLow, lkwHigh float64) SurfaceCorrectionEntry {
	return SurfaceCorrectionEntry{PkwLow: pkwLow, PkwHigh: pkwHigh, LkwLow: lkwLow, LkwHigh: lkwHigh}
}

func pavingSurfaceCorrection(v30, v40, v50 float64) SurfaceCorrectionEntry {
	return SurfaceCorrectionEntry{Paving30: v30, Paving40: v40, Paving50: v50, UsesPavingThresholds: true}
}

func legacyVehicleSurfaceCorrection(values [4]float64) SurfaceCorrectionEntry {
	return SurfaceCorrectionEntry{LegacyPerVehicle: values, UsesLegacyPerVehicle: true}
}

// surfaceCorrectionTable maps surface type to Table 4 correction values.
// Generic legacy surface identifiers are kept as aliases for backward compatibility.
var surfaceCorrectionTable = map[SurfaceType]SurfaceCorrectionEntry{
	SurfaceSMA:                 bandedSurfaceCorrection(-2.6, -1.8, -1.8, -2.0),
	SurfaceSMA5_8:              bandedSurfaceCorrection(-2.6, notApplicableSurfaceCorrection(), -1.8, notApplicableSurfaceCorrection()),
	SurfaceSMA8_11:             bandedSurfaceCorrection(notApplicableSurfaceCorrection(), -1.8, notApplicableSurfaceCorrection(), -2.0),
	SurfaceAB:                  bandedSurfaceCorrection(-2.7, -1.9, -1.9, -2.1),
	SurfaceOPA:                 bandedSurfaceCorrection(notApplicableSurfaceCorrection(), -4.5, notApplicableSurfaceCorrection(), -4.4),
	SurfaceOPA11:               bandedSurfaceCorrection(notApplicableSurfaceCorrection(), -4.5, notApplicableSurfaceCorrection(), -4.4),
	SurfaceOPA8:                bandedSurfaceCorrection(notApplicableSurfaceCorrection(), -5.5, notApplicableSurfaceCorrection(), -5.4),
	SurfacePaving:              pavingSurfaceCorrection(5.0, 6.0, 7.0),
	SurfacePavingEven:          pavingSurfaceCorrection(1.0, 2.0, 3.0),
	SurfacePavingOther:         pavingSurfaceCorrection(5.0, 6.0, 7.0),
	SurfaceConcrete:            bandedSurfaceCorrection(-1.4, -1.4, -2.3, -2.3),
	SurfaceLOA:                 bandedSurfaceCorrection(-3.2, notApplicableSurfaceCorrection(), -1.0, notApplicableSurfaceCorrection()),
	SurfaceSMALA8:              bandedSurfaceCorrection(notApplicableSurfaceCorrection(), -2.8, notApplicableSurfaceCorrection(), -4.6),
	SurfaceDSHV:                bandedSurfaceCorrection(-3.9, -2.8, -0.9, -2.3),
	SurfaceGussasphalt:         bandedSurfaceCorrection(-2.0, -2.0, -1.5, -1.5),
	SurfaceGussasphaltStandard: bandedSurfaceCorrection(0.0, 0.0, 0.0, 0.0),
	// Legacy non-normative fallback retained until this input category is revisited.
	SurfaceUnpavedOrDamaged: legacyVehicleSurfaceCorrection([4]float64{4.0, 4.0, 2.0, 3.0}),
}

// SurfaceCorrection returns the DStrO correction for a given surface type
// and vehicle group at the given speed. Returns 0 if the surface is unknown
// or if the selected Table 4 cell is not applicable for that speed range.
func SurfaceCorrection(st SurfaceType, vg VehicleGroup, speedKPH float64) float64 {
	entry, ok := surfaceCorrectionTable[st]
	if !ok {
		return 0
	}

	if entry.UsesLegacyPerVehicle {
		return entry.LegacyPerVehicle[vg]
	}

	if entry.UsesPavingThresholds {
		switch {
		case speedKPH <= 30:
			return entry.Paving30
		case speedKPH <= 40:
			return entry.Paving40
		default:
			return entry.Paving50
		}
	}

	correction := entry.LkwHigh
	if vg == Pkw || vg == Krad {
		correction = entry.PkwHigh
		if speedKPH <= 60 {
			correction = entry.PkwLow
		}
	} else if speedKPH <= 60 {
		correction = entry.LkwLow
	}

	if math.IsNaN(correction) {
		return 0
	}

	return correction
}

// GradientCorrection computes the Längsneigungskorrektur D_LN for a road
// segment with the given gradient, vehicle group, and speed.
//
// RLS-19 Section 3.3.6, Equations 7a / 7b / 7c.
// Gradients outside [–12 %, +12 %] are clamped to the boundary values.
// Krad is treated identically to Pkw (Eq. 7a with Pkw speed).
func GradientCorrection(gradientPercent float64, vg VehicleGroup, speedKPH float64) float64 {
	g := gradientPercent
	if g > 12 {
		g = 12
	}

	if g < -12 {
		g = -12
	}

	switch vg {
	case Pkw, Krad:
		// Eq. 7a
		if g < -6 {
			return (g + 6) / (-6) * (90 - math.Min(speedKPH, 70)) / 20
		}

		if g > 2 {
			return (g - 2) / 10 * (speedKPH + 70) / 100
		}

		return 0

	case Lkw1:
		// Eq. 7b
		if g < -4 {
			return (g + 4) / (-8) * (speedKPH - 20) / 10
		}

		if g > 2 {
			return (g - 2) / 10 * speedKPH / 10
		}

		return 0

	case Lkw2:
		// Eq. 7c
		if g < -4 {
			return (g + 4) / (-8) * (speedKPH - 10) / 10
		}

		if g > 2 {
			return (g - 2) / 10 * (speedKPH + 10) / 10
		}

		return 0

	default:
		return 0
	}
}

// kktTable holds the K_KT maximum correction values per junction type (Tabelle 5).
var kktTable = map[JunctionType]float64{
	JunctionSignalized: 3,
	JunctionRoundabout: 2,
	JunctionOther:      0,
}

// JunctionCorrection returns the Knotenpunktkorrektur D_KKT for a junction
// type KT at distance x from the nearest junction.
//
// RLS-19 Section 3.3.7, Equation 8:
//
//	D_KKT(x) = K_KT · max(1 – x/120, 0)
func JunctionCorrection(jt JunctionType, distanceM float64) float64 {
	kkt, ok := kktTable[jt]
	if !ok || jt == JunctionNone {
		return 0
	}

	factor := 1 - distanceM/120
	if factor < 0 {
		factor = 0
	}

	return kkt * factor
}

// PropagationConstants holds physical constants for the propagation model.
var PropagationConstants = struct {
	// AirAbsorptionCoeff is the broadband A-weighted air absorption coefficient
	// in dB/km per RLS-19 Eq. 13: D_atm = α·s/1000 = 5·s/1000 = s/200.
	// Value 5.0 dB/km is the RLS-19 simplified annual-average approximation
	// (not a frequency-specific ISO 9613-2 value).
	AirAbsorptionCoeff float64
	// ReferenceDistance is the reference distance for geometric divergence [m].
	ReferenceDistance float64
}{
	AirAbsorptionCoeff: 5.0,
	ReferenceDistance:  1.0,
}
