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
	// Eisenbahn track type (Table 7). Ignored for Strassenbahn vehicles.
	Fahrbahn FahrbahnartType // from tables.go
	// Strassenbahn track type (Table 15). Ignored for Eisenbahn vehicles.
	SFahrbahn       SFahrbahnartType
	Surface         SurfaceCondType // from this file
	BridgeType      int             // 0=none, 1-4 per Table 9 (Eisenbahn) or Table 16 (Strassenbahn)
	BridgeMitig     bool            // K_LM applies
	CurveRadiusM    float64         // 0 = straight, >0 = curved
	PermanentlySlow bool            // Nr. 5.3.2: section permanently at ≤ 30 km/h (Straßenbahn only)

	// MeasuredVehicles provides Section 9 measurement-based vehicle categories
	// (Fz >= 100) that extend the standard Beiblatt 1-3 lookup table.
	// When a VehicleInput references an Fz in this slice, its measured spectra
	// are used instead of the standard tables.
	MeasuredVehicles []MeasuredVehicle
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

	// Detect mode from first vehicle.
	isStrassenbahn := len(input.Vehicles) > 0 && IsStrassenbahnFz(input.Vehicles[0].Fz)

	// Apply speed clamp per Nr. 5.3.2:
	//   - Default: Straßenbahn minimum speed is 50 km/h.
	//   - Exception: sections permanently at ≤ 30 km/h (r > 200 m, no switches/
	//     stations/crossings) use v = 30 km/h instead of clamping to 50.
	effectiveSpeed := input.SpeedKPH
	if isStrassenbahn && effectiveSpeed < 50 {
		if input.PermanentlySlow {
			effectiveSpeed = 30
		} else {
			effectiveSpeed = 50
		}
	}

	fzMap := buildFzMap()

	// Register Section 9 measured vehicles into the lookup map.
	for i := range input.MeasuredVehicles {
		fzKat := input.MeasuredVehicles[i].ToFzKategorie()
		fzMap[fzKat.Fz] = &fzKat
	}

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
				tq, nQ, nQ0, effectiveSpeed,
				input.Fahrbahn, input.SFahrbahn, input.Surface,
				input.BridgeType, input.BridgeMitig, input.CurveRadiusM,
				isStrassenbahn,
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
	sFahrbahn SFahrbahnartType,
	surface SurfaceCondType,
	bridgeType int,
	bridgeMitig bool,
	curveRadiusM float64,
	isStrassenbahn bool,
) BeiblattSpectrum {
	// Speed factor b: use per-vehicle override (Strassenbahn Table 14) when set,
	// otherwise look up from the Eisenbahn speed-factor table (Table 6).
	var b BeiblattSpectrum
	if tq.B != nil {
		b = *tq.B
	} else {
		b = SpeedFactorBForTeilquelle(tq.M)
	}

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		L := tq.AA + tq.DeltaA[f] // a_A + Δa_f

		// Axle correction: only for rolling noise (SourceTypeRolling).
		if tq.SourceType == SourceTypeRolling && nQ > 0 && nQ0 > 0 {
			L += 10.0 * math.Log10(float64(nQ)/float64(nQ0))
		}

		// Speed correction: b_f * lg(v/v0).
		L += b[f] * math.Log10(speedKPH/v0)

		if isStrassenbahn {
			// Strassenbahn: c1 from Table 15, bridge from Table 16, curve per Nr. 5.3.2.
			L += sumC1StrassenbahnForTeilquelle(sFahrbahn, tq.M, f)
			L += bridgeCorrectionStrassenbahnForTeilquelle(bridgeType, bridgeMitig, tq.M)
			L += curveCorrectionStrassenbahnForTeilquelle(curveRadiusM, tq.M)
		} else {
			// Eisenbahn: c1 from Table 7, c2 from Table 8, bridge from Table 9, curve per Nr. 4.3.
			L += sumC1ForTeilquelle(fahrbahn, tq.M, f)
			L += sumC2ForTeilquelle(surface, tq.M, f)
			L += bridgeCorrectionForTeilquelle(bridgeType, bridgeMitig, tq.M)
			L += curveCorrectionForTeilquelle(curveRadiusM, tq.M)
		}

		result[f] = L
	}

	return result
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
// Includes both Eisenbahn (Fz 1-10) and Strassenbahn (Fz 21-23).
func buildFzMap() map[int]*FzKategorie {
	m := make(map[int]*FzKategorie, len(FzKategorien)+len(FzKategorienStrassenbahn))
	for i := range FzKategorien {
		m[FzKategorien[i].Fz] = &FzKategorien[i]
	}

	for i := range FzKategorienStrassenbahn {
		m[FzKategorienStrassenbahn[i].Fz] = &FzKategorienStrassenbahn[i]
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

	measuredFz, err := validateMeasuredVehicles(input.MeasuredVehicles)
	if err != nil {
		return err
	}

	fzMap := buildFzMap()
	hasEisenbahn := false
	hasStrassenbahn := false

	for i, vi := range input.Vehicles {
		_, inStandard := fzMap[vi.Fz]
		_, inMeasured := measuredFz[vi.Fz]

		if !inStandard && !inMeasured {
			return fmt.Errorf("vehicle[%d]: unknown Fz-Kategorie %d", i, vi.Fz)
		}

		if vi.NPerHour < 0 || math.IsNaN(vi.NPerHour) || math.IsInf(vi.NPerHour, 0) {
			return fmt.Errorf("vehicle[%d]: NPerHour must be finite and >= 0, got %g", i, vi.NPerHour)
		}

		// Measured vehicles (Fz >= 100) are treated as Eisenbahn for mode detection.
		if IsStrassenbahnFz(vi.Fz) {
			hasStrassenbahn = true
		} else {
			hasEisenbahn = true
		}
	}

	if hasEisenbahn && hasStrassenbahn {
		return errors.New("cannot mix Eisenbahn (Fz 1-10) and Strassenbahn (Fz 21-23) in one segment")
	}

	return nil
}

// validateMeasuredVehicles validates Section 9 measured vehicles and returns
// a set of their Fz numbers for lookup during vehicle validation.
func validateMeasuredVehicles(mvs []MeasuredVehicle) (map[int]struct{}, error) {
	result := make(map[int]struct{}, len(mvs))

	for i, mv := range mvs {
		err := mv.Validate()
		if err != nil {
			return nil, fmt.Errorf("measured_vehicles[%d]: %w", i, err)
		}

		result[mv.Fz] = struct{}{}
	}

	return result, nil
}
