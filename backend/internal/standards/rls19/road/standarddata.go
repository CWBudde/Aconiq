package road

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the RLS-19 coefficient tables this module carries.
//
// Some RLS-19 tables are held as switch statements rather than as data — the
// Tabelle 8 reflection losses and the Tabelle 7 parking movement rates are
// written that way — so those are evaluated over their enumerated domain here.
// A digest that covered only the declared tables would sit still while a
// hand-edited switch changed the numbers, which is exactly the failure this
// field exists to catch.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "rls19/data-pack-version", Value: BuiltinDataPackVersion},
		{Name: "rls19/tabelle-02-verkehrsmengen", Value: table2Coefficients},
		{Name: "rls19/tabelle-03-grundemission", Value: baseEmissionTable},
		{Name: "rls19/tabelle-04-fahrbahnkorrektur", Value: surfaceCorrectionTable},
		{Name: "rls19/tabelle-05-knotenpunktkorrektur", Value: kktTable},
		{Name: "rls19/tabelle-07-parkbewegungen", Value: parkingMovementTable()},
		{Name: "rls19/tabelle-08-reflexionsverluste", Value: reflectionLossTable()},
		{Name: "rls19/parkplatz-fahrzeugzuschlaege", Value: parkingVehicleSurchargeTable()},
		{Name: "rls19/ausbreitungskonstanten", Value: PropagationConstants},
	}}
}

// parkingMovementTable evaluates the Tabelle 7 default movement rates over
// every facility type and period.
func parkingMovementTable() []float64 {
	facilities := []ParkingFacilityType{ParkingFacilityPR, ParkingFacilityTankRast}
	periods := []TimePeriod{TimePeriodDay, TimePeriodNight}

	values := make([]float64, 0, len(facilities)*len(periods))
	for _, facility := range facilities {
		for _, period := range periods {
			values = append(values, DefaultMovementsPerHour(facility, period))
		}
	}

	return values
}

// reflectionLossTable evaluates the Tabelle 8 reflection losses D_RV over every
// typed reflector surface class.
func reflectionLossTable() []float64 {
	types := []ReflectorType{
		ReflectorTypeUnspecified,
		ReflectorTypeFacadeOrReflecting,
		ReflectorTypeReflectionReducing,
		ReflectorTypeStronglyReflectionReducing,
	}

	values := make([]float64, 0, len(types))
	for _, reflectorType := range types {
		values = append(values, Reflector{Type: reflectorType}.effectiveLoss())
	}

	return values
}

// parkingVehicleSurchargeTable evaluates the per-vehicle-type parking surcharge
// D_P,PT over every vehicle type.
func parkingVehicleSurchargeTable() []float64 {
	types := []ParkingVehicleType{ParkingPkw, ParkingMotorrad, ParkingLkwOmnibus}

	values := make([]float64, 0, len(types))
	for _, vehicleType := range types {
		values = append(values, parkingVehicleTypeSurcharge(vehicleType))
	}

	return values
}
