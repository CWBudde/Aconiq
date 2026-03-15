package schall03

import (
	"errors"
	"math"
)

// AreaPointContrib is a (octave-band level, count) pair for area aggregation
// (Gl. 5), representing q_i,h point sources of the same type.
type AreaPointContrib struct {
	Level BeiblattSpectrum // L_WA,f,h,i in dB
	Count float64          // q_i,h — number of point sources of this type
}

// AreaLineContrib is a (octave-band level, length, count) triplet for
// area aggregation (Gl. 5), representing q_j,h line sources.
type AreaLineContrib struct {
	Level   BeiblattSpectrum // L_W'A,f,h,j in dB
	LengthM float64          // l_j in metres
	Count   float64          // q_j,h — number of line sources of this type
}

// ComputePointSourceLevel computes L_WA,f,h,i per Gl. 3 for a single
// Einzelschallquelle (Punktschallquelle).
//
//	L_WA,f,h,i = L_WA,h,i + ΔL_W,f,h,i + 10·lg(n_i) + Σ K_k
//
// data.LWA is L_WA,h,i; data.DeltaLW contains ΔL_W,f,h,i.
// nPerHour is n_i (events per hour).
// kK is Σ K_k in dB (pass 0 if no corrections apply).
func ComputePointSourceLevel(data YardSourceData, nPerHour float64, kK float64) BeiblattSpectrum {
	lgN := 10.0 * math.Log10(math.Max(nPerHour, 1e-9))

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		result[f] = data.LWA + data.DeltaLW[f] + lgN + kK
	}

	return result
}

// ComputeLineSourceLevel computes L_W'A,f,h,j per Gl. 4 for a single
// Linienschallquelle.
//
//	L_W'A,f,h,j = L_W'A,h,j + ΔL_W',f,h,j + 10·lg(n_j) + Σ K_k
//
// nPerHour is n_j (events per hour).
// kK is Σ K_k in dB (pass 0 if none).
func ComputeLineSourceLevel(data YardSourceData, nPerHour float64, kK float64) BeiblattSpectrum {
	lgN := 10.0 * math.Log10(math.Max(nPerHour, 1e-9))

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		result[f] = data.LWA + data.DeltaLW[f] + lgN + kK
	}

	return result
}

// ComputeAreaSourceLevel computes the area-source level L_W”A,f,h per Gl. 5
// by aggregating point and line sources over a Teilfläche of area areaM2.
//
// pointSources contains the grouped point source contributions; lineSources
// the line source contributions.  areaM2 is S_F (m²) of the Teilfläche and
// must be > 0.  Either slice may be nil or empty.
func ComputeAreaSourceLevel(
	pointSources []AreaPointContrib,
	lineSources []AreaLineContrib,
	areaM2 float64,
) BeiblattSpectrum {
	const s0 = 1.0 // S_0 = 1 m²
	const l0 = 1.0 // l_0 = 1 m

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		sum := 0.0

		for _, ps := range pointSources {
			sum += ps.Count * math.Pow(10, 0.1*ps.Level[f])
		}

		for _, ls := range lineSources {
			sum += ls.Count * math.Pow(10, 0.1*ls.Level[f]) * (ls.LengthM / l0)
		}

		// Normalize by area to yield the area-specific level L_W''A (per m²)
		sum /= areaM2 / s0

		if sum > 0 {
			result[f] = 10 * math.Log10(sum)
		} else {
			result[f] = math.Inf(-1)
		}
	}

	return result
}

// LineToPointTeilstueck converts a line source spectrum to a Teilstück point
// level per Gl. 6:
//
//	L_WA,f,h,kS = L_W'A,f,h + 10·lg(l_kS / l_0)   with l_0 = 1 m
func LineToPointTeilstueck(lineSpectrum BeiblattSpectrum, lKS float64) BeiblattSpectrum {
	const l0 = 1.0
	offset := 10.0 * math.Log10(lKS/l0)

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		result[f] = lineSpectrum[f] + offset
	}

	return result
}

// AreaToPointTeilflaeche converts an area source spectrum to a Teilfläche point
// level per Gl. 7:
//
//	L_WA,f,h,kF = L_W''A,f,h + 10·lg(S_kF / S_0)   with S_0 = 1 m²
func AreaToPointTeilflaeche(areaSpectrum BeiblattSpectrum, sKF float64) BeiblattSpectrum {
	const s0 = 1.0
	offset := 10.0 * math.Log10(sKF/s0)

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		result[f] = areaSpectrum[f] + offset
	}

	return result
}

// YardPointImmissionInput describes the geometry for a single point source
// to a single receiver in a Rangierbahnhof (no directivity, C₂=20).
type YardPointImmissionInput struct {
	SourceLevel     BeiblattSpectrum // L_WA,f,h,i from Gl. 3 or Gl. 6/7
	SourceHeightM   float64          // h_g over ground level
	ReceiverDistM   float64          // horizontal distance source → receiver
	ReceiverHeightM float64          // h_r over ground
	WaterFractionW  float64          // fraction of path over water (0–1)
	BarrierGeom     *BarrierGeometry // nil = no barrier
}

// ComputeYardPointSourceImmission computes the L_p,Aeq contribution from a
// single Rangierbahnhof point source to one receiver (Gl. 30, one term).
//
// Key differences vs. Strecken propagation:
//   - No directivity D_I (yard sources radiate omnidirectionally)
//   - D_Ω (solid-angle ground reflection, Gl. 9) is retained — it applies to
//     all point sources near a reflective ground plane, independent of D_I
//   - Barrier diffraction uses C₂=20 via ComputeAbarYard
func ComputeYardPointSourceImmission(inp YardPointImmissionInput) (float64, error) {
	if inp.ReceiverDistM <= 0 {
		return 0, errors.New("receiver distance must be > 0")
	}

	dp := inp.ReceiverDistM
	dSlant := math.Sqrt(dp*dp + (inp.SourceHeightM-inp.ReceiverHeightM)*(inp.SourceHeightM-inp.ReceiverHeightM))

	if dSlant < 1 {
		dSlant = 1
	}

	dOmega := solidAngleDOmega(dp, inp.SourceHeightM, inp.ReceiverHeightM)
	adivVal := adiv(dSlant)

	hm := (inp.SourceHeightM + inp.ReceiverHeightM) / 2
	if hm < 0 {
		hm = 0
	}

	dLand := dp * (1 - inp.WaterFractionW)
	dWater := dp * inp.WaterFractionW

	// Compute A_gr once (frequency-independent per Gl. 13–16).
	agrVal := agrW(dWater, dp)
	if dLand > 0 {
		agrVal += agrB(hm, dSlant, dLand)
	}

	// Compute A_bar per band if barrier present.
	var aBarBands BeiblattSpectrum

	if inp.BarrierGeom != nil {
		var agrBandValues BeiblattSpectrum
		for f := range NumBeiblattOctaveBands {
			agrBandValues[f] = agrVal
		}

		aBarBands = ComputeAbarYard(*inp.BarrierGeom, agrBandValues)
	}

	var sum float64

	for f := range NumBeiblattOctaveBands {
		aAtmVal := aatm(AirAbsorptionAlpha[f], dSlant)
		atot := adivVal + aAtmVal + agrVal + aBarBands[f]
		// No D_I for yard sources (omnidirectional radiation).
		contrib := inp.SourceLevel[f] + dOmega - atot
		sum += math.Pow(10, 0.1*contrib)
	}

	if sum <= 0 {
		return math.Inf(-1), nil
	}

	return 10 * math.Log10(sum), nil
}
