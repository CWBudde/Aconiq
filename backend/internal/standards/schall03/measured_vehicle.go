package schall03

import (
	"errors"
	"fmt"
	"math"
)

// MeasuredVehicle holds measurement-based emission data for a vehicle type
// that is not covered by the standard Beiblatt 1-3 tables, per Section 9
// of Schall 03 (Anlage 2, Nr. 9.1–9.3).
//
// The measurement data is typically derived from pass-by measurements per
// DIN EN ISO 3095 and processed according to the rules of Nr. 9.1.1.
type MeasuredVehicle struct {
	// Fz is the custom Fahrzeug-Kategorie number. Must be >= 100 to avoid
	// collision with standard Beiblatt 1-3 categories (1-10, 21-23).
	Fz int `json:"fz"`

	// Name is a descriptive name for the measured vehicle type.
	Name string `json:"name"`

	// NAchs0 is the reference axle count for the vehicle unit.
	NAchs0 int `json:"n_achs_0"`

	// Teilquellen provides the per-Teilquelle octave-band spectra derived
	// from measurement data. Each entry follows the same Teilquelle structure
	// as Beiblatt 1-3: M (1-11), SourceType, HeightH, DeltaA, AA.
	Teilquellen []Teilquelle `json:"teilquellen"`

	// RoughnessSplit describes how the measured rolling noise was split
	// between wheel roughness (Radrauheit, m=2) and rail roughness
	// (Schienenrauheit, m=1) per Nr. 9.1.1.
	RoughnessSplit RoughnessSplitMethod `json:"roughness_split"`

	// MeasurementCorrection is the Table 20 correction in dB applied to
	// the measured wheel-roughness component to account for measurement
	// condition variability.
	MeasurementCorrection float64 `json:"measurement_correction_db"`

	// MeasurementDescription is free-text documentation of the measurement
	// setup, required by Nr. 9.3.
	MeasurementDescription string `json:"measurement_description,omitempty"`
}

// RoughnessSplitMethod identifies the wheel/rail roughness splitting
// procedure used per Nr. 9.1.1.
type RoughnessSplitMethod string

const (
	// RoughnessSplitVerySmooth: Measurements on very smooth rails with
	// unmeasured surface condition. 100% of rolling noise assigned to
	// wheel roughness (Table 19, row 1: -20 dB rail contribution).
	RoughnessSplitVerySmooth RoughnessSplitMethod = "very_smooth"

	// RoughnessSplitSmooth: Measurements on smooth rails with verified
	// surface condition (TSI/VDV 154 limit). 80% of rolling noise energy
	// assigned to wheel roughness (Table 19, row 2: -7 dB rail).
	// This is the Regelfall (default procedure).
	RoughnessSplitSmooth RoughnessSplitMethod = "smooth"

	// RoughnessSplitUnknown: Measurements on rails with unknown surface
	// condition. 50/50 energy split between wheel and rail roughness
	// (Table 19, row 3: -3 dB each). Not allowed for Grauguss-Klotz
	// braked vehicles. Only for Straßenbahnen and justified exceptions.
	RoughnessSplitUnknown RoughnessSplitMethod = "unknown"
)

// Table19RailContributionDB returns the rail roughness (Teilquelle m=1)
// contribution relative to the total rolling noise in dB, per Table 19.
func Table19RailContributionDB(method RoughnessSplitMethod) float64 {
	switch method {
	case RoughnessSplitVerySmooth:
		return -20
	case RoughnessSplitSmooth:
		return -7
	case RoughnessSplitUnknown:
		return -3 // equal energy split, at least 50% to wheel
	default:
		return -7 // default to Regelfall
	}
}

// Table19WheelContributionDB returns the wheel roughness (Teilquelle m=2)
// contribution relative to the total rolling noise in dB, per Table 19.
// The wheel contribution is the remainder after subtracting the rail
// contribution (in linear energy domain).
func Table19WheelContributionDB(method RoughnessSplitMethod) float64 {
	railDB := Table19RailContributionDB(method)
	// L_wheel = 10 * lg(1 - 10^(railDB/10))
	railFraction := math.Pow(10, railDB/10)
	wheelFraction := 1 - railFraction

	if wheelFraction <= 0 {
		return 0 // degenerate: all noise is rail
	}

	return 10 * math.Log10(wheelFraction)
}

// Table20CorrectionDB returns the measurement-condition correction from
// Table 20 for converting measured emission to average operating condition.
//
// Parameters:
//   - brakeType: "disc" (Scheibenbremsen), "composite" (Verbundstoff-Klotz),
//     or "cast_iron" (Grauguss-Klotz)
//   - measurementSites: number of measurement sites (1 or 3+)
//   - sameVehicle: true if measurements are from the same vehicle type
//     (e.g., TSI/VDV 154), false if from different vehicles
func Table20CorrectionDB(brakeType string, measurementSites int, sameVehicle bool) float64 {
	// Table 20 has 3 columns:
	// Col A: 1 site, mean over different Fz
	// Col B: 3+ sites, mean over different Fz
	// Col C: 1 site (TSI/VDV 154), mean over same Fz
	type row struct {
		colA, colB, colC float64
	}

	table := map[string]row{
		"disc":      {2, 0, 3},
		"composite": {2, 1, 4},
		"cast_iron": {3, 2, 5},
	}

	r, ok := table[brakeType]
	if !ok {
		return 3 // conservative default
	}

	if sameVehicle {
		return r.colC
	}

	if measurementSites >= 3 {
		return r.colB
	}

	return r.colA
}

// Validate checks the measured vehicle data for consistency.
func (mv MeasuredVehicle) Validate() error {
	if mv.Fz < 100 {
		return fmt.Errorf("MeasuredVehicle: Fz must be >= 100 (got %d); standard categories 1-23 are reserved", mv.Fz)
	}

	if mv.Name == "" {
		return errors.New("MeasuredVehicle: Name is required")
	}

	if mv.NAchs0 <= 0 {
		return fmt.Errorf("MeasuredVehicle: NAchs0 must be > 0, got %d", mv.NAchs0)
	}

	if len(mv.Teilquellen) == 0 {
		return errors.New("MeasuredVehicle: at least one Teilquelle is required")
	}

	for i, tq := range mv.Teilquellen {
		err := validateMeasuredTeilquelle(i, tq)
		if err != nil {
			return err
		}
	}

	switch mv.RoughnessSplit {
	case RoughnessSplitVerySmooth, RoughnessSplitSmooth, RoughnessSplitUnknown:
		// valid
	default:
		return fmt.Errorf("MeasuredVehicle: unknown roughness_split %q", mv.RoughnessSplit)
	}

	if math.IsNaN(mv.MeasurementCorrection) || math.IsInf(mv.MeasurementCorrection, 0) {
		return errors.New("MeasuredVehicle: measurement_correction_db must be finite")
	}

	return nil
}

func validateMeasuredTeilquelle(i int, tq Teilquelle) error {
	if tq.M < 1 || tq.M > 11 {
		return fmt.Errorf("MeasuredVehicle: teilquellen[%d].M must be 1-11, got %d", i, tq.M)
	}

	if tq.HeightH < 1 || tq.HeightH > 3 {
		return fmt.Errorf("MeasuredVehicle: teilquellen[%d].HeightH must be 1-3, got %d", i, tq.HeightH)
	}

	if math.IsNaN(tq.AA) || math.IsInf(tq.AA, 0) {
		return fmt.Errorf("MeasuredVehicle: teilquellen[%d].AA must be finite", i)
	}

	for f, v := range tq.DeltaA {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("MeasuredVehicle: teilquellen[%d].DeltaA[%d] must be finite", i, f)
		}
	}

	return nil
}

// ToFzKategorie converts a MeasuredVehicle to a standard FzKategorie so it
// can be used in the emission pipeline alongside Beiblatt 1-3 categories.
func (mv MeasuredVehicle) ToFzKategorie() FzKategorie {
	return FzKategorie{
		Fz:          mv.Fz,
		Name:        mv.Name,
		NAchs0:      mv.NAchs0,
		Teilquellen: mv.Teilquellen,
	}
}

// Section9SignificanceCheck tests whether the measured vehicle differs
// significantly from the standard Beiblatt spectra per Nr. 9.2.2.
// A significant deviation exists when:
//   - the A-weighted total level differs by >= 2 dB for any Teilquelle, OR
//   - any single octave band differs by >= 4 dB for any Teilquelle.
//
// Returns true if the deviation is significant, along with a description.
func Section9SignificanceCheck(measured, reference FzKategorie) (bool, string) {
	for _, mTQ := range measured.Teilquellen {
		for _, rTQ := range reference.Teilquellen {
			if mTQ.M != rTQ.M {
				continue
			}

			// Check A-weighted total level.
			totalDiff := math.Abs(mTQ.AA - rTQ.AA)
			if totalDiff >= 2.0 {
				return true, fmt.Sprintf(
					"Teilquelle m=%d: A-weighted total differs by %.1f dB (>= 2 dB threshold)",
					mTQ.M, totalDiff,
				)
			}

			// Check per-octave-band.
			for f := range NumBeiblattOctaveBands {
				bandDiff := math.Abs(mTQ.DeltaA[f] - rTQ.DeltaA[f])
				if bandDiff >= 4.0 {
					return true, fmt.Sprintf(
						"Teilquelle m=%d, band %d: octave-band difference %.1f dB (>= 4 dB threshold)",
						mTQ.M, f, bandDiff,
					)
				}
			}
		}
	}

	return false, "no significant deviation"
}
