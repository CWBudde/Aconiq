package runestimate

import (
	"fmt"
	"math"
)

// FormatBand renders a band for a person: "≈ 6–9 min", "≈ 40–70 s",
// "≈ 2–3 h". The unit is chosen by the slow end, so both ends read in one
// unit, and the ends are rounded outwards so the band never narrows in the
// telling.
//
// The frontend formats the same JSON itself, in the reader's locale; this is
// the CLI's wording.
func FormatBand(b Band) string {
	if b.High < 1 {
		return "< 1 s"
	}

	unit, scale := bandUnit(b.High)

	// One unit finer where the fast end would read as zero: "≈ 40–240 min"
	// rather than "≈ 0–4 h".
	if b.Low < scale {
		unit, scale = finerUnit(unit)
	}

	low := math.Max(math.Floor(b.Low/scale), 0)
	high := math.Ceil(b.High / scale)

	if low >= high {
		return fmt.Sprintf("≈ %.0f %s", high, unit)
	}

	return fmt.Sprintf("≈ %.0f–%.0f %s", low, high, unit)
}

func finerUnit(unit string) (string, float64) {
	if unit == "h" {
		return "min", 60
	}

	return "s", 1
}

func bandUnit(seconds float64) (string, float64) {
	switch {
	case seconds < 120:
		return "s", 1
	case seconds < 2*3600:
		return "min", 60
	default:
		return "h", 3600
	}
}
