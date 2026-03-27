package road

import (
	"fmt"
	"math"
)

// Table2RoadClass identifies the RLS-19 road-class group used in Tabelle 2.
type Table2RoadClass int

const (
	Table2RoadClassMotorwayExpressway Table2RoadClass = iota
	Table2RoadClassFederalRoad
	Table2RoadClassStateCountyMunicipalLinkRoad
	Table2RoadClassMunicipalRoad
)

// Table2Period identifies the Tabelle 2 time period.
type Table2Period int

const (
	Table2PeriodDay Table2Period = iota
	Table2PeriodNight
)

// Table2HourlyTraffic stores the Tabelle 2 hourly traffic representation.
// MPerHour is the hourly traffic volume; P1Percent and P2Percent are the
// Lkw1 and Lkw2 shares within M, expressed in percent.
type Table2HourlyTraffic struct {
	MPerHour  float64
	P1Percent float64
	P2Percent float64
}

// ToTrafficInput expands the Tabelle 2 representation into the package's
// per-group hourly counts. Motorcycles are not part of Tabelle 2 and remain
// an explicit input.
func (t Table2HourlyTraffic) ToTrafficInput(kradPerHour float64) TrafficInput {
	lkw1 := t.MPerHour * t.P1Percent / 100
	lkw2 := t.MPerHour * t.P2Percent / 100
	pkw := t.MPerHour - lkw1 - lkw2

	return TrafficInput{
		PkwPerHour:  pkw,
		Lkw1PerHour: lkw1,
		Lkw2PerHour: lkw2,
		KradPerHour: kradPerHour,
	}
}

// DTVToHourly converts a DTV value to the Tabelle 2 hourly representation
// for the selected road class and time period.
func DTVToHourly(dtv float64, roadClass Table2RoadClass, period Table2Period) (Table2HourlyTraffic, error) {
	if math.IsNaN(dtv) || math.IsInf(dtv, 0) || dtv < 0 {
		return Table2HourlyTraffic{}, fmt.Errorf("dtv must be finite and >= 0, got %g", dtv)
	}

	switch roadClass {
	case Table2RoadClassMotorwayExpressway:
		switch period {
		case Table2PeriodDay:
			return Table2HourlyTraffic{MPerHour: 0.0555 * dtv, P1Percent: 3, P2Percent: 11}, nil
		case Table2PeriodNight:
			return Table2HourlyTraffic{MPerHour: 0.0140 * dtv, P1Percent: 10, P2Percent: 25}, nil
		}
	case Table2RoadClassFederalRoad:
		switch period {
		case Table2PeriodDay:
			return Table2HourlyTraffic{MPerHour: 0.0575 * dtv, P1Percent: 3, P2Percent: 7}, nil
		case Table2PeriodNight:
			return Table2HourlyTraffic{MPerHour: 0.0100 * dtv, P1Percent: 7, P2Percent: 13}, nil
		}
	case Table2RoadClassStateCountyMunicipalLinkRoad:
		switch period {
		case Table2PeriodDay:
			return Table2HourlyTraffic{MPerHour: 0.0575 * dtv, P1Percent: 3, P2Percent: 5}, nil
		case Table2PeriodNight:
			return Table2HourlyTraffic{MPerHour: 0.0100 * dtv, P1Percent: 5, P2Percent: 6}, nil
		}
	case Table2RoadClassMunicipalRoad:
		switch period {
		case Table2PeriodDay:
			return Table2HourlyTraffic{MPerHour: 0.0575 * dtv, P1Percent: 3, P2Percent: 4}, nil
		case Table2PeriodNight:
			return Table2HourlyTraffic{MPerHour: 0.0100 * dtv, P1Percent: 3, P2Percent: 4}, nil
		}
	default:
		return Table2HourlyTraffic{}, fmt.Errorf("unknown Tabelle 2 road class %d", roadClass)
	}

	return Table2HourlyTraffic{}, fmt.Errorf("unknown Tabelle 2 period %d", period)
}
