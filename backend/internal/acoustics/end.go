// Package acoustics holds the definitions more than one standards module has to
// agree on, so that agreeing is not a matter of eight copies staying in step.
//
// It starts with the END indicator family — the day/evening/night set that
// Directive 2002/49/EC defines. Six modules report it and carried
// byte-identical copies of the levels, the indicator payload and the Lden
// formula: the four cnossos scaffolds and the bub/buf counterparts that alias
// them. A correction to the directive's defining equation had to be applied six
// times, and nothing would have caught it being applied five.
//
// Not every module in the repository reports this set, and this package makes
// no claim on the ones that do not: rls19/road and schall03 publish the German
// Beurteilungspegel LrDay/LrNight and keep their own indicator model.
//
// What is deliberately *not* here yet: the energy summation helper, which
// exists in nine copies with three different silence semantics (-999 in some
// modules, -Inf in others, and one that returns NaN). Unifying those changes
// numbers, so it is its own piece of work — see PLAN.md Priority 7, "Extract a
// shared acoustics core".
package acoustics

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// The END indicator names, as they appear in receiver tables, raster band names
// and every module's indicator order.
const (
	IndicatorLday     = "Lday"
	IndicatorLevening = "Levening"
	IndicatorLnight   = "Lnight"
	IndicatorLden     = "Lden"
)

// SilenceDB is the level reported when a receiver has no acoustic energy at
// all. It is what ComputeLden returns for a non-positive energy total, where
// 10 lg is undefined.
const SilenceDB = -999.0

// PeriodLevels stores receiver levels per time period.
type PeriodLevels struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	Lday     float64
	Levening float64
	Lnight   float64
	Lden     float64
}

// ReceiverOutput pairs a receiver with the indicators computed for it.
type ReceiverOutput struct {
	Receiver   geo.PointReceiver
	Indicators ReceiverIndicators
}

// ComputeLden computes the day-evening-night indicator from period levels.
//
// This is Annex I of Directive 2002/49/EC: the three periods weighted by their
// length in hours, with the +5 dB evening and +10 dB night penalties applied
// before the energy sum.
func ComputeLden(levels PeriodLevels) float64 {
	dayLin := 12 * math.Pow(10, levels.Lday/10)
	eveningLin := 4 * math.Pow(10, (levels.Levening+5)/10)
	nightLin := 8 * math.Pow(10, (levels.Lnight+10)/10)

	total := (dayLin + eveningLin + nightLin) / 24.0
	if total <= 0 {
		return SilenceDB
	}

	return 10 * math.Log10(total)
}

// ToReceiverIndicators builds the final indicator payload.
func (levels PeriodLevels) ToReceiverIndicators() ReceiverIndicators {
	return ReceiverIndicators{
		Lday:     levels.Lday,
		Levening: levels.Levening,
		Lnight:   levels.Lnight,
		Lden:     ComputeLden(levels),
	}
}

// IndicatorOrder is the order in which the END indicators are written to a
// receiver table: the composite first, then the periods that make it up.
func IndicatorOrder() []string {
	return []string{IndicatorLden, IndicatorLnight, IndicatorLday, IndicatorLevening}
}

// Values renders one receiver's indicators as the map a receiver table record
// carries.
func (indicators ReceiverIndicators) Values() map[string]float64 {
	return map[string]float64{
		IndicatorLden:     indicators.Lden,
		IndicatorLnight:   indicators.Lnight,
		IndicatorLday:     indicators.Lday,
		IndicatorLevening: indicators.Levening,
	}
}
