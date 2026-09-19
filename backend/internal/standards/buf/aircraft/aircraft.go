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
func ComputeEmission(source AircraftSource) (PeriodLevels, error) {
	emission, err := cnossosaircraft.ComputeEmission(source)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute emission: %w", err)
	}

	return PeriodLevels(emission), nil
}

// ComputeLden computes the day-evening-night indicator from period levels.
func ComputeLden(levels PeriodLevels) float64 {
	return cnossosaircraft.ComputeLden(levels)
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []AircraftSource, cfg PropagationConfig) (PeriodLevels, error) {
	levels, err := cnossosaircraft.ComputeReceiverPeriodLevels(receiver, sources, cfg)
	if err != nil {
		return PeriodLevels{}, fmt.Errorf("compute receiver period levels: %w", err)
	}

	return levels, nil
}

// ComputeReceiverOutputs computes indicators for all receivers in order.
func ComputeReceiverOutputs(receivers []geo.PointReceiver, sources []AircraftSource, cfg PropagationConfig) ([]ReceiverOutput, error) {
	outputs, err := cnossosaircraft.ComputeReceiverOutputs(receivers, sources, cfg)
	if err != nil {
		return nil, fmt.Errorf("compute receiver outputs: %w", err)
	}

	return outputs, nil
}
