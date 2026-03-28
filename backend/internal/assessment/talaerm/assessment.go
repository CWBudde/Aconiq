package talaerm

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// MessabschlagDB is the measurement deduction per Nr. 6.9 TA Lärm.
const MessabschlagDB = 3.0

// GemengelageMaxDay is the maximum effective day threshold for Gemengelage per Nr. 6.7
// (not exceeding Kern-/Dorf-/Mischgebiet = 60 dB(A)).
const GemengelageMaxDay = 60

// GemengelageMaxNight is the maximum effective night threshold for Gemengelage per Nr. 6.7.
const GemengelageMaxNight = 45

// PeriodLevels holds the Beurteilungspegel for day and night.
type PeriodLevels struct {
	Day   float64 `json:"day"`
	Night float64 `json:"night"`
}

// Gemengelage holds the effective thresholds for a mixed-area situation per Nr. 6.7.
type Gemengelage struct {
	EffectiveDay   int `json:"effective_day"`
	EffectiveNight int `json:"effective_night"`
}

// ReceiverInput provides all input values for a single receiver assessment.
type ReceiverInput struct {
	ReceiverID         string
	AreaCategory       AreaCategory
	Zusatzbelastung    PeriodLevels  // Lr day/night from assessed plant
	Vorbelastung       *PeriodLevels // Lr day/night from all other plants (optional)
	PeakDay            *float64      // LAFmax day (optional)
	PeakNight          *float64      // LAFmax night (optional)
	Gemengelage        *Gemengelage  // mixed-area override (optional)
	IsMeasurementBased bool          // triggers Nr. 6.9 Messabschlag (-3 dB)
}

// LevelAssessment holds the assessment result for a single period.
type LevelAssessment struct {
	LevelRaw     float64 `json:"level_raw"`
	LevelRounded int     `json:"level_rounded"`
	Richtwert    int     `json:"richtwert"`
	Exceeds      bool    `json:"exceeds"`
	ExceedanceDB int     `json:"exceedance_db"`
}

// PeakAssessment holds the peak level assessment result.
type PeakAssessment struct {
	PeakDay      *float64 `json:"peak_day,omitempty"`
	PeakNight    *float64 `json:"peak_night,omitempty"`
	LimitDay     int      `json:"limit_day"`
	LimitNight   int      `json:"limit_night"`
	DayExceeds   bool     `json:"day_exceeds"`
	NightExceeds bool     `json:"night_exceeds"`
}

// ReceiverAssessment holds the complete assessment result for a single receiver.
type ReceiverAssessment struct {
	ReceiverID           string           `json:"receiver_id"`
	AreaCategory         AreaCategory     `json:"area_category"`
	AreaCategoryLabelDE  string           `json:"area_category_label_de"`
	AreaCategoryCode     string           `json:"area_category_code"`
	Richtwerte           Thresholds       `json:"richtwerte"`
	ZusatzbelastungDay   LevelAssessment  `json:"zusatzbelastung_day"`
	ZusatzbelastungNight LevelAssessment  `json:"zusatzbelastung_night"`
	Vorbelastung         *PeriodLevels    `json:"vorbelastung,omitempty"`
	Gesamtbelastung      *PeriodLevels    `json:"gesamtbelastung,omitempty"`
	Irrelevanz           IrrelevanzResult `json:"irrelevanz"`
	Peak                 *PeakAssessment  `json:"peak,omitempty"`
	MeasurementDeduction bool             `json:"measurement_deduction"`
	Exceeds              bool             `json:"exceeds"`
	SummaryDE            string           `json:"summary_de"`
}

// AssessReceiver performs the Regelfallpruefung for a single receiver per TA Laerm.
func AssessReceiver(input ReceiverInput) (ReceiverAssessment, error) {
	if strings.TrimSpace(input.ReceiverID) == "" {
		return ReceiverAssessment{}, errors.New("receiver id is required")
	}

	// Determine Richtwerte: Gemengelage override or standard thresholds.
	richtwerte, err := effectiveRichtwerte(input)
	if err != nil {
		return ReceiverAssessment{}, err
	}

	// Apply Nr. 6.9 Messabschlag if measurement-based.
	effectiveDay := input.Zusatzbelastung.Day
	effectiveNight := input.Zusatzbelastung.Night

	if input.IsMeasurementBased {
		effectiveDay -= MessabschlagDB
		effectiveNight -= MessabschlagDB
	}

	// Assess day and night Zusatzbelastung.
	dayAssessment := assessLevel(effectiveDay, richtwerte.Day)
	nightAssessment := assessLevel(effectiveNight, richtwerte.Night)

	// Irrelevance check on the effective Zusatzbelastung.
	irrelevanz := CheckIrrelevanz(effectiveDay, effectiveNight, richtwerte)

	// Gesamtbelastung if Vorbelastung is provided.
	var gesamtbelastung *PeriodLevels

	if input.Vorbelastung != nil {
		gDay := ComputeGesamtbelastung(input.Vorbelastung.Day, effectiveDay)
		gNight := ComputeGesamtbelastung(input.Vorbelastung.Night, effectiveNight)

		gesamtbelastung = &PeriodLevels{Day: gDay, Night: gNight}
	}

	// Peak assessment.
	var peakResult *PeakAssessment

	if input.PeakDay != nil || input.PeakNight != nil {
		pa := assessPeaks(input, richtwerte)
		peakResult = &pa
	}

	// Determine overall exceedance.
	// Per Nr. 3.2.1: if irrelevant for both periods, Zusatzbelastung exceedance
	// does not matter. But peak exceedance is independent of irrelevance.
	bothIrrelevant := irrelevanz.DayIrrelevant && irrelevanz.NightIrrelevant
	lrExceeds := dayAssessment.Exceeds || nightAssessment.Exceeds

	peakExceeds := false
	if peakResult != nil {
		peakExceeds = peakResult.DayExceeds || peakResult.NightExceeds
	}

	exceeds := peakExceeds
	if !bothIrrelevant {
		exceeds = exceeds || lrExceeds
	}

	result := ReceiverAssessment{
		ReceiverID:           input.ReceiverID,
		AreaCategory:         input.AreaCategory,
		AreaCategoryLabelDE:  AreaCategoryLabelDE(input.AreaCategory),
		AreaCategoryCode:     AreaCategoryCode(input.AreaCategory),
		Richtwerte:           richtwerte,
		ZusatzbelastungDay:   dayAssessment,
		ZusatzbelastungNight: nightAssessment,
		Vorbelastung:         input.Vorbelastung,
		Gesamtbelastung:      gesamtbelastung,
		Irrelevanz:           irrelevanz,
		Peak:                 peakResult,
		MeasurementDeduction: input.IsMeasurementBased,
		Exceeds:              exceeds,
	}

	result.SummaryDE = buildSummaryDE(result)

	return result, nil
}

func effectiveRichtwerte(input ReceiverInput) (Thresholds, error) {
	if input.Gemengelage != nil {
		g := input.Gemengelage
		if g.EffectiveDay > GemengelageMaxDay {
			return Thresholds{}, fmt.Errorf(
				"gemengelage effective day %d exceeds maximum %d (Kern-/Dorf-/Mischgebiet)",
				g.EffectiveDay, GemengelageMaxDay,
			)
		}

		if g.EffectiveNight > GemengelageMaxNight {
			return Thresholds{}, fmt.Errorf(
				"gemengelage effective night %d exceeds maximum %d (Kern-/Dorf-/Mischgebiet)",
				g.EffectiveNight, GemengelageMaxNight,
			)
		}

		return Thresholds{Day: g.EffectiveDay, Night: g.EffectiveNight}, nil
	}

	return ThresholdsOutdoor(input.AreaCategory)
}

func assessLevel(level float64, richtwert int) LevelAssessment {
	rounded := int(math.Ceil(level))
	exceedance := max(0, rounded-richtwert)

	return LevelAssessment{
		LevelRaw:     level,
		LevelRounded: rounded,
		Richtwert:    richtwert,
		Exceeds:      rounded > richtwert,
		ExceedanceDB: exceedance,
	}
}

func assessPeaks(input ReceiverInput, richtwerte Thresholds) PeakAssessment {
	peakLimits := PeakLimits{
		Day:   richtwerte.Day + 30,
		Night: richtwerte.Night + 20,
	}

	pa := PeakAssessment{
		PeakDay:    input.PeakDay,
		PeakNight:  input.PeakNight,
		LimitDay:   peakLimits.Day,
		LimitNight: peakLimits.Night,
	}

	if input.PeakDay != nil {
		pa.DayExceeds = *input.PeakDay > float64(peakLimits.Day)
	}

	if input.PeakNight != nil {
		pa.NightExceeds = *input.PeakNight > float64(peakLimits.Night)
	}

	return pa
}

func buildSummaryDE(result ReceiverAssessment) string {
	var parts []string

	// Header with receiver identification and Richtwerte.
	header := fmt.Sprintf(
		"Immissionsort %s (%s, Buchstabe %s): Immissionsrichtwerte Tag/Nacht %d/%d dB(A).",
		result.ReceiverID,
		result.AreaCategoryLabelDE,
		result.AreaCategoryCode,
		result.Richtwerte.Day,
		result.Richtwerte.Night,
	)
	parts = append(parts, header)

	// Zusatzbelastung with optional Messabschlag note.
	zusatzLine := fmt.Sprintf(
		"Zusatzbelastung Tag/Nacht %d/%d dB(A).",
		result.ZusatzbelastungDay.LevelRounded,
		result.ZusatzbelastungNight.LevelRounded,
	)

	if result.MeasurementDeduction {
		zusatzLine += " Messabschlag von 3 dB nach Nr. 6.9 wurde angewendet."
	}

	parts = append(parts, zusatzLine)

	// Irrelevance.
	if result.Irrelevanz.DayIrrelevant && result.Irrelevanz.NightIrrelevant {
		parts = append(parts,
			"Irrelevanzkriterium erfüllt: Zusatzbelastung unterschreitet den Immissionsrichtwert um mindestens 6 dB(A).",
		)
	}

	// Gesamtbelastung.
	if result.Gesamtbelastung != nil {
		parts = append(parts, fmt.Sprintf(
			"Gesamtbelastung Tag/Nacht %.1f/%.1f dB(A).",
			result.Gesamtbelastung.Day,
			result.Gesamtbelastung.Night,
		))
	}

	// Peak assessment.
	if result.Peak != nil {
		if result.Peak.DayExceeds || result.Peak.NightExceeds {
			parts = append(parts, "Kurzzeitige Geräuschspitzen überschreiten die zulässigen Werte.")
		} else {
			parts = append(parts, "Kurzzeitige Geräuschspitzen werden eingehalten.")
		}
	}

	// Overall verdict.
	if result.Exceeds {
		parts = append(parts, "Die Immissionsrichtwerte werden überschritten.")
	} else {
		parts = append(parts, "Die Immissionsrichtwerte werden eingehalten.")
	}

	return strings.Join(parts, " ")
}
