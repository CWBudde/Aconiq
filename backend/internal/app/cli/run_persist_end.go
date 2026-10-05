package cli

import (
	"time"

	"github.com/aconiq/backend/internal/acoustics"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/results"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/framework"
)

// endPersistSpec is what one END module contributes to the shared persist
// path. The receiver outputs are acoustics.ReceiverOutput for
// every module in the family, so everything else — the hash payload, the
// receiver table, the run summary — is identical by construction, and only
// these two values differ.
type endPersistSpec struct {
	// modelVersion is the version the run summary names. An alias module names
	// the module whose numbers it publishes, not itself.
	modelVersion string

	// reportingPrecisionDB is stamped into the run summary when non-zero. All
	// eight END modules set it; three of them — cnossos-industry, bub-industry
	// and buf-aircraft — did not until the value was written everywhere, and
	// their digest goldens moved by that one key when it was. The zero check
	// stays for a future module that reports no precision of its own: such a
	// module must leave the key out rather than publish a 0 dB one.
	reportingPrecisionDB float64

	// export is the module's own bundle writer. The layout is shared, but the
	// raster files are named after the standard that produced them, so an alias
	// module still writes its own.
	export func(baseDir string, outputs []acoustics.ReceiverOutput, layout results.GridLayout) (acoustics.ExportOutputs, error)
}

// endPersistSpecs holds one entry per module reporting the END day/evening/night
// indicator set. A standard missing from this map is a standard the END persist
// path cannot write, and says so rather than writing something plausible.
var endPersistSpecs = map[string]endPersistSpec{
	cnossosroad.StandardID: {
		modelVersion:         cnossosroad.BuiltinModelVersion,
		reportingPrecisionDB: cnossosroad.ReportingPrecisionDB,
		export:               cnossosroad.ExportResultBundle,
	},
	bubroad.StandardID: {
		modelVersion:         bubroad.BuiltinModelVersion,
		reportingPrecisionDB: bubroad.ReportingPrecisionDB,
		export:               bubroad.ExportResultBundle,
	},
	cnossosrail.StandardID: {
		modelVersion:         cnossosrail.BuiltinModelVersion,
		reportingPrecisionDB: cnossosrail.ReportingPrecisionDB,
		export:               cnossosrail.ExportResultBundle,
	},
	bubrail.StandardID: {
		modelVersion:         cnossosrail.BuiltinModelVersion,
		reportingPrecisionDB: cnossosrail.ReportingPrecisionDB,
		export:               bubrail.ExportResultBundle,
	},
	cnossosindustry.StandardID: {
		modelVersion:         cnossosindustry.BuiltinModelVersion,
		reportingPrecisionDB: cnossosindustry.ReportingPrecisionDB,
		export:               cnossosindustry.ExportResultBundle,
	},
	bubindustry.StandardID: {
		modelVersion:         cnossosindustry.BuiltinModelVersion,
		reportingPrecisionDB: cnossosindustry.ReportingPrecisionDB,
		export:               bubindustry.ExportResultBundle,
	},
	cnossosaircraft.StandardID: {
		modelVersion:         cnossosaircraft.BuiltinModelVersion,
		reportingPrecisionDB: cnossosaircraft.ReportingPrecisionDB,
		export:               cnossosaircraft.ExportResultBundle,
	},
	bufaircraft.StandardID: {
		modelVersion:         bufaircraft.BuiltinModelVersion,
		reportingPrecisionDB: bufaircraft.ReportingPrecisionDB,
		export:               bufaircraft.ExportResultBundle,
	},
}

// persistENDRunOutputs writes one run reporting the END indicator set: cnossos-road,
// -rail, -industry and -aircraft, their bub/buf counterparts, and anything else
// that reports Lden/Lnight/Lday/Levening.
//
// This replaces eight functions that were clones of one another — three of them
// close enough that dupl fired on them and carried a suppression. What made
// them clones was not the persist path but the indicator model: each module
// declared its own copy of the END types, so the same code had to be written
// once per type. They share acoustics.ReceiverOutput now, and one function
// serves them all.
func persistENDRunOutputs(
	standardID string,
	runDir string,
	outputs []acoustics.ReceiverOutput,
	layout results.GridLayout,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistENDRunOutputs"

	// Before the plan literal: `export` closes over layout, so a stamp
	// applied later would reach the run summary and miss the raster sidecar.
	layout = withComputeCRS(layout, projection)

	spec, ok := endPersistSpecs[standardID]
	if !ok {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, scope, "no END persist spec for standard "+standardID, nil)
	}

	plan := receiverPersistPlan[acoustics.ReceiverOutput]{
		scope:          scope,
		hashLabel:      standardID + " receiver outputs",
		hashErrMessage: "hash " + standardID + " outputs",
		modelVersion:   spec.modelVersion,
		indicatorOrder: acoustics.IndicatorOrder(),
		receiver:       func(output acoustics.ReceiverOutput) geo.PointReceiver { return output.Receiver },
		indicators:     func(output acoustics.ReceiverOutput) any { return output.Indicators },
		values:         func(output acoustics.ReceiverOutput) map[string]float64 { return output.Indicators.Values() },
		export:         exportBundle(scope, "export "+standardID+" results", spec.export, outputs, layout),
	}

	if spec.reportingPrecisionDB != 0 {
		plan.decorateSummary = func(summary map[string]any) {
			summary["reporting_precision_db"] = spec.reportingPrecisionDB
		}
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, layout, sourceCount, receiverMode, tier, projection)
}
