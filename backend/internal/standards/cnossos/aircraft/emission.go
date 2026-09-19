package aircraft

import "math"

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one aircraft source.
func ComputeEmission(source AircraftSource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		Lday:     emissionForPeriod(source, source.MovementDay),
		Levening: emissionForPeriod(source, source.MovementEvening),
		Lnight:   emissionForPeriod(source, source.MovementNight),
	}, nil
}

func emissionForPeriod(source AircraftSource, movement MovementPeriod) float64 {
	if movement.MovementsPerHour <= 0 {
		return -999.0
	}

	base := source.ReferencePowerLevelDB
	base += AircraftClassCorrection(source.AircraftClass)
	base += OperationCorrection(source.OperationType)
	base += engineStateCorrection(source.EngineStateFactor)
	base += ProcedureCorrection(source.ProcedureType)
	base += ThrustModeCorrection(source.ThrustMode)

	return base + 10*math.Log10(movement.MovementsPerHour)
}

// AircraftClassCorrection returns the emission correction for one aircraft
// class. Exported for the same reason as LateralDirectivity in propagation.go:
// buf/aircraft aliases this package and samples the term for its own table.
func AircraftClassCorrection(class string) float64 {
	switch class {
	case AircraftClassRegional:
		return -4.0
	case AircraftClassNarrow:
		return 0.0
	case AircraftClassWide:
		return 3.5
	case AircraftClassCargo:
		return 5.0
	default:
		return 0
	}
}

// OperationCorrection returns the emission-side correction for one operation
// type. Exported for the same reason as AircraftClassCorrection.
func OperationCorrection(operation string) float64 {
	switch operation {
	case OperationDeparture:
		return 2.0
	case OperationArrival:
		return -1.0
	default:
		return 0
	}
}

func engineStateCorrection(factor float64) float64 {
	return 10 * math.Log10(factor)
}

// ProcedureCorrection returns the emission correction for one procedure type.
// Exported for the same reason as AircraftClassCorrection.
func ProcedureCorrection(procedure string) float64 {
	switch procedure {
	case ProcedureStandardSID:
		return 1.5
	case ProcedureStandardSTAR:
		return -0.5
	case ProcedureContinuousDescent:
		return -2.0
	default:
		return 0
	}
}

// ThrustModeCorrection returns the emission correction for one thrust mode.
// Exported for the same reason as AircraftClassCorrection.
func ThrustModeCorrection(mode string) float64 {
	switch mode {
	case ThrustTakeoff:
		return 2.5
	case ThrustReduced:
		return 0.5
	case ThrustIdle:
		return -3.0
	default:
		return 0
	}
}
