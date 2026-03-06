package road

// PeriodLevels stores receiver levels per time period.
type PeriodLevels struct {
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

// ToReceiverIndicators builds the final indicator payload.
func (levels PeriodLevels) ToReceiverIndicators() ReceiverIndicators {
	return ReceiverIndicators{
		LrDay:   levels.LrDay,
		LrNight: levels.LrNight,
	}
}

