package talaerm

import (
	"errors"
	"math"
)

// ComputeGesamtbelastung computes the total noise load per Anhang equation G1:
//
//	LG = 10 · lg(10^(0.1·LV) + 10^(0.1·LZ))
func ComputeGesamtbelastung(vorbelastung, zusatzbelastung float64) float64 {
	return 10 * math.Log10(math.Pow(10, 0.1*vorbelastung)+math.Pow(10, 0.1*zusatzbelastung))
}

// IsIrrelevant checks whether the Zusatzbelastung is irrelevant per Nr. 3.2.1 Abs. 2.
// The Zusatzbelastung is irrelevant if it is at least 6 dB(A) below the applicable
// Immissionsrichtwert.
func IsIrrelevant(zusatzbelastung float64, richtwert int) bool {
	return zusatzbelastung <= float64(richtwert)-6.0
}

// IrrelevanzResult holds the outcome of the irrelevance check for both assessment periods.
type IrrelevanzResult struct {
	DayIrrelevant   bool    `json:"day_irrelevant"`
	NightIrrelevant bool    `json:"night_irrelevant"`
	DayMarginDB     float64 `json:"day_margin_db"`
	NightMarginDB   float64 `json:"night_margin_db"`
}

// CheckIrrelevanz checks irrelevance for both day and night periods independently.
func CheckIrrelevanz(zusatzDay, zusatzNight float64, richtwerte Thresholds) IrrelevanzResult {
	dayMargin := float64(richtwerte.Day) - 6.0 - zusatzDay
	nightMargin := float64(richtwerte.Night) - 6.0 - zusatzNight

	return IrrelevanzResult{
		DayIrrelevant:   dayMargin >= 0,
		NightIrrelevant: nightMargin >= 0,
		DayMarginDB:     dayMargin,
		NightMarginDB:   nightMargin,
	}
}

// LoadInput provides the input values for a load assessment.
type LoadInput struct {
	ZusatzbelastungDay   float64  // Lr day from the assessed plant
	ZusatzbelastungNight float64  // Lr night from the assessed plant
	VorbelastungDay      *float64 // Lr day from all other plants (nil if not determined)
	VorbelastungNight    *float64 // Lr night (nil if not determined)
}

// LoadAssessment holds the complete result of a TA Lärm load assessment.
type LoadAssessment struct {
	ZusatzbelastungDay   float64          `json:"zusatzbelastung_day"`
	ZusatzbelastungNight float64          `json:"zusatzbelastung_night"`
	VorbelastungDay      *float64         `json:"vorbelastung_day,omitempty"`
	VorbelastungNight    *float64         `json:"vorbelastung_night,omitempty"`
	GesamtbelastungDay   *float64         `json:"gesamtbelastung_day,omitempty"`
	GesamtbelastungNight *float64         `json:"gesamtbelastung_night,omitempty"`
	Irrelevanz           IrrelevanzResult `json:"irrelevanz"`
}

// AssessLoad performs the full Vorbelastung/Zusatzbelastung/Gesamtbelastung assessment
// including the Relevanzprüfung per TA Lärm Nr. 3.2.1.
func AssessLoad(input LoadInput, richtwerte Thresholds) (LoadAssessment, error) {
	if math.IsNaN(input.ZusatzbelastungDay) || math.IsInf(input.ZusatzbelastungDay, 0) ||
		math.IsNaN(input.ZusatzbelastungNight) || math.IsInf(input.ZusatzbelastungNight, 0) {
		return LoadAssessment{}, errors.New("zusatzbelastung values must be finite")
	}

	if input.VorbelastungDay != nil && (math.IsNaN(*input.VorbelastungDay) || math.IsInf(*input.VorbelastungDay, 0)) {
		return LoadAssessment{}, errors.New("vorbelastung day must be finite")
	}

	if input.VorbelastungNight != nil && (math.IsNaN(*input.VorbelastungNight) || math.IsInf(*input.VorbelastungNight, 0)) {
		return LoadAssessment{}, errors.New("vorbelastung night must be finite")
	}

	result := LoadAssessment{
		ZusatzbelastungDay:   input.ZusatzbelastungDay,
		ZusatzbelastungNight: input.ZusatzbelastungNight,
		VorbelastungDay:      input.VorbelastungDay,
		VorbelastungNight:    input.VorbelastungNight,
	}

	result.Irrelevanz = CheckIrrelevanz(input.ZusatzbelastungDay, input.ZusatzbelastungNight, richtwerte)

	if input.VorbelastungDay != nil {
		g := ComputeGesamtbelastung(*input.VorbelastungDay, input.ZusatzbelastungDay)
		result.GesamtbelastungDay = &g
	}

	if input.VorbelastungNight != nil {
		g := ComputeGesamtbelastung(*input.VorbelastungNight, input.ZusatzbelastungNight)
		result.GesamtbelastungNight = &g
	}

	return result, nil
}
