package rail

import "math"

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

// ComputeLden computes the day-evening-night indicator from period levels.
func ComputeLden(levels PeriodLevels) float64 {
	dayLin := 12 * math.Pow(10, levels.Lday/10)
	eveningLin := 4 * math.Pow(10, (levels.Levening+5)/10)
	nightLin := 8 * math.Pow(10, (levels.Lnight+10)/10)
	total := (dayLin + eveningLin + nightLin) / 24.0
	if total <= 0 {
		return -999.0
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
