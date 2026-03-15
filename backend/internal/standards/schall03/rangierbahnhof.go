package schall03

import "math"

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
// the line source contributions.  areaM2 is S_F (m²) of the Teilfläche.
// Either slice may be nil or empty.
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
