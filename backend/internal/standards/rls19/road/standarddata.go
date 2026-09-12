package road

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the RLS-19 coefficient tables this module carries.
//
// The Tabelle 8 reflection losses are held as a switch statement rather than as
// data, so they are evaluated over their enumerated domain here. A digest that
// covered only the declared tables would sit still while a hand-edited switch
// changed the numbers, which is exactly the failure this field exists to catch.
//
// The two parking tables were written that way too until the Parkplatztypen
// became named rows; they are declared data now, and the helpers below project
// them to the flat []float64 the digest has always hashed. That projection must
// keep its element type and its order: the digest is a function of the table
// name and the encoded value, so reshaping it would move the digest for a
// change that touched no coefficient.
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

// parkingMovementTable flattens the Tabelle 7 default movement rates, facility
// by facility and day before night.
func parkingMovementTable() []float64 {
	values := make([]float64, 0, len(parkingMovementRates)*2)
	for _, row := range parkingMovementRates {
		values = append(values, row.DayPerHour, row.NightPerHour)
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

// parkingVehicleSurchargeTable flattens the Tabelle 6 surcharges D_P,PT in
// table order. The name is the one the digest has carried since the table was
// first pinned and must not change with the Go identifiers.
func parkingVehicleSurchargeTable() []float64 {
	values := make([]float64, 0, len(parkingLotSurcharges))
	for _, row := range parkingLotSurcharges {
		values = append(values, row.SurchargeDB)
	}

	return values
}
