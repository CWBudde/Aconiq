// Package aircraft is the BUF aircraft mapping module. It is an alias package
// over cnossos/aircraft: the acoustics are identical, so the types, the
// emission model and the propagation kernel are that package's, named here.
//
// What this module keeps of its own is its identity, and only that: the
// standard id, the descriptor with its seven divergent parameter defaults, the
// builtin model version, the compliance boundary, the coefficient tables under
// their own preview-aircraft-mapping/ names, and the one propagation default
// that differs — LateralDirectivityDB, 1.0 here against 0 there. Converging any
// of those would move this module's numbers, which is a release decision rather
// than a deduplication.
package aircraft

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
)

const (
	// StandardID identifies the BUF aircraft module entry in the standards registry.
	StandardID = "buf-aircraft"

	IndicatorLday     = cnossosaircraft.IndicatorLday
	IndicatorLevening = cnossosaircraft.IndicatorLevening
	IndicatorLnight   = cnossosaircraft.IndicatorLnight
	IndicatorLden     = cnossosaircraft.IndicatorLden

	SourceTypeLine = cnossosaircraft.SourceTypeLine

	OperationDeparture = cnossosaircraft.OperationDeparture
	OperationArrival   = cnossosaircraft.OperationArrival

	AircraftClassRegional = cnossosaircraft.AircraftClassRegional
	AircraftClassNarrow   = cnossosaircraft.AircraftClassNarrow
	AircraftClassWide     = cnossosaircraft.AircraftClassWide
	AircraftClassCargo    = cnossosaircraft.AircraftClassCargo

	ProcedureStandardSID       = cnossosaircraft.ProcedureStandardSID
	ProcedureStandardSTAR      = cnossosaircraft.ProcedureStandardSTAR
	ProcedureContinuousDescent = cnossosaircraft.ProcedureContinuousDescent

	ThrustTakeoff = cnossosaircraft.ThrustTakeoff
	ThrustReduced = cnossosaircraft.ThrustReduced
	ThrustIdle    = cnossosaircraft.ThrustIdle
)

type (
	MovementPeriod     = cnossosaircraft.MovementPeriod
	AirportRef         = cnossosaircraft.AirportRef
	AircraftSource     = cnossosaircraft.AircraftSource
	PropagationConfig  = cnossosaircraft.PropagationConfig
	PeriodLevels       = cnossosaircraft.PeriodLevels
	ReceiverIndicators = cnossosaircraft.ReceiverIndicators
	ReceiverOutput     = cnossosaircraft.ReceiverOutput
)

// ExportOutputs describes written files for receiver table and raster output.
type ExportOutputs = acoustics.ExportOutputs

// ExportResultBundle exports Lden/Lnight receiver table and raster outputs.
// The layout is the shared END one; only the raster is named after this
// standard, so two bundles from different modules stay comparable.
func ExportResultBundle(baseDir string, outputs []ReceiverOutput, grid results.GridLayout) (ExportOutputs, error) {
	bundle, err := acoustics.ExportENDBundle(baseDir, StandardID, outputs, grid)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("export %s result bundle: %w", StandardID, err)
	}

	return bundle, nil
}

// DefaultPropagationConfig returns baseline aircraft propagation terms. It is
// the cnossos/aircraft baseline with LateralDirectivityDB raised to 1.0, which
// is the one number this module changes and the reason it is not a plain
// delegation.
func DefaultPropagationConfig() PropagationConfig {
	cfg := cnossosaircraft.DefaultPropagationConfig()
	cfg.LateralDirectivityDB = 1.0

	return cfg
}

// ComputeEmission computes day/evening/night emission levels for one source.
//
// The conversion is what stops this being a plain delegation: cnossos/aircraft
// returns an unexported periodEmission, which an alias cannot name.
//
// The error travels unchanged, here and in the two functions below. Each
// callee already carries whatever context it means to — ComputeReceiverOutputs
// says "compute receiver outputs", the other two return their validation
// errors bare — and the implementation these wrappers replaced was that same
// code, so adding a prefix here would put a second one on a message that had
// one, or a first on a message that had none.
func ComputeEmission(source AircraftSource) (PeriodLevels, error) {
	emission, err := cnossosaircraft.ComputeEmission(source)
	if err != nil {
		//nolint:wrapcheck // preserves the error text of the implementation this alias replaced
		return PeriodLevels{}, err
	}

	return PeriodLevels(emission), nil
}

// ComputeLden computes the day-evening-night indicator from period levels.
func ComputeLden(levels PeriodLevels) float64 {
	return cnossosaircraft.ComputeLden(levels)
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
//
//nolint:wrapcheck // preserves the error text of the implementation this alias replaced
func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []AircraftSource, cfg PropagationConfig) (PeriodLevels, error) {
	return cnossosaircraft.ComputeReceiverPeriodLevels(receiver, sources, cfg)
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
//
//nolint:wrapcheck // preserves the error text of the implementation this alias replaced
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []AircraftSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	return cnossosaircraft.ComputeReceiverOutputs(receivers, sources, cfg)
}
