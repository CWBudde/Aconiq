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

// table2Factors holds the Tabelle 2 hourly factor and Lkw1/Lkw2 percentage
// shares for a given road class and time period. MFactor multiplies the DTV
// to yield the hourly traffic volume.
type table2Factors struct {
	MFactor   float64
	P1Percent float64
	P2Percent float64
}

// table2Coefficients is the Tabelle 2 lookup table, keyed by road class and
// then by time period. Values must match RLS-19 Tabelle 2 exactly.
var table2Coefficients = map[Table2RoadClass]map[Table2Period]table2Factors{
	Table2RoadClassMotorwayExpressway: {
		Table2PeriodDay:   {MFactor: 0.0555, P1Percent: 3, P2Percent: 11},
		Table2PeriodNight: {MFactor: 0.0140, P1Percent: 10, P2Percent: 25},
	},
	Table2RoadClassFederalRoad: {
		Table2PeriodDay:   {MFactor: 0.0575, P1Percent: 3, P2Percent: 7},
		Table2PeriodNight: {MFactor: 0.0100, P1Percent: 7, P2Percent: 13},
	},
	Table2RoadClassStateCountyMunicipalLinkRoad: {
		Table2PeriodDay:   {MFactor: 0.0575, P1Percent: 3, P2Percent: 5},
		Table2PeriodNight: {MFactor: 0.0100, P1Percent: 5, P2Percent: 6},
	},
	Table2RoadClassMunicipalRoad: {
		Table2PeriodDay:   {MFactor: 0.0575, P1Percent: 3, P2Percent: 4},
		Table2PeriodNight: {MFactor: 0.0100, P1Percent: 3, P2Percent: 4},
	},
}

// DTVToHourly converts a DTV value to the Tabelle 2 hourly representation
// for the selected road class and time period.
func DTVToHourly(dtv float64, roadClass Table2RoadClass, period Table2Period) (Table2HourlyTraffic, error) {
	if math.IsNaN(dtv) || math.IsInf(dtv, 0) || dtv < 0 {
		return Table2HourlyTraffic{}, fmt.Errorf("dtv must be finite and >= 0, got %g", dtv)
	}

	byPeriod, ok := table2Coefficients[roadClass]
	if !ok {
		return Table2HourlyTraffic{}, fmt.Errorf("unknown Tabelle 2 road class %d", roadClass)
	}

	factors, ok := byPeriod[period]
	if !ok {
		return Table2HourlyTraffic{}, fmt.Errorf("unknown Tabelle 2 period %d", period)
	}

	return Table2HourlyTraffic{
		MPerHour:  factors.MFactor * dtv,
		P1Percent: factors.P1Percent,
		P2Percent: factors.P2Percent,
	}, nil
}
