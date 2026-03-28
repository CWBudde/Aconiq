package talaerm

import (
	"errors"
	"fmt"
	"math"
)

// AssessmentPeriod identifies the assessment time period per TA Lärm Nr. 6.4.
type AssessmentPeriod string

const (
	PeriodDay   AssessmentPeriod = "day"
	PeriodNight AssessmentPeriod = "night"
)

// Time boundaries for assessment periods (Nr. 6.4 TA Lärm).
const (
	DayStartHour = 6  // 06:00
	DayEndHour   = 22 // 22:00
)

// Reference durations Tr in hours per Nr. 6.4 / A.1.3.
const (
	DayDurationH         = 16.0 // Tr = 16 h for day
	NightFullDurationH   = 8.0  // Tr = 8 h for full night
	NightLoudestHourH    = 1.0  // Tr = 1 h for lauteste Nachtstunde
	durationSumTolerance = 1e-9
)

// Teilzeit represents a sub-period within an assessment period, carrying
// the partial duration, its equivalent continuous sound level, and any
// applicable surcharges per Nr. A.3.3.
type Teilzeit struct {
	DurationH float64 // Tj in hours
	LAeq      float64 // Mittelungspegel during Tj
	KT        float64 // Tonhaltigkeit/Informationshaltigkeit (0, 3, or 6 dB)
	KI        float64 // Impulshaltigkeit (0, 3, or 6 dB)
	KR        float64 // erhöhte Empfindlichkeit (0 or 6 dB)
}

// ErhoehtEmpfindlichkeitZuschlag is the surcharge value in dB per Nr. 6.5.
const ErhoehtEmpfindlichkeitZuschlag = 6.0

// ValidateTeilzeiten checks that a slice of Teilzeiten is consistent with
// the given assessment period. It returns an error if any constraint is
// violated.
func ValidateTeilzeiten(period AssessmentPeriod, teilzeiten []Teilzeit) error {
	if len(teilzeiten) == 0 {
		return errors.New("at least one Teilzeit is required")
	}

	sumDuration := 0.0

	for i, tz := range teilzeiten {
		if tz.DurationH <= 0 {
			return fmt.Errorf("teilzeit[%d]: DurationH must be > 0, got %g", i, tz.DurationH)
		}

		if math.IsNaN(tz.LAeq) || math.IsInf(tz.LAeq, 0) {
			return fmt.Errorf("teilzeit[%d]: LAeq must be finite", i)
		}

		if !isValidSurcharge036(tz.KT) {
			return fmt.Errorf("teilzeit[%d]: KT must be 0, 3, or 6, got %g", i, tz.KT)
		}

		if !isValidSurcharge036(tz.KI) {
			return fmt.Errorf("teilzeit[%d]: KI must be 0, 3, or 6, got %g", i, tz.KI)
		}

		if !isValidSurcharge06(tz.KR) {
			return fmt.Errorf("teilzeit[%d]: KR must be 0 or 6, got %g", i, tz.KR)
		}

		sumDuration += tz.DurationH
	}

	return validateDurationSum(period, sumDuration)
}

func validateDurationSum(period AssessmentPeriod, sum float64) error {
	switch period {
	case PeriodDay:
		if math.Abs(sum-DayDurationH) > durationSumTolerance {
			return fmt.Errorf("sum of Teilzeit durations must equal %g h for day, got %g h", DayDurationH, sum)
		}
	case PeriodNight:
		matchesFull := math.Abs(sum-NightFullDurationH) <= durationSumTolerance
		matchesLoudest := math.Abs(sum-NightLoudestHourH) <= durationSumTolerance

		if !matchesFull && !matchesLoudest {
			return fmt.Errorf("sum of Teilzeit durations must equal %g h or %g h for night, got %g h",
				NightFullDurationH, NightLoudestHourH, sum)
		}
	default:
		return fmt.Errorf("unknown assessment period %q", period)
	}

	return nil
}

func isValidSurcharge036(v float64) bool {
	return v == 0 || v == 3 || v == 6
}

func isValidSurcharge06(v float64) bool {
	return v == 0 || v == 6
}

// IsErhoehtEmpfindlichkeitTime checks if the given hour falls within a
// sensitive time window per Nr. 6.5 TA Lärm.
// hour is 0-23, isWeekday is true for Mon-Sat (Werktage).
// Note: Saturday is a Werktag in German law.
func IsErhoehtEmpfindlichkeitTime(hour int, isWeekday bool) bool {
	if isWeekday {
		return hour == 6 || hour == 20 || hour == 21
	}

	// Sonn- und Feiertage
	return hour == 6 || hour == 7 || hour == 8 ||
		hour == 13 || hour == 14 ||
		hour == 20 || hour == 21
}

// NightAssessmentDuration returns the reference duration Tr for the night
// period depending on the assessment mode.
// mode must be "full" (8 h) or "loudest_hour" (1 h).
func NightAssessmentDuration(mode string) float64 {
	switch mode {
	case "full":
		return NightFullDurationH
	case "loudest_hour":
		return NightLoudestHourH
	default:
		return 0
	}
}
