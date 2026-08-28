package aircraft

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the preview coefficient sets this scaffold module
// computes with.
//
// This module is a near-copy of cnossos/aircraft: it shares that module's
// emission code and differs only in its default lateral directivity, its model
// version and its descriptor. The tables are therefore mostly the same numbers
// under a different name, and the digest is what makes that visible rather than
// something a reader has to diff two packages to discover.
//
// The values are this project's own invented preview constants, kept stable so
// results stay reproducible. They are not NPD data, not ECAC Doc 29 profiles
// and not coefficients of any German mapping directive; see
// docs/conformance/cnossos-umfangserklaerung.md. A digest of invented constants
// is still worth carrying: it answers "could these two runs have produced
// different numbers?", which is a question about the data a module holds and
// not about the authority behind it.
//
// This module declares no coefficient table at all — every constant sits in a
// switch statement or inside a capped expression — so the tables below are
// produced by evaluating those functions over their enumerated domain. A digest
// that covered only declared package-level variables would be empty here, and
// would stay empty while a hand-edited switch changed every number.
//
// The movement-count and engine-state terms are not represented: both are
// 10 lg(x) over an unbounded domain with no constant of their own. Neither is
// the 20 lg(d) + 11 divergence, which is closed-form code rather than data; a
// change to it is a change to the code path, which the tool version already
// identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-aircraft-mapping/model-version", Value: BuiltinModelVersion},
		{Name: "preview-aircraft-mapping/class-corrections", Value: aircraftClassCorrectionTable()},
		{Name: "preview-aircraft-mapping/operation-corrections", Value: operationCorrectionTable()},
		{Name: "preview-aircraft-mapping/procedure-corrections", Value: procedureCorrectionTable()},
		{Name: "preview-aircraft-mapping/thrust-mode-corrections", Value: thrustModeCorrectionTable()},
		{Name: "preview-aircraft-mapping/operation-mode-adjustments", Value: operationModeAdjustmentTable()},
		{Name: "preview-aircraft-mapping/bank-angle-samples", Value: bankAngleSampleTable()},
		{Name: "preview-aircraft-mapping/lateral-offset-samples", Value: lateralOffsetSampleTable()},
		{Name: "preview-aircraft-mapping/propagation-defaults", Value: DefaultPropagationConfig()},
	}}
}

// standardDataOperations lists the operation types in the fixed order every
// per-operation table below is built in.
func standardDataOperations() []string {
	return []string{OperationDeparture, OperationArrival}
}

// aircraftClassCorrectionTable evaluates the class correction over every
// allowed aircraft class.
func aircraftClassCorrectionTable() []float64 {
	classes := []string{AircraftClassRegional, AircraftClassNarrow, AircraftClassWide, AircraftClassCargo}

	values := make([]float64, 0, len(classes))
	for _, class := range classes {
		values = append(values, aircraftClassCorrection(class))
	}

	return values
}

// operationCorrectionTable evaluates the emission-side operation correction
// over every allowed operation type.
func operationCorrectionTable() []float64 {
	operations := standardDataOperations()

	values := make([]float64, 0, len(operations))
	for _, operation := range operations {
		values = append(values, operationCorrection(operation))
	}

	return values
}

// procedureCorrectionTable evaluates the procedure correction over every
// allowed procedure type.
func procedureCorrectionTable() []float64 {
	procedures := []string{ProcedureStandardSID, ProcedureStandardSTAR, ProcedureContinuousDescent}

	values := make([]float64, 0, len(procedures))
	for _, procedure := range procedures {
		values = append(values, procedureCorrection(procedure))
	}

	return values
}

// thrustModeCorrectionTable evaluates the thrust-mode correction over every
// allowed thrust mode.
func thrustModeCorrectionTable() []float64 {
	modes := []string{ThrustTakeoff, ThrustReduced, ThrustIdle}

	values := make([]float64, 0, len(modes))
	for _, mode := range modes {
		values = append(values, thrustModeCorrection(mode))
	}

	return values
}

// operationModeAdjustmentTable evaluates the propagation-side operation
// adjustment at the default configuration, which pins how each operation type
// is mapped onto the climb and approach terms.
func operationModeAdjustmentTable() []float64 {
	operations := standardDataOperations()
	cfg := DefaultPropagationConfig()

	values := make([]float64, 0, len(operations))
	for _, operation := range operations {
		values = append(values, operationModeAdjustment(AircraftSource{OperationType: operation}, cfg))
	}

	return values
}

// bankAngleSampleTable samples the bank-angle correction. The sample angles sit
// below, at and above the 37.5 degrees where the term reaches its 2.5 dB cap,
// so both the divisor and the cap are pinned.
func bankAngleSampleTable() []float64 {
	angles := []float64{0, 7.5, 15, 30, 37.5, 45}

	values := make([]float64, 0, len(angles))
	for _, angle := range angles {
		values = append(values, bankAngleCorrection(angle))
	}

	return values
}

// lateralOffsetSampleTable samples the lateral-offset part of the directivity
// term against a zero configuration, which isolates it from the configured
// lateral directivity carried in the propagation defaults — the one number this
// module changes relative to cnossos/aircraft. The sample offsets sit below, at
// and above the 225 m where the term reaches its 1.5 dB cap, so both the
// divisor and the cap are pinned.
func lateralOffsetSampleTable() []float64 {
	offsets := []float64{0, 75, 150, 225, 300}

	values := make([]float64, 0, len(offsets))
	for _, offset := range offsets {
		values = append(values, lateralDirectivity(AircraftSource{LateralOffsetM: offset}, PropagationConfig{}))
	}

	return values
}
