package schall03

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// FahrbahnartSchwellengleis represents the reference track type (Schwellengleis)
// which carries no c1 corrections.  It is intentionally set to a value that
// does not match any entry in C1FahrbahnartTable.
const FahrbahnartSchwellengleis FahrbahnartType = -1

// SurfaceCondType identifies active surface condition measures for Table 8.
type SurfaceCondType int

const (
	SurfaceCondNone                 SurfaceCondType = iota // no surface correction
	SurfaceCondBuG                                         // besonders ueberwachtes Gleis
	SurfaceCondSchienenstegdaempf                          // Schienenstegdaempfer
	SurfaceCondSchienenstegabschirm                        // Schienenstegabschirmung
)

// StreckeEmissionInput holds all parameters needed to compute emission for one
// track segment per Gl. 1-2 of Anlage 2 zu §4 der 16. BImSchV.
type StreckeEmissionInput struct {
	// Train composition: slice of (FzKategorie number, n_Fz trains per hour).
	Vehicles []VehicleInput
	// Operating speed in km/h.
	SpeedKPH float64
	// Track infrastructure.
	Fahrbahn     FahrbahnartType // from tables.go
	Surface      SurfaceCondType // from this file
	BridgeType   int             // 0=none, 1-4 per Table 9
	BridgeMitig  bool            // K_LM applies
	CurveRadiusM float64         // 0 = straight, >0 = curved
}

// VehicleInput describes one Fahrzeug-Kategorie contribution to a train.
type VehicleInput struct {
	Fz        int     // Fz-Kategorie (1-10)
	NPerHour  float64 // Fahrzeugeinheiten per hour of this Fz type
	AxleCount int     // actual axle count (0 = use n_Achs,0 default)
}

// StreckeEmissionResult contains emission levels per height level for one period.
type StreckeEmissionResult struct {
	// PerHeight maps height index h (1, 2, 3) to the combined octave-band
	// sound power level L_W'A,f,h across all Fz types and Teilquellen at that
	// height.
	PerHeight map[int]BeiblattSpectrum
}

// v0 is the reference speed in km/h.
const v0 = 100.0

// ComputeStreckeEmission computes the normative emission per Gl. 1-2 of
// Anlage 2 zu §4 der 16. BImSchV (Schall 03).
func ComputeStreckeEmission(input StreckeEmissionInput) (*StreckeEmissionResult, error) {
	err := validateEmissionInput(input)
	if err != nil {
		return nil, err
	}

	fzMap := buildFzMap()

	// Collect per-band linear power contributions per height.
	// heights: 1 (0m SO), 2 (4m SO), 3 (5m SO).
	heightSums := map[int][NumBeiblattOctaveBands]float64{}

	for _, vi := range input.Vehicles {
		fz, ok := fzMap[vi.Fz]
		if !ok {
			return nil, fmt.Errorf("unknown Fz-Kategorie %d", vi.Fz)
		}

		nFz := vi.NPerHour

		for _, tq := range fz.Teilquellen {
			nQ := vi.AxleCount
			nQ0 := fz.NAchs0

			if nQ <= 0 {
				nQ = nQ0
			}

			level := computeTeilquelleLevel(
				tq, nQ, nQ0, input.SpeedKPH,
				input.Fahrbahn, input.Surface,
				input.BridgeType, input.BridgeMitig, input.CurveRadiusM,
			)

			h := tq.HeightH
			sums := heightSums[h]

			for f := range NumBeiblattOctaveBands {
				// Gl. 2: n_Fz * 10^(0.1 * L_W'A,f,h,m,Fz)
				sums[f] += nFz * math.Pow(10, 0.1*level[f]) //nolint:gosec // f bounded by NumBeiblattOctaveBands = len(sums)
			}

			heightSums[h] = sums
		}
	}

	result := &StreckeEmissionResult{
		PerHeight: make(map[int]BeiblattSpectrum, len(heightSums)),
	}

	for h, sums := range heightSums {
		var spectrum BeiblattSpectrum

		for f := range NumBeiblattOctaveBands {
			if sums[f] > 0 { //nolint:gosec // f bounded by NumBeiblattOctaveBands = len(sums)
				spectrum[f] = 10 * math.Log10(sums[f])
			} else {
				spectrum[f] = math.Inf(-1)
			}
		}

		result.PerHeight[h] = spectrum
	}

	return result, nil
}

// computeTeilquelleLevel computes L_W'A,f,h,m,Fz per Gl. 1 for one Teilquelle.
func computeTeilquelleLevel(
	tq Teilquelle,
	nQ, nQ0 int,
	speedKPH float64,
	fahrbahn FahrbahnartType,
	surface SurfaceCondType,
	bridgeType int,
	bridgeMitig bool,
	curveRadiusM float64,
) BeiblattSpectrum {
	b := SpeedFactorBForTeilquelle(tq.M)

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		L := tq.AA + tq.DeltaA[f] // a_A + Δa_f

		// Axle correction: only for rolling noise (m=1,2,3,4).
		if isRollingNoise(tq.M) && nQ > 0 && nQ0 > 0 {
			L += 10.0 * math.Log10(float64(nQ)/float64(nQ0))
		}

		// Speed correction: b_f * lg(v/v0).
		L += b[f] * math.Log10(speedKPH/v0)

		// Fahrbahn correction c1.
		L += sumC1ForTeilquelle(fahrbahn, tq.M, f)

		// Surface condition correction c2.
		L += sumC2ForTeilquelle(surface, tq.M, f)

		// Bridge correction K_Br (+ K_LM).
		L += bridgeCorrectionForTeilquelle(bridgeType, bridgeMitig, tq.M)

		// Curve correction K_L.
		L += curveCorrectionForTeilquelle(curveRadiusM, tq.M)

		result[f] = L
	}

	return result
}

// isRollingNoise returns true for Teilquellen m=1,2,3,4.
func isRollingNoise(m int) bool {
	return m >= 1 && m <= 4
}

// sumC1ForTeilquelle returns the total c1 correction from Table 7 for a given
// Fahrbahnart, Teilquelle m and octave band index f.
func sumC1ForTeilquelle(fahrbahn FahrbahnartType, m, f int) float64 {
	// Schwellengleis (default) has no c1 corrections — the Fahrbahn types in
	// the table are deviations from the reference.  Only the three types
	// encoded in C1FahrbahnartTable carry corrections.
	for i := range C1FahrbahnartTable {
		if C1FahrbahnartTable[i].Type != fahrbahn {
			continue
		}

		total := 0.0

		for _, corr := range C1FahrbahnartTable[i].Corrections {
			if slices.Contains(corr.Teilquellen, m) {
				total += corr.C1[f]
			}
		}

		return total
	}

	return 0
}

// sumC2ForTeilquelle returns the total c2 correction from Table 8 for a given
// surface condition, Teilquelle m and octave band index f.
func sumC2ForTeilquelle(surface SurfaceCondType, m, f int) float64 {
	if surface == SurfaceCondNone {
		return 0
	}

	// Map SurfaceCondType to C2MeasureType.
	var measure C2MeasureType

	switch surface {
	case SurfaceCondBuG:
		measure = C2BuG
	case SurfaceCondSchienenstegdaempf:
		measure = C2Schienenstegdaempfer
	case SurfaceCondSchienenstegabschirm:
		measure = C2Schienenstegabschirmung
	default:
		return 0
	}

	total := 0.0

	for _, entry := range C2SurfaceConditionTable {
		if entry.Measure == measure && slices.Contains(entry.Teilquellen, m) {
			total += entry.C2[f]
		}
	}

	return total
}

// bridgeCorrectionForTeilquelle returns the K_Br (+K_LM) bridge correction for
// a given bridge type and Teilquelle m.  Bridge corrections apply only to
// rolling noise at track level (m=1,2).
func bridgeCorrectionForTeilquelle(bridgeType int, bridgeMitig bool, m int) float64 {
	if bridgeType <= 0 || bridgeType > len(BridgeCorrectionTable) {
		return 0
	}

	// Bridge correction applies to rolling noise only (m=1,2).
	if m != 1 && m != 2 {
		return 0
	}

	entry := BridgeCorrectionTable[bridgeType-1]
	correction := entry.KBr

	if bridgeMitig && !math.IsNaN(entry.KLM) {
		correction += entry.KLM
	}

	return correction
}

// curveCorrectionForTeilquelle returns K_L for the given curve radius and
// Teilquelle m.  Curve noise correction applies only to rolling noise (m=1,2).
func curveCorrectionForTeilquelle(curveRadiusM float64, m int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	kL, _ := CurveNoiseCorrectionForRadius(curveRadiusM)

	return kL
}

// buildFzMap builds a lookup map from Fz number to *FzKategorie.
func buildFzMap() map[int]*FzKategorie {
	m := make(map[int]*FzKategorie, len(FzKategorien))
	for i := range FzKategorien {
		m[FzKategorien[i].Fz] = &FzKategorien[i]
	}

	return m
}

// validateEmissionInput checks the emission input for basic validity.
func validateEmissionInput(input StreckeEmissionInput) error {
	if input.SpeedKPH <= 0 {
		return fmt.Errorf("speed must be > 0, got %g km/h", input.SpeedKPH)
	}

	if math.IsNaN(input.SpeedKPH) || math.IsInf(input.SpeedKPH, 0) {
		return fmt.Errorf("speed must be finite, got %g km/h", input.SpeedKPH)
	}

	if len(input.Vehicles) == 0 {
		return errors.New("at least one vehicle is required")
	}

	fzMap := buildFzMap()

	for i, vi := range input.Vehicles {
		if _, ok := fzMap[vi.Fz]; !ok {
			return fmt.Errorf("vehicle[%d]: unknown Fz-Kategorie %d", i, vi.Fz)
		}

		if vi.NPerHour < 0 || math.IsNaN(vi.NPerHour) || math.IsInf(vi.NPerHour, 0) {
			return fmt.Errorf("vehicle[%d]: NPerHour must be finite and >= 0, got %g", i, vi.NPerHour)
		}
	}

	return nil
}
