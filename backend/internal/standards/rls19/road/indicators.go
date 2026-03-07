package road

// PeriodLevels stores receiver levels per RLS-19 time period.
type PeriodLevels struct {
	LrDay   float64 `json:"lr_day"`   // Beurteilungspegel Tag (06-22)
	LrNight float64 `json:"lr_night"` // Beurteilungspegel Nacht (22-06)
}

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

// ToReceiverIndicators builds the final indicator payload from period levels.
func (levels PeriodLevels) ToReceiverIndicators() ReceiverIndicators {
	return ReceiverIndicators(levels)
}
